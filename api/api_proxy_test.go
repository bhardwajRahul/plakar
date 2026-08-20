package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PlakarKorp/pkg"
	"github.com/stretchr/testify/require"
)

// TestProxy drives the services-proxy endpoints. servicesProxy reads
// PLAKAR_SERVICE_ENDPOINT per request, so the endpoint-error cases can use
// t.Setenv against the shared mux instead of rebuilding it.
func TestProxy(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	t.Run("alerting get unauthorized", func(t *testing.T) {
		// No auth token in the cookie jar -> handler returns a 401 JSON body.
		w := get(t, mux, "/api/proxy/v1/account/services/alerting")
		require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())

		var resp map[string]string
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, "authorization_error", resp["error"])
	})

	t.Run("alerting set unauthorized", func(t *testing.T) {
		body := `{"enabled":true,"email_report":true}`
		req, _ := http.NewRequest("PUT", "/api/proxy/v1/account/services/alerting", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())

		var resp map[string]string
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, "authorization_error", resp["error"])
	})

	t.Run("integration path not implemented", func(t *testing.T) {
		w := get(t, mux, "/api/proxy/v1/integration/some-id/some/path")
		// Returns a plain error -> mapped to 500 by handleError.
		require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
	})

	t.Run("bad endpoint", func(t *testing.T) {
		// An unparseable endpoint URL exercises the
		// `targetBase, err := url.Parse(...); if err != nil` branch.
		t.Setenv("PLAKAR_SERVICE_ENDPOINT", "://this is not a url")

		w := get(t, mux, "/api/proxy/v1/account/me")
		require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
	})

	t.Run("unreachable", func(t *testing.T) {
		// A closed local port makes the outbound http.DefaultClient.Do fail with
		// connection-refused, exercising the `if err != nil` branch after it.
		// Hermetic: nothing listens on port 1.
		t.Setenv("PLAKAR_SERVICE_ENDPOINT", "http://127.0.0.1:1")

		w := get(t, mux, "/api/proxy/v1/account/me")
		require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
	})
}

// TestIntegrationMatchesSearch pins the search filter used by
// servicesGetIntegration: case-insensitive substring over Name OR
// DisplayName; empty needle matches everything.
func TestIntegrationMatchesSearch(t *testing.T) {
	cases := []struct {
		name, needle string
		it           pkg.Integration
		want         bool
	}{
		{"empty needle matches", "", pkg.Integration{Name: "aws-s3"}, true},
		{"name substring, lowercase", "aws", pkg.Integration{Name: "aws-s3"}, true},
		{"name substring, mixed case in field", "s3", pkg.Integration{Name: "aws-S3", DisplayName: "Amazon S3"}, true},
		{"display name substring", "amazon", pkg.Integration{Name: "aws-s3", DisplayName: "Amazon S3"}, true},
		{"no match", "gcp", pkg.Integration{Name: "aws-s3", DisplayName: "Amazon S3"}, false},
		{"empty display name doesn't panic", "aws", pkg.Integration{Name: "aws-s3", DisplayName: ""}, true},
		{"empty name doesn't panic", "amazon", pkg.Integration{Name: "", DisplayName: "Amazon S3"}, true},
		{"unicode passthrough", "π", pkg.Integration{Name: "geo-π-service"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := integrationMatchesSearch(&tc.it, tc.needle)
			require.Equal(t, tc.want, got)
		})
	}
}
