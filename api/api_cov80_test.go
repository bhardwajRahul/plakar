package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	_ "github.com/PlakarKorp/integrations/fs/exporter"
	"github.com/PlakarKorp/pkg"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/stretchr/testify/require"
)

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
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	// No auth token in the cookie jar -> handler returns a 401 JSON body.
	w := get(t, mux, "/api/proxy/v1/account/services/alerting")
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "authorization_error", resp["error"])
}

func TestCov80AlertingSetUnauthorized(t *testing.T) {
	mux, _, snap, _ := server(t, "")
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
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	w := get(t, mux, "/api/proxy/v1/integration/some-id/some/path")
	// Returns a plain error -> mapped to 500 by handleError.
	require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
}

// --- integrationsInstall: malformed JSON body (pre-PkgManager) -------------

func TestCov80IntegrationsInstallBadBody(t *testing.T) {
	mux, _, snap, _ := server(t, "")
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
	mux, _, snap, ctx := server(t, "")
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
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// "d" holds four .txt files; recursive search with limit=1 forces the
	// "one extra item -> HasNext" branch (limit is incremented internally).
	w := get(t, mux, "/api/snapshot/vfs/search/"+id+":/subdir?recursive=true&limit=1&pattern=txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var page ItemsPage[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.True(t, page.HasNext)
	require.Len(t, page.Items, 1)
}

func TestCov80VFSSearchMimeFilter(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// A single mime filter exercises the Mimes pass-through (not the >20 cap).
	w := get(t, mux, "/api/snapshot/vfs/search/"+id+":/subdir?recursive=true&mime=text/plain")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "has_next")
}

// --- snapshotVFSErrors: paging window break branch -------------------------

func TestCov80VFSErrorsWindowBreak(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// offset 0 / limit 1 over a clean directory exercises the i>=offset+limit
	// break path in the error iterator window arithmetic.
	w := get(t, mux, "/api/snapshot/vfs/errors/"+id+":/subdir?offset=0&limit=1")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "total")
}

// --- snapshotVFSDownloaderSigned: default generated name (no name param) ---

func TestCov80DownloaderSignedDefaultName(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

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

	// No name query param -> handler synthesizes "snapshot-<id>-<ts>" + ext.
	w = get(t, mux, "/api/snapshot/vfs/downloader-sign-url/"+resp.Id+"?format=zip")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Header().Get("Content-Disposition"), "snapshot-")
	require.Contains(t, w.Header().Get("Content-Disposition"), ".zip")
}

// --- snapshotVFSDownloader: bad snapshot id in path ------------------------

func TestCov80DownloaderBadSnapshotID(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	// Empty snapshot id segment -> SnapshotPathParam returns a 400.
	body := `{"name":"dl","items":[{"pathname":"/subdir/dummy.txt}]}`
	req, _ := http.NewRequest("POST", "/api/snapshot/vfs/downloader/:/subdir", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

// --- repositoryLocatePathname: exact offset/limit window -------------------

func TestCov80LocatePathnameExactWindow(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	// A resource that resolves in the single snapshot. With limit=1 and offset=0,
	// offset+limit == len(locations), exercising the exact-window slice branch
	// (locations[offset:offset+limit]) rather than the tail branch.
	w := get(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt&limit=1&offset=0")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.GreaterOrEqual(t, items.Total, 1)
}

func TestCov80LocatePathnameDefaultSortAsc(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	// No explicit sort -> default ascending Timestamp sortFunc branch.
	w := get(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "total")
}

// --- repositorySnapshots: exact offset/limit window ------------------------

func TestCov80RepositorySnapshotsExactWindow(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	// limit=1 with a single snapshot drives offset+limit == len(headers), the
	// exact-window slice branch of repositorySnapshots.
	w := get(t, mux, "/api/repository/snapshots?limit=1&offset=0")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.GreaterOrEqual(t, items.Total, 1)
}

// --- apiInfo: demo-mode env branch -----------------------------------------

func TestCov80ApiInfoDemoMode(t *testing.T) {
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

// --- snapshotVFSChildren: descending sort + paging over a real dir ---------

func TestCov80VFSChildrenDescSort(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	w := get(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir?sort=-Name&offset=0&limit=2")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Greater(t, len(items.Items), 0)
}
