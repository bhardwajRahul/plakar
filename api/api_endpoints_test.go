package api

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/PlakarKorp/integrations/fs/exporter"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/kloset/snapshot"
	"github.com/PlakarKorp/plakar/appcontext"
	ptesting "github.com/PlakarKorp/plakar/testing"
	"github.com/stretchr/testify/require"
)

// newAPIServer builds a real repository with one snapshot and returns an http
// handler wired through SetupRoutes with no auth token and norefresh=true (so
// endpoints don't try to reach the `cached` daemon to rebuild state).
func newAPIServer(t *testing.T) (*http.ServeMux, *repository.Repository, *snapshot.Snapshot, *appcontext.AppContext) {
	t.Helper()
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockFile("subdir/dummy.txt", 0644, "hello dummy"),
		ptesting.NewMockFile("subdir/foo.txt", 0644, "hello foo"),
	})

	mux := http.NewServeMux()
	SetupRoutes(mux, repo, ctx, "", true /* norefresh */)
	return mux, repo, snap, ctx
}

func doGET(t *testing.T, mux *http.ServeMux, url string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func TestAPIRepositoryInfo(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	w := doGET(t, mux, "/api/repository/info")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Contains(t, resp, "item")
}

func TestAPIRepositorySnapshots(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	w := doGET(t, mux, "/api/repository/snapshots")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "total")
}

func TestAPIRepositorySnapshotsBadSince(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	w := doGET(t, mux, "/api/repository/snapshots?since=not-a-date")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAPIRepositoryImporterTypes(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	w := doGET(t, mux, "/api/repository/importer-types")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestAPISnapshotHeader(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	id := snap.Header.GetIndexID()
	w := doGET(t, mux, "/api/snapshot/"+hex.EncodeToString(id[:]))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestAPISnapshotHeaderBadID(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	w := doGET(t, mux, "/api/snapshot/nothexnothex")
	require.NotEqual(t, http.StatusOK, w.Code)
}

func TestAPISnapshotVFSBrowse(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	indexID := snap.Header.GetIndexID()
	id := hex.EncodeToString(indexID[:])
	w := doGET(t, mux, "/api/snapshot/vfs/"+id+":/subdir")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestAPISnapshotVFSChildren(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	indexID := snap.Header.GetIndexID()
	id := hex.EncodeToString(indexID[:])
	w := doGET(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestAPISnapshotVFSErrors(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	indexID := snap.Header.GetIndexID()
	id := hex.EncodeToString(indexID[:])
	w := doGET(t, mux, "/api/snapshot/vfs/errors/"+id+":/")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestAPISnapshotVFSChunks(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	indexID := snap.Header.GetIndexID()
	id := hex.EncodeToString(indexID[:])
	w := doGET(t, mux, "/api/snapshot/vfs/chunks/"+id+":/subdir/dummy.txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestAPISnapshotVFSSearch(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	indexID := snap.Header.GetIndexID()
	id := hex.EncodeToString(indexID[:])
	w := doGET(t, mux, "/api/snapshot/vfs/search/"+id+":/subdir?recursive=true")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestAPIRepositoryLocatePathname(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	w := doGET(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestAPISnapshotReader(t *testing.T) {
	// The reader endpoint serves file content. With an empty token, the URL
	// signer's VerifyMiddleware falls through to the (no-op) token auth.
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	indexID := snap.Header.GetIndexID()
	id := hex.EncodeToString(indexID[:])
	w := doGET(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "hello dummy")
}

func TestAPISnapshotSignURL(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	indexID := snap.Header.GetIndexID()
	id := hex.EncodeToString(indexID[:])
	req, err := http.NewRequest("POST", "/api/snapshot/reader-sign-url/"+id+":/subdir/dummy.txt", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp map[string]map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp["item"]["signature"])
}

func TestAPISnapshotReaderRenderVariants(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	indexID := snap.Header.GetIndexID()
	id := hex.EncodeToString(indexID[:])
	base := "/api/snapshot/reader/" + id + ":/subdir/dummy.txt"

	for _, render := range []string{"text", "text_styled", "code"} {
		w := doGET(t, mux, base+"?render="+render)
		require.Equal(t, http.StatusOK, w.Code, "render=%s body=%s", render, w.Body.String())
	}

	// download=true sets a Content-Disposition attachment header.
	w := doGET(t, mux, base+"?download=true")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Disposition"), "attachment")

	// an invalid render value is rejected.
	w = doGET(t, mux, base+"?render=bogus")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAPISnapshotDownloaderFlow(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	indexID := snap.Header.GetIndexID()
	id := hex.EncodeToString(indexID[:])

	// POST a download request describing which files to bundle.
	body := `{"name":"dl","items":[{"pathname":"/subdir/dummy.txt"}]}`
	req, err := http.NewRequest("POST", "/api/snapshot/vfs/downloader/"+id+":/subdir", bytes.NewBufferString(body))
	require.NoError(t, err)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp struct {
		Id string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Id)

	// Without a format the signed endpoint rejects the request.
	w = doGET(t, mux, "/api/snapshot/vfs/downloader-sign-url/"+resp.Id)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// With a valid archive format it serves the bundle. (Re-POST first, as the
	// id is single-use once consumed.)
	req, err = http.NewRequest("POST", "/api/snapshot/vfs/downloader/"+id+":/subdir", bytes.NewBufferString(body))
	require.NoError(t, err)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	w = doGET(t, mux, "/api/snapshot/vfs/downloader-sign-url/"+resp.Id+"?format=zip")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestAPIUnknownEndpoint(t *testing.T) {
	mux, _, snap, _ := newAPIServer(t)
	defer snap.Close()

	w := doGET(t, mux, "/api/does-not-exist")
	require.Equal(t, http.StatusNotFound, w.Code)
}
