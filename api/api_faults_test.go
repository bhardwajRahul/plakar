package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/PlakarKorp/kloset/caching"
	"github.com/PlakarKorp/kloset/caching/pebble"
	"github.com/PlakarKorp/kloset/connectors/storage"
	"github.com/PlakarKorp/kloset/hashing"
	"github.com/PlakarKorp/kloset/logging"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/kloset/resources"
	"github.com/PlakarKorp/kloset/versioning"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/cookies"
	ptesting "github.com/PlakarKorp/plakar/testing"
	"github.com/stretchr/testify/require"
)

// faultServer builds an API mux over the mock storage backend with the given
// fault-injection behavior and an empty (but valid) repository state.
// repository.New only fails for behaviors whose List(State) errors
// (brokenState), so the empty-state behavior used here always succeeds.
func faultServer(t *testing.T, behavior string, norefresh bool) (*http.ServeMux, *appcontext.AppContext) {
	t.Helper()

	loc := "mock:///test/location"
	if behavior != "" {
		loc += "?behavior=" + behavior
	}

	tmpCacheDir, err := os.MkdirTemp("", "tmp_cache_apifault")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpCacheDir) })

	config := ptesting.NewConfiguration()
	serializedConfig, err := config.ToBytes()
	require.NoError(t, err)

	hasher := hashing.GetHasher(hashing.DEFAULT_HASHING_ALGORITHM)
	wrappedConfigRd, err := storage.Serialize(hasher, resources.RT_CONFIG, versioning.GetCurrentVersion(resources.RT_CONFIG), bytes.NewReader(serializedConfig))
	require.NoError(t, err)
	wrappedConfig, err := io.ReadAll(wrappedConfigRd)
	require.NoError(t, err)

	ctx := appcontext.NewAppContext()
	t.Cleanup(ctx.Close)
	cache := caching.NewManager(pebble.Constructor(tmpCacheDir))
	t.Cleanup(func() { cache.Close() })
	ctx.SetCache(cache)
	ctx.CacheDir = tmpCacheDir
	ctx.SetLogger(logging.NewLogger(io.Discard, io.Discard))
	ctx.SetCookies(cookies.NewManager(tmpCacheDir))
	ctx.Client = "plakar-test/1.0.0"

	lstore, err := storage.Create(ctx.GetInner(), map[string]string{"location": loc}, wrappedConfig)
	require.NoError(t, err, "creating storage")
	repo, err := repository.New(ctx.GetInner(), nil, lstore, wrappedConfig)
	require.NoError(t, err, "creating repository")

	mux := http.NewServeMux()
	SetupRoutes(mux, repo, ctx, "", norefresh)
	return mux, ctx
}

func faultGET(t *testing.T, mux *http.ServeMux, url string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

// --- repository snapshot-listing parameter validation -----------------------

// TestFaultRepositorySnapshotsParamErrors covers the parameter-validation error
// returns in repositorySnapshots (offset/limit/since/sort) which short-circuit
// before any storage access.
func TestFaultRepositorySnapshotsParamErrors(t *testing.T) {
	mux, _ := faultServer(t, "", true)

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
			w := faultGET(t, mux, fmt.Sprintf("/api/repository/snapshots?%s", c.query))
			require.Equal(t, c.status, w.Code, "body=%s", w.Body.String())
		})
	}
}

// TestFaultRepositoryLocatePathnameParamErrors covers the parameter-validation
// error returns in repositoryLocatePathname.
func TestFaultRepositoryLocatePathnameParamErrors(t *testing.T) {
	mux, _ := faultServer(t, "", true)

	for _, q := range []string{"offset=abc", "limit=abc", "sort=NoSuchKey"} {
		t.Run(q, func(t *testing.T) {
			w := faultGET(t, mux, "/api/repository/locate-pathname?"+q)
			require.GreaterOrEqual(t, w.Code, 400, "body=%s", w.Body.String())
		})
	}
}

// TestFaultRepositoryEmptyStateOK confirms the snapshot-listing handlers serve
// an empty result (200) over a valid but empty repository, exercising the
// no-snapshot loop tails.
func TestFaultRepositoryEmptyStateOK(t *testing.T) {
	mux, _ := faultServer(t, "", true)
	for _, path := range []string{
		"/api/repository/snapshots",
		"/api/repository/importer-types",
		"/api/repository/locate-pathname",
		"/api/repository/info",
	} {
		w := faultGET(t, mux, path)
		require.Equal(t, http.StatusOK, w.Code, "path=%s body=%s", path, w.Body.String())
	}
}

// --- services proxy error branches ------------------------------------------

// TestFaultServicesProxyBadEndpoint points the proxy at an unparseable endpoint
// URL, exercising the `targetBase, err := url.Parse(...); if err != nil` branch
// of servicesProxy.
func TestFaultServicesProxyBadEndpoint(t *testing.T) {
	t.Setenv("PLAKAR_SERVICE_ENDPOINT", "://this is not a url")
	mux, _ := faultServer(t, "", true)
	w := faultGET(t, mux, "/api/proxy/v1/account/me")
	require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
}

// TestFaultServicesProxyUnreachable points the proxy at a closed local port so
// the outbound http.DefaultClient.Do fails with connection-refused, exercising
// the `resp, err := http.DefaultClient.Do(req); if err != nil` branch. Hermetic:
// nothing listens on 127.0.0.1:0-equivalent unreachable port.
func TestFaultServicesProxyUnreachable(t *testing.T) {
	t.Setenv("PLAKAR_SERVICE_ENDPOINT", "http://127.0.0.1:1")
	mux, _ := faultServer(t, "", true)
	w := faultGET(t, mux, "/api/proxy/v1/account/me")
	require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
}

// --- alerting service configuration handlers --------------------------------

// TestFaultAlertingGetNoAuth covers the no-auth-token 401 branch of
// servicesGetAlertingServiceConfiguration (no network access required).
func TestFaultAlertingGetNoAuth(t *testing.T) {
	mux, _ := faultServer(t, "", true)
	w := faultGET(t, mux, "/api/proxy/v1/account/services/alerting")
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "authorization_error")
}

// TestFaultAlertingSetNoAuth covers the no-auth-token 401 branch of
// servicesSetAlertingServiceConfiguration.
func TestFaultAlertingSetNoAuth(t *testing.T) {
	mux, _ := faultServer(t, "", true)
	req, err := http.NewRequest("PUT", "/api/proxy/v1/account/services/alerting", bytes.NewBufferString(`{"enabled":true}`))
	require.NoError(t, err)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())
}
