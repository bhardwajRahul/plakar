package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/PlakarKorp/integrations/fs/exporter"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/kloset/snapshot"
	ptesting "github.com/PlakarKorp/plakar/testing"
	"github.com/stretchr/testify/require"
)

func TestHandleErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"not-readable -> 400", repository.ErrNotReadable, http.StatusBadRequest},
		{"blob-not-found -> 404", repository.ErrBlobNotFound, http.StatusNotFound},
		{"packfile-not-found -> 404", repository.ErrPackfileNotFound, http.StatusNotFound},
		{"fs-not-exist -> 404", fs.ErrNotExist, http.StatusNotFound},
		{"snapshot-not-found -> 404", snapshot.ErrNotFound, http.StatusNotFound},
		{"non-wrapped fs-not-exist -> 500", errors.New("boom: " + fs.ErrNotExist.Error()), http.StatusInternalServerError},
		{"unknown -> 500", errors.New("some random failure"), http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/whatever", nil)
			w := httptest.NewRecorder()
			handleError(w, req, c.err)
			require.Equal(t, c.want, w.Code)

			var body ApiErrorRes
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.NotNil(t, body.Error)
		})
	}
}

func TestUnknownEndpoint(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	w := get(t, mux, "/api/does-not-exist")
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestAuthTokenRequired(t *testing.T) {
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockFile("a.txt", 0644, "a"),
	})
	defer snap.Close()

	mux := http.NewServeMux()
	SetupRoutes(mux, repo, ctx, "secret-token", true)

	// Missing Authorization header -> 401.
	w := get(t, mux, "/api/info")
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// Wrong token -> 401.
	req, _ := http.NewRequest("GET", "/api/info", nil)
	req.Header.Set("Authorization", "Bearer nope")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// Correct token -> 200.
	req, _ = http.NewRequest("GET", "/api/info", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Contains(t, resp, "repository_id")
	require.Contains(t, resp, "version")
}

func TestInfoDemoMode(t *testing.T) {
	t.Setenv("PLAKAR_DEMO_MODE", "true")
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	w := get(t, mux, "/api/info")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var resp struct {
		DemoMode bool `json:"demo_mode"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.DemoMode)
}

func TestApiInfoAuthenticated(t *testing.T) {
	mux, _, snap, ctx := server(t, "")
	defer snap.Close()

	// Drop an auth token into the cookie jar so apiInfo reports authenticated.
	require.NoError(t, ctx.GetCookies().PutAuthToken("some-auth-token"))

	w := get(t, mux, "/api/info")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp struct {
		Authenticated bool   `json:"authenticated"`
		RepositoryId  string `json:"repository_id"`
		Version       string `json:"version"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Authenticated)
	require.NotEmpty(t, resp.RepositoryId)
}
