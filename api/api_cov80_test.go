package api

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	_ "github.com/PlakarKorp/integrations/fs/exporter"
	"github.com/PlakarKorp/pkg"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/kloset/snapshot"
	"github.com/PlakarKorp/plakar/appcontext"
	ptesting "github.com/PlakarKorp/plakar/testing"
	"github.com/stretchr/testify/require"
)

// cov80Server builds a real fs-backed repository with a single rich snapshot and
// wires SetupRoutes with norefresh=true. Distinct helper name so it does not
// clash with the team's other coverage helpers.
func cov80Server(t *testing.T) (*http.ServeMux, *repository.Repository, *snapshot.Snapshot, *appcontext.AppContext) {
	t.Helper()
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("d"),
		ptesting.NewMockFile("d/one.txt", 0644, "one"),
		ptesting.NewMockFile("d/two.txt", 0644, "two"),
		ptesting.NewMockFile("d/three.txt", 0644, "three"),
		ptesting.NewMockFile("d/four.txt", 0644, "four"),
		ptesting.NewMockFile("top.txt", 0644, "top"),
	})
	mux := http.NewServeMux()
	SetupRoutes(mux, repo, ctx, "", true /* norefresh */)
	return mux, repo, snap, ctx
}

func cov80GET(t *testing.T, mux *http.ServeMux, url string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func cov80SnapID(snap *snapshot.Snapshot) string {
	id := snap.Header.GetIndexID()
	return hex.EncodeToString(id[:])
}

// attachPkgManager installs a real (but empty, hermetic) pkg.Manager onto ctx so
// the integration/uninstall handlers have a backend that resolves locally
// against an empty temp directory without touching the network.
func attachPkgManager(t *testing.T, ctx *appcontext.AppContext) {
	t.Helper()
	dir := t.TempDir()
	backend, err := pkg.NewFlatBackend(ctx.GetInner(),
		filepath.Join(dir, "plugins"), filepath.Join(dir, "cache"), &pkg.FlatBackendOptions{})
	require.NoError(t, err)
	mgr, err := pkg.New(backend, &pkg.Options{})
	require.NoError(t, err)
	ctx.SetPkgManager(mgr)
}

// --- Alerting service config: unauthenticated 401 branches -----------------

func TestCov80AlertingGetUnauthorized(t *testing.T) {
	mux, _, snap, _ := cov80Server(t)
	defer snap.Close()

	// No auth token in the cookie jar -> handler returns a 401 JSON body.
	w := cov80GET(t, mux, "/api/proxy/v1/account/services/alerting")
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "authorization_error", resp["error"])
}

func TestCov80AlertingSetUnauthorized(t *testing.T) {
	mux, _, snap, _ := cov80Server(t)
	defer snap.Close()

	body := `{"enabled":true,"email_report":true}`
	req, _ := http.NewRequest("PUT", "/api/proxy/v1/account/services/alerting", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "authorization_error", resp["error"])
}

// --- servicesGetIntegrationPath: always Not implemented --------------------

func TestCov80GetIntegrationPathNotImplemented(t *testing.T) {
	mux, _, snap, _ := cov80Server(t)
	defer snap.Close()

	w := cov80GET(t, mux, "/api/proxy/v1/integration/some-id/some/path")
	// Returns a plain error -> mapped to 500 by handleError.
	require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
}

// --- integrationsInstall: malformed JSON body (pre-PkgManager) -------------

func TestCov80IntegrationsInstallBadBody(t *testing.T) {
	mux, _, snap, _ := cov80Server(t)
	defer snap.Close()

	// Malformed body fails the JSON decode before any package manager call.
	// The handler still encodes a (failed) IntegrationsResponse with 200.
	req, _ := http.NewRequest("POST", "/api/integrations/install", bytes.NewBufferString("{not-json"))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp IntegrationsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "pkg_install", resp.Type)
	require.Equal(t, "failed", resp.Status)
	require.NotEmpty(t, resp.Messages)
}

// --- integrationsUninstall: unknown plugin via a real empty PkgManager -----

func TestCov80IntegrationsUninstall(t *testing.T) {
	mux, _, snap, ctx := cov80Server(t)
	defer snap.Close()
	attachPkgManager(t, ctx)

	req, _ := http.NewRequest("DELETE", "/api/integrations/some-plugin", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp IntegrationsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "pkg_uninstall", resp.Type)
	require.NotEmpty(t, resp.Messages)
}

// --- snapshotVFSSearch: HasNext pagination + name pattern ------------------

func TestCov80VFSSearchHasNext(t *testing.T) {
	mux, _, snap, _ := cov80Server(t)
	defer snap.Close()
	id := cov80SnapID(snap)

	// "d" holds four .txt files; recursive search with limit=1 forces the
	// "one extra item -> HasNext" branch (limit is incremented internally).
	w := cov80GET(t, mux, "/api/snapshot/vfs/search/"+id+":/d?recursive=true&limit=1&pattern=txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var page ItemsPage[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.True(t, page.HasNext)
	require.Len(t, page.Items, 1)
}

func TestCov80VFSSearchMimeFilter(t *testing.T) {
	mux, _, snap, _ := cov80Server(t)
	defer snap.Close()
	id := cov80SnapID(snap)

	// A single mime filter exercises the Mimes pass-through (not the >20 cap).
	w := cov80GET(t, mux, "/api/snapshot/vfs/search/"+id+":/d?recursive=true&mime=text/plain")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "has_next")
}

// --- snapshotVFSErrors: paging window break branch -------------------------

func TestCov80VFSErrorsWindowBreak(t *testing.T) {
	mux, _, snap, _ := cov80Server(t)
	defer snap.Close()
	id := cov80SnapID(snap)

	// offset 0 / limit 1 over a clean directory exercises the i>=offset+limit
	// break path in the error iterator window arithmetic.
	w := cov80GET(t, mux, "/api/snapshot/vfs/errors/"+id+":/d?offset=0&limit=1")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "total")
}

// --- snapshotVFSDownloaderSigned: default generated name (no name param) ---

func TestCov80DownloaderSignedDefaultName(t *testing.T) {
	mux, _, snap, _ := cov80Server(t)
	defer snap.Close()
	id := cov80SnapID(snap)

	body := `{"name":"dl","items":[{"pathname":"/d/one.txt"}]}`
	req, _ := http.NewRequest("POST", "/api/snapshot/vfs/downloader/"+id+":/d", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var resp struct {
		Id string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Id)

	// No name query param -> handler synthesizes "snapshot-<id>-<ts>" + ext.
	w = cov80GET(t, mux, "/api/snapshot/vfs/downloader-sign-url/"+resp.Id+"?format=zip")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Header().Get("Content-Disposition"), "snapshot-")
	require.Contains(t, w.Header().Get("Content-Disposition"), ".zip")
}

// --- snapshotVFSDownloader: bad snapshot id in path ------------------------

func TestCov80DownloaderBadSnapshotID(t *testing.T) {
	mux, _, snap, _ := cov80Server(t)
	defer snap.Close()

	// Empty snapshot id segment -> SnapshotPathParam returns a 400.
	body := `{"name":"dl","items":[{"pathname":"/d/one.txt"}]}`
	req, _ := http.NewRequest("POST", "/api/snapshot/vfs/downloader/:/d", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

// --- repositoryLocatePathname: exact offset/limit window -------------------

func TestCov80LocatePathnameExactWindow(t *testing.T) {
	mux, _, snap, _ := cov80Server(t)
	defer snap.Close()

	// A resource that resolves in the single snapshot. With limit=1 and offset=0,
	// offset+limit == len(locations), exercising the exact-window slice branch
	// (locations[offset:offset+limit]) rather than the tail branch.
	w := cov80GET(t, mux, "/api/repository/locate-pathname?resource=/d/one.txt&limit=1&offset=0")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.GreaterOrEqual(t, items.Total, 1)
}

func TestCov80LocatePathnameDefaultSortAsc(t *testing.T) {
	mux, _, snap, _ := cov80Server(t)
	defer snap.Close()

	// No explicit sort -> default ascending Timestamp sortFunc branch.
	w := cov80GET(t, mux, "/api/repository/locate-pathname?resource=/d/one.txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "total")
}

// --- repositorySnapshots: exact offset/limit window ------------------------

func TestCov80RepositorySnapshotsExactWindow(t *testing.T) {
	mux, _, snap, _ := cov80Server(t)
	defer snap.Close()

	// limit=1 with a single snapshot drives offset+limit == len(headers), the
	// exact-window slice branch of repositorySnapshots.
	w := cov80GET(t, mux, "/api/repository/snapshots?limit=1&offset=0")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.GreaterOrEqual(t, items.Total, 1)
}

// --- apiInfo: demo-mode env branch -----------------------------------------

func TestCov80ApiInfoDemoMode(t *testing.T) {
	t.Setenv("PLAKAR_DEMO_MODE", "true")
	mux, _, snap, _ := cov80Server(t)
	defer snap.Close()

	w := cov80GET(t, mux, "/api/info")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var resp struct {
		DemoMode bool `json:"demo_mode"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.DemoMode)
}

// --- snapshotVFSChildren: descending sort + paging over a real dir ---------

func TestCov80VFSChildrenDescSort(t *testing.T) {
	mux, _, snap, _ := cov80Server(t)
	defer snap.Close()
	id := cov80SnapID(snap)

	w := cov80GET(t, mux, "/api/snapshot/vfs/children/"+id+":/d?sort=-Name&offset=0&limit=2")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Greater(t, len(items.Items), 0)
}
