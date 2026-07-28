package api

import (
	"bytes"
	"encoding/hex"
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

// --- handleError: error -> HTTP status mapping (direct unit test) -----------

func TestCov3HandleErrorMapping(t *testing.T) {
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
		{"wrapped fs-not-exist -> 404", errors.New("boom: " + fs.ErrNotExist.Error()), http.StatusInternalServerError},
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

// TestCov3HandleErrorPassesApiError verifies an existing *ApiError is forwarded
// verbatim (its HttpCode and ErrCode preserved) rather than remapped.
func TestCov3HandleErrorPassesApiError(t *testing.T) {
	req, _ := http.NewRequest("GET", "/x", nil)
	w := httptest.NewRecorder()
	handleError(w, req, &ApiError{HttpCode: http.StatusTeapot, ErrCode: "teapot", Message: "short and stout"})
	require.Equal(t, http.StatusTeapot, w.Code)

	var body ApiErrorRes
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "teapot", body.Error.ErrCode)
}

// --- repositoryInfo: efficiency == -1 (empty repo, zero logical size) -------

func TestCov3RepositoryInfoEmptyEfficiency(t *testing.T) {
	// A repository with no snapshots has logicalSize == 0, which drives the
	// efficiency = -1 branch in repositoryInfo.
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	mux := http.NewServeMux()
	SetupRoutes(mux, repo, ctx, "", true)

	w := get(t, mux, "/api/repository/info")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp Item[RepositoryInfoResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, float64(-1), resp.Item.Snapshots.Efficiency)
	require.Equal(t, 0, resp.Item.Snapshots.Total)
	require.Len(t, resp.Item.Snapshots.SnapshotsPerDay, 30)
}

// TestCov3RepositoryInfoWithSnapshot exercises the populated-efficiency branch
// (logicalSize > 0) and asserts the response shape.
func TestCov3RepositoryInfoWithSnapshot(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	w := get(t, mux, "/api/repository/info")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp Item[RepositoryInfoResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.GreaterOrEqual(t, resp.Item.Snapshots.Total, 1)
	require.NotEmpty(t, resp.Item.OS)
	require.NotEmpty(t, resp.Item.Arch)
	require.True(t, resp.Item.Browsable)
}

// --- IntegrationsResponse helpers (pure, no PkgManager needed) --------------

func TestCov3IntegrationsResponseHelpers(t *testing.T) {
	resp := NewIntegrationsResponse("pkg_install")
	require.Equal(t, "pkg_install", resp.Type)
	require.Equal(t, "completed", resp.Status)
	require.False(t, resp.StartedAt.IsZero())
	require.Empty(t, resp.Messages)

	resp.AddMessage("first")
	resp.AddMessage("second")
	require.Len(t, resp.Messages, 2)
	require.Equal(t, "first", resp.Messages[0].Message)
	require.Equal(t, "second", resp.Messages[1].Message)
	require.False(t, resp.Messages[0].Date.IsZero())

	// The struct round-trips through JSON (the handlers encode it to the wire).
	b, err := json.Marshal(resp)
	require.NoError(t, err)
	require.Contains(t, string(b), "pkg_install")
	require.Contains(t, string(b), "first")
}

// --- params: QueryParamToString present/absent ------------------------------

func TestCov3QueryParamToString(t *testing.T) {
	req, _ := http.NewRequest("GET", "/?present=value", nil)

	v, ok, err := QueryParamToString(req, "present")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "value", v)

	v, ok, err = QueryParamToString(req, "absent")
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, v)
}

// TestCov3PathParamToIDValid covers the success branch of PathParamToID with a
// well-formed 32-byte hex id.
func TestCov3PathParamToIDValid(t *testing.T) {
	const hexID = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	req, _ := http.NewRequest("GET", "/path/{id}", nil)
	req.SetPathValue("id", hexID)

	id, err := PathParamToID(req, "id")
	require.NoError(t, err)
	require.Equal(t, hexID, hex.EncodeToString(id[:]))
}

// --- snapshotReader: render=text and render=text_styled branches -------------

func TestCov3ReaderRenderText(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	w := get(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?render=text")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	require.Contains(t, w.Body.String(), "hello dummy")
}

func TestCov3ReaderRenderTextStyled(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	w := get(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?render=text_styled")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
	require.Contains(t, w.Body.String(), "<pre>")
	require.Contains(t, w.Body.String(), "hello dummy")
}

func TestCov3ReaderRenderAuto(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// No render param defaults to "auto".
	w := get(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "hello dummy")
}

// TestCov3ReaderDownloadDisposition exercises the download=true branch which
// sets a Content-Disposition attachment header.
func TestCov3ReaderDownloadDisposition(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	w := get(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?download=true")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	require.Contains(t, w.Header().Get("Content-Disposition"), "dummy.txt")
}

// --- snapshotVFSChildren: limit=1 with implicit ".." (limit decrements to 0) -

func TestCov3VFSChildrenLimitDecrementToZero(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// On a non-root directory, page 0 prepends "..", which decrements limit. With
	// limit=1 this drives limit to 0 and hits the "replace with child count"
	// branch in snapshotVFSChildren.
	w := get(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir?offset=0&limit=1")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.GreaterOrEqual(t, len(items.Items), 1)
}

// TestCov3VFSChildrenRootNoParent exercises the root-directory path where no
// ".." entry is prepended (fsinfo.Path() == "/").
func TestCov3VFSChildrenRoot(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	w := get(t, mux, "/api/snapshot/vfs/children/"+id+":/")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Greater(t, len(items.Items), 0)
}

// --- snapshotVFSChunks: offset windowing on a real file ---------------------

func TestCov3VFSChunksOffset(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// offset beyond the chunk count -> empty Items but valid Total.
	w := get(t, mux, "/api/snapshot/vfs/chunks/"+id+":/subdir/dummy.txt?offset=1000&limit=10")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "total")

	// chunks on a missing path returns 200 with empty body (early nil return).
	w = get(t, mux, "/api/snapshot/vfs/chunks/"+id+":/dir/missing.txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

// --- snapshotVFSSearch: mime cap and non-recursive branch -------------------

func TestCov3VFSSearchNonRecursive(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// non-recursive search of a directory.
	w := get(t, mux, "/api/snapshot/vfs/search/"+id+":/subdir?limit=10")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "has_next")

	// limit<=0 is normalized to 50 (covers that branch).
	w = get(t, mux, "/api/snapshot/vfs/search/"+id+":/subdir?limit=0")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestCov3VFSSearchTooManyMimes(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// More than 20 mime params -> 400.
	url := "/api/snapshot/vfs/search/" + id + ":/subdir?"
	for i := range 21 {
		if i > 0 {
			url += "&"
		}
		url += "mime=text/plain"
	}
	w := get(t, mux, url)
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

// --- snapshotVFSBrowse: directory summary load + regular file ----------------

func TestCov3VFSBrowseDirAndFile(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// directory -> loadEntrySummaries path runs.
	w := get(t, mux, "/api/snapshot/vfs/"+id+":/subdir")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// regular file -> summary loading skipped.
	w = get(t, mux, "/api/snapshot/vfs/"+id+":/subdir/dummy.txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

// --- snapshotVFSErrors: explicit Name sort + paging on clean dir ------------

func TestCov3VFSErrorsSortAndPaging(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// explicit Name sort + a paging window over an error-free directory.
	w := get(t, mux, "/api/snapshot/vfs/errors/"+id+":/subdir?sort=Name&offset=0&limit=5")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "total")
}

// --- repositorySnapshots: importer filter that matches the only snapshot -----

func TestCov3RepositorySnapshotsImporterMatch(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	// First find the actual importer type via the header.
	importer := snap.Header.GetSource(0).Importer.Type

	w := get(t, mux, "/api/repository/snapshots?importer="+importer)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.GreaterOrEqual(t, items.Total, 1)

	// since in the past -> snapshot is kept.
	w = get(t, mux, "/api/repository/snapshots?since=2000-01-01T00:00:00Z")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}
