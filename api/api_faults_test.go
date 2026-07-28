package api

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// --- repository snapshot-listing parameter validation -----------------------

// TestFaultRepositorySnapshotsParamErrors covers the parameter-validation error
// returns in repositorySnapshots (offset/limit/since/sort) which short-circuit
// before any storage access.
func TestFaultRepositorySnapshotsParamErrors(t *testing.T) {
	mux, _, _, _ := server(t, "")

	cases := []struct {
		name   string
		query  string
		status int
	}{
		{"bad offset", "offset=abc", http.StatusBadRequest},
		{"bad limit", "limit=abc", http.StatusBadRequest},
		{"bad since", "since=not-a-date", http.StatusBadRequest},
		{"bad sort", "sort=NoSuchKey", http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := get(t, mux, fmt.Sprintf("/api/repository/snapshots?%s", c.query))
			require.Equal(t, c.status, w.Code, "body=%s", w.Body.String())
		})
	}
}

// TestFaultRepositoryLocatePathnameParamErrors covers the parameter-validation
// error returns in repositoryLocatePathname.
func TestFaultRepositoryLocatePathnameParamErrors(t *testing.T) {
	mux, _, _, _ := server(t, "")

	for _, q := range []string{"offset=abc", "limit=abc", "sort=NoSuchKey"} {
		t.Run(q, func(t *testing.T) {
			w := get(t, mux, "/api/repository/locate-pathname?"+q)
			require.GreaterOrEqual(t, w.Code, 400, "body=%s", w.Body.String())
		})
	}
}

// TestFaultRepositoryEmptyStateOK confirms the snapshot-listing handlers serve
// an empty result (200) over a valid but empty repository, exercising the
// no-snapshot loop tails.
func TestFaultRepositoryEmptyStateOK(t *testing.T) {
	mux, _, _, _ := server(t, "")
	for _, path := range []string{
		"/api/repository/snapshots",
		"/api/repository/importer-types",
		"/api/repository/locate-pathname",
		"/api/repository/info",
	} {
		w := get(t, mux, path)
		require.Equal(t, http.StatusOK, w.Code, "path=%s body=%s", path, w.Body.String())
	}
}

// --- services proxy error branches ------------------------------------------

// TestFaultServicesProxyBadEndpoint points the proxy at an unparseable endpoint
// URL, exercising the `targetBase, err := url.Parse(...); if err != nil` branch
// of servicesProxy.
func TestFaultServicesProxyBadEndpoint(t *testing.T) {
	t.Setenv("PLAKAR_SERVICE_ENDPOINT", "://this is not a url")
	mux, _, _, _ := server(t, "")
	w := get(t, mux, "/api/proxy/v1/account/me")
	require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
}

// TestFaultServicesProxyUnreachable points the proxy at a closed local port so
// the outbound http.DefaultClient.Do fails with connection-refused, exercising
// the `resp, err := http.DefaultClient.Do(req); if err != nil` branch. Hermetic:
// nothing listens on 127.0.0.1:0-equivalent unreachable port.
func TestFaultServicesProxyUnreachable(t *testing.T) {
	t.Setenv("PLAKAR_SERVICE_ENDPOINT", "http://127.0.0.1:1")
	mux, _, _, _ := server(t, "")
	w := get(t, mux, "/api/proxy/v1/account/me")
	require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
}

// --- alerting service configuration handlers --------------------------------

// TestFaultAlertingGetNoAuth covers the no-auth-token 401 branch of
// servicesGetAlertingServiceConfiguration (no network access required).
func TestFaultAlertingGetNoAuth(t *testing.T) {
	mux, _, _, _ := server(t, "")
	w := get(t, mux, "/api/proxy/v1/account/services/alerting")
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "authorization_error")
}

// TestFaultAlertingSetNoAuth covers the no-auth-token 401 branch of
// servicesSetAlertingServiceConfiguration.
func TestFaultAlertingSetNoAuth(t *testing.T) {
	mux, _, _, _ := server(t, "")
	req, err := http.NewRequest("PUT", "/api/proxy/v1/account/services/alerting", bytes.NewBufferString(`{"enabled":true}`))
	require.NoError(t, err)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())
}
