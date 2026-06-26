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

// covServer builds a real repository with a richer snapshot than newAPIServer
// (nested dirs, multiple files) and wires SetupRoutes with norefresh=true.
// Distinct name to avoid clashing with the team's newAPIServer helper.
func covServer(t *testing.T) (*http.ServeMux, *repository.Repository, *snapshot.Snapshot, *appcontext.AppContext) {
	t.Helper()
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockFile("subdir/dummy.txt", 0644, "hello dummy"),
		ptesting.NewMockFile("subdir/foo.txt", 0644, "hello foo"),
		ptesting.NewMockFile("subdir/baz.txt", 0644, "hello baz"),
		ptesting.NewMockDir("subdir/nested"),
		ptesting.NewMockFile("subdir/nested/deep.txt", 0644, "deep content"),
		ptesting.NewMockFile("top.txt", 0644, "top level"),
	})
	mux := http.NewServeMux()
	SetupRoutes(mux, repo, ctx, "", true /* norefresh */)
	return mux, repo, snap, ctx
}

func covGET(t *testing.T, mux *http.ServeMux, url string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func covSnapID(snap *snapshot.Snapshot) string {
	id := snap.Header.GetIndexID()
	return hex.EncodeToString(id[:])
}

// --- Auth middleware with a non-empty token --------------------------------

func TestCovAuthTokenRequired(t *testing.T) {
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockFile("a.txt", 0644, "a"),
	})
	defer snap.Close()

	mux := http.NewServeMux()
	SetupRoutes(mux, repo, ctx, "secret-token", true)

	// Missing Authorization header -> 401.
	w := covGET(t, mux, "/api/info")
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

// --- Snapshot header / not-found branches ----------------------------------

func TestCovSnapshotHeaderNotFound(t *testing.T) {
	mux, _, snap, _ := covServer(t)
	defer snap.Close()

	// Valid hex, valid length, but no such snapshot -> 404.
	w := covGET(t, mux, "/api/snapshot/7e0e6e24a6e29faf11d022dca77826fe8b8a000aff5ea27e16650d03acefc93c")
	require.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

func TestCovSnapshotHeaderBadParam(t *testing.T) {
	mux, _, snap, _ := covServer(t)
	defer snap.Close()

	w := covGET(t, mux, "/api/snapshot/zz")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

// --- VFS browse / children paging edges ------------------------------------

func TestCovVFSBrowseRootAndFile(t *testing.T) {
	mux, _, snap, _ := covServer(t)
	defer snap.Close()
	id := covSnapID(snap)

	// Root directory (path empty -> "/").
	w := covGET(t, mux, "/api/snapshot/vfs/"+id+":/")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// A regular file entry.
	w = covGET(t, mux, "/api/snapshot/vfs/"+id+":/top.txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// Non-existent path -> error (not 200).
	w = covGET(t, mux, "/api/snapshot/vfs/"+id+":/does/not/exist")
	require.NotEqual(t, http.StatusOK, w.Code)
}

func TestCovVFSChildrenPaging(t *testing.T) {
	mux, _, snap, _ := covServer(t)
	defer snap.Close()
	id := covSnapID(snap)

	// Default listing of a directory with several entries.
	w := covGET(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Greater(t, len(items.Items), 0)

	// offset>0 path: exercises the ".." offset-decrement branch.
	w = covGET(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir?offset=1&limit=2")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// Explicit sort key.
	w = covGET(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir?sort=Name")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestCovVFSChildrenErrors(t *testing.T) {
	mux, _, snap, _ := covServer(t)
	defer snap.Close()
	id := covSnapID(snap)

	// children of a regular file -> 400 "not a directory".
	w := covGET(t, mux, "/api/snapshot/vfs/children/"+id+":/top.txt")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// invalid sort key -> 400.
	w = covGET(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir?sort=Bogus")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// invalid offset -> error.
	w = covGET(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir?offset=abc")
	require.NotEqual(t, http.StatusOK, w.Code)
}

// --- VFS chunks paging -----------------------------------------------------

func TestCovVFSChunksPaging(t *testing.T) {
	mux, _, snap, _ := covServer(t)
	defer snap.Close()
	id := covSnapID(snap)

	w := covGET(t, mux, "/api/snapshot/vfs/chunks/"+id+":/subdir/dummy.txt?offset=0&limit=10")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "total")

	// bad limit -> error.
	w = covGET(t, mux, "/api/snapshot/vfs/chunks/"+id+":/subdir/dummy.txt?limit=abc")
	require.NotEqual(t, http.StatusOK, w.Code)
}

// --- VFS errors handler ----------------------------------------------------

func TestCovVFSErrorsHandler(t *testing.T) {
	mux, _, snap, _ := covServer(t)
	defer snap.Close()
	id := covSnapID(snap)

	// happy path on a dir.
	w := covGET(t, mux, "/api/snapshot/vfs/errors/"+id+":/subdir?offset=0&limit=10")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// invalid sort key -> 400.
	w = covGET(t, mux, "/api/snapshot/vfs/errors/"+id+":/?sort=Bogus")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// -Name sort key is accepted.
	w = covGET(t, mux, "/api/snapshot/vfs/errors/"+id+":/?sort=-Name")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// errors of a regular file -> 400 not a directory.
	w = covGET(t, mux, "/api/snapshot/vfs/errors/"+id+":/top.txt")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

// --- VFS search paging / mime edges ----------------------------------------

func TestCovVFSSearchVariants(t *testing.T) {
	mux, _, snap, _ := covServer(t)
	defer snap.Close()
	id := covSnapID(snap)

	// recursive search with offset/limit and a name pattern.
	w := covGET(t, mux, "/api/snapshot/vfs/search/"+id+":/subdir?recursive=true&offset=0&limit=1&pattern=txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "has_next")

	// bad offset -> 400.
	w = covGET(t, mux, "/api/snapshot/vfs/search/"+id+":/subdir?offset=abc")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// bad limit -> 400.
	w = covGET(t, mux, "/api/snapshot/vfs/search/"+id+":/subdir?limit=abc")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

// --- Snapshot reader render error branch -----------------------------------

func TestCovReaderInvalidRender(t *testing.T) {
	mux, _, snap, _ := covServer(t)
	defer snap.Close()
	id := covSnapID(snap)

	w := covGET(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?render=invalid")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// code render path.
	w = covGET(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?render=code")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// reader for a non-existent file -> error.
	w = covGET(t, mux, "/api/snapshot/reader/"+id+":/nope.txt")
	require.NotEqual(t, http.StatusOK, w.Code)
}

// --- Downloader: signed-url error flows ------------------------------------

func TestCovDownloaderSignedNotFound(t *testing.T) {
	mux, _, snap, _ := covServer(t)
	defer snap.Close()

	// Unknown download id -> 404.
	w := covGET(t, mux, "/api/snapshot/vfs/downloader-sign-url/does-not-exist?format=zip")
	require.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

func TestCovDownloaderFormats(t *testing.T) {
	mux, _, snap, _ := covServer(t)
	defer snap.Close()
	id := covSnapID(snap)

	post := func() string {
		body := `{"name":"dl","items":[{"pathname":"/subdir/dummy.txt"}]}`
		req, _ := http.NewRequest("POST", "/api/snapshot/vfs/downloader/"+id+":/subdir", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		var resp struct {
			Id string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.NotEmpty(t, resp.Id)
		return resp.Id
	}

	// tar format.
	w := covGET(t, mux, "/api/snapshot/vfs/downloader-sign-url/"+post()+"?format=tar")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Header().Get("Content-Type"), "tar")

	// tarball format.
	w = covGET(t, mux, "/api/snapshot/vfs/downloader-sign-url/"+post()+"?format=tarball")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// unknown format -> 400.
	w = covGET(t, mux, "/api/snapshot/vfs/downloader-sign-url/"+post()+"?format=bogus")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

func TestCovDownloaderBadBody(t *testing.T) {
	mux, _, snap, _ := covServer(t)
	defer snap.Close()
	id := covSnapID(snap)

	// malformed JSON body -> 400.
	req, _ := http.NewRequest("POST", "/api/snapshot/vfs/downloader/"+id+":/subdir", bytes.NewBufferString("{not-json"))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

// --- Repository snapshots: filters, sort, offset/limit edges ---------------

func TestCovRepositorySnapshotsFilters(t *testing.T) {
	mux, _, snap, _ := covServer(t)
	defer snap.Close()

	// importer filter that matches nothing -> total counts only matching ones.
	w := covGET(t, mux, "/api/repository/snapshots?importer=nonexistent")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// since in the future -> no matching snapshots but still 200.
	w = covGET(t, mux, "/api/repository/snapshots?since=2999-01-01T00:00:00Z")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// descending sort.
	w = covGET(t, mux, "/api/repository/snapshots?sort=-Timestamp")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// offset beyond the number of headers -> empty items.
	w = covGET(t, mux, "/api/repository/snapshots?offset=1000")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// invalid sort key -> 400.
	w = covGET(t, mux, "/api/repository/snapshots?sort=Bogus")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// invalid offset -> error.
	w = covGET(t, mux, "/api/repository/snapshots?offset=abc")
	require.NotEqual(t, http.StatusOK, w.Code)
}

// --- Repository locate-pathname: filters and sort --------------------------

func TestCovLocatePathnameFilters(t *testing.T) {
	mux, _, snap, _ := covServer(t)
	defer snap.Close()

	// Resource that exists in the snapshot.
	w := covGET(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt&sort=-Timestamp")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "total")

	// importerType filter that matches nothing.
	w = covGET(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt&importerType=nope")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// importerOrigin filter that matches nothing.
	w = covGET(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt&importerOrigin=nope")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// importerDirectory filter that matches nothing.
	w = covGET(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt&importerDirectory=nope")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// Resource that does not resolve in any snapshot -> empty result, 200.
	w = covGET(t, mux, "/api/repository/locate-pathname?resource=/no/such/file")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// invalid sort key -> 400.
	w = covGET(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt&sort=Bogus")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// offset beyond results.
	w = covGET(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt&offset=1000")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

// --- Importer types --------------------------------------------------------

func TestCovImporterTypes(t *testing.T) {
	mux, _, snap, _ := covServer(t)
	defer snap.Close()

	w := covGET(t, mux, "/api/repository/importer-types")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[map[string]string]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Equal(t, items.Total, len(items.Items))
}
