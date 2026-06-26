package api

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/PlakarKorp/integrations/fs/exporter"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/kloset/snapshot"
	"github.com/PlakarKorp/plakar/appcontext"
	ptesting "github.com/PlakarKorp/plakar/testing"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// cov2Server is a fresh real-fs server like covServer but lets the caller pick
// the auth token so signed-URL flows can be exercised end to end.
func cov2Server(t *testing.T, token string) (*http.ServeMux, *repository.Repository, *snapshot.Snapshot, *appcontext.AppContext) {
	t.Helper()
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockFile("subdir/dummy.txt", 0644, "hello dummy"),
		ptesting.NewMockFile("noext", 0644, "plain text no extension"),
		ptesting.NewMockFile("top.txt", 0644, "top level"),
	})
	mux := http.NewServeMux()
	SetupRoutes(mux, repo, ctx, token, true /* norefresh */)
	return mux, repo, snap, ctx
}

func cov2SnapID(snap *snapshot.Snapshot) string {
	id := snap.Header.GetIndexID()
	return hex.EncodeToString(id[:])
}

// --- apiInfo: authenticated branch -----------------------------------------

func TestCov2ApiInfoAuthenticated(t *testing.T) {
	mux, _, snap, ctx := cov2Server(t, "")
	defer snap.Close()

	// Drop an auth token into the cookie jar so apiInfo reports authenticated.
	require.NoError(t, ctx.GetCookies().PutAuthToken("some-auth-token"))

	w := covGET(t, mux, "/api/info")
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

// --- Signed-URL reader: full JWT verification flow -------------------------

// signReader posts to the sign endpoint (auth via Bearer token) and returns the
// JWT signature for a snapshot path.
func signReader(t *testing.T, mux *http.ServeMux, token, snapPath string) string {
	t.Helper()
	req, _ := http.NewRequest("POST", "/api/snapshot/reader-sign-url/"+snapPath, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var resp struct {
		Item struct {
			Signature string `json:"signature"`
		} `json:"item"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Item.Signature)
	return resp.Item.Signature
}

func TestCov2SignedReaderValid(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := cov2Server(t, token)
	defer snap.Close()
	id := cov2SnapID(snap)

	sig := signReader(t, mux, token, id+":/subdir/dummy.txt")

	// A valid signature lets the (otherwise token-protected) reader through.
	w := covGET(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?signature="+sig)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "hello dummy")
}

func TestCov2SignedReaderTamperedPathAndSnapshot(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := cov2Server(t, token)
	defer snap.Close()
	id := cov2SnapID(snap)

	sig := signReader(t, mux, token, id+":/subdir/dummy.txt")

	// Same valid signature but requesting a different path -> rejected.
	w := covGET(t, mux, "/api/snapshot/reader/"+id+":/top.txt?signature="+sig)
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())
}

func TestCov2SignedReaderBadSignature(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := cov2Server(t, token)
	defer snap.Close()
	id := cov2SnapID(snap)

	// Garbage signature -> JWT parse failure -> 401.
	w := covGET(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?signature=not-a-jwt")
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())
}

func TestCov2SignedReaderExpired(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := cov2Server(t, token)
	defer snap.Close()
	id := cov2SnapID(snap)

	// Hand-craft an already-expired token signed with the right key.
	now := time.Now().Add(-3 * time.Hour)
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, SnapshotSignedURLClaims{
		SnapshotID: id,
		Path:       "/subdir/dummy.txt",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "plakar-api",
		},
	})
	sig, err := jwtToken.SignedString([]byte(token))
	require.NoError(t, err)

	w := covGET(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?signature="+sig)
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "expired")
}

func TestCov2SignedReaderWrongSigningMethod(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := cov2Server(t, token)
	defer snap.Close()
	id := cov2SnapID(snap)

	// A token with the "none" alg should be rejected by the method check.
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodNone, SnapshotSignedURLClaims{
		SnapshotID: id,
		Path:       "/subdir/dummy.txt",
	})
	sig, err := jwtToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	w := covGET(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?signature="+sig)
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())
}

// --- Reader middleware fallthrough to token auth ---------------------------

func TestCov2ReaderNoSignatureRequiresToken(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := cov2Server(t, token)
	defer snap.Close()
	id := cov2SnapID(snap)

	// No signature and no Authorization header -> token middleware rejects.
	w := covGET(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt")
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())

	// With the correct Bearer token it succeeds.
	req, _ := http.NewRequest("GET", "/api/snapshot/reader/"+id+":/subdir/dummy.txt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

// --- Sign endpoint error branches ------------------------------------------

func TestCov2SignReaderNonexistentPath(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := cov2Server(t, token)
	defer snap.Close()
	id := cov2SnapID(snap)

	// Signing a path that does not exist in the snapshot returns an error.
	req, _ := http.NewRequest("POST", "/api/snapshot/reader-sign-url/"+id+":/no/such/file", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.NotEqual(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestCov2SignReaderBadSnapshotID(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := cov2Server(t, token)
	defer snap.Close()

	// Empty snapshot id segment -> SnapshotPathParam missing-arg error.
	req, _ := http.NewRequest("POST", "/api/snapshot/reader-sign-url/:/subdir/dummy.txt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

// --- renderCode: content-type lexer fallback (no extension) ----------------

func TestCov2RenderCodeNoExtension(t *testing.T) {
	mux, _, snap, _ := cov2Server(t, "")
	defer snap.Close()
	id := cov2SnapID(snap)

	// "noext" has no filename extension, so lexers.Match returns nil and the
	// handler falls back to the resolved content type / fallback lexer.
	w := covGET(t, mux, "/api/snapshot/reader/"+id+":/noext?render=code")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "<!DOCTYPE html>")
}

// --- handleError: not-found mapping via reader on missing file -------------

func TestCov2ReaderMissingFileNotFound(t *testing.T) {
	mux, _, snap, _ := cov2Server(t, "")
	defer snap.Close()
	id := cov2SnapID(snap)

	// GetEntry on a missing path bubbles up an fs.ErrNotExist which handleError
	// maps to a 404.
	w := covGET(t, mux, "/api/snapshot/reader/"+id+":/missing.txt")
	require.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

// --- SnapshotPathParam: invalid id prefix ----------------------------------

func TestCov2VFSBrowseUnknownPrefix(t *testing.T) {
	mux, _, snap, _ := cov2Server(t, "")
	defer snap.Close()

	// A well-formed but unmatched snapshot prefix -> LocateSnapshotByPrefix
	// error surfaced as a 400 invalid_params.
	w := covGET(t, mux, "/api/snapshot/vfs/deadbeef:/subdir")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

// --- VFS children: ".." parent paging on offset>0 page ---------------------

func TestCov2VFSChildrenSecondPage(t *testing.T) {
	mux, _, snap, _ := cov2Server(t, "")
	defer snap.Close()
	id := cov2SnapID(snap)

	// offset>0 takes the branch that decrements offset for the implicit "..".
	w := covGET(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir?offset=1&limit=5")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
}

// --- VFS errors: offset/limit windowing ------------------------------------

func TestCov2VFSErrorsPaging(t *testing.T) {
	mux, _, snap, _ := cov2Server(t, "")
	defer snap.Close()
	id := cov2SnapID(snap)

	// Exercise the offset/limit window arithmetic on a clean (no-error) dir.
	w := covGET(t, mux, "/api/snapshot/vfs/errors/"+id+":/subdir?offset=0&limit=1")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// bad offset -> error from QueryParamToInt64.
	w = covGET(t, mux, "/api/snapshot/vfs/errors/"+id+":/subdir?offset=-1")
	require.NotEqual(t, http.StatusOK, w.Code)

	// errors on a missing dir -> 404.
	w = covGET(t, mux, "/api/snapshot/vfs/errors/"+id+":/no-such-dir")
	require.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

// --- Downloader signed: custom name keeps extension behavior ---------------

func TestCov2DownloaderSignedCustomName(t *testing.T) {
	mux, _, snap, _ := cov2Server(t, "")
	defer snap.Close()
	id := cov2SnapID(snap)

	body := `{"name":"dl","items":[{"pathname":"/subdir/dummy.txt"}]}`
	req, _ := http.NewRequest("POST", "/api/snapshot/vfs/downloader/"+id+":/subdir", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var resp struct {
		Id string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// name already carries an extension -> the ext-appending branch is skipped.
	w = covGET(t, mux, "/api/snapshot/vfs/downloader-sign-url/"+resp.Id+"?format=zip&name=custom.zip")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Header().Get("Content-Disposition"), "custom.zip")
}

// --- SnapshotPathParam unit: missing id ------------------------------------

func TestCov2SnapshotPathParamMissingID(t *testing.T) {
	mux, repo, snap, _ := cov2Server(t, "")
	defer snap.Close()
	_ = mux

	req, _ := http.NewRequest("GET", "/x/{snapshot_path}", nil)
	req.SetPathValue("snapshot_path", "")
	_, _, err := SnapshotPathParam(req, repo, "snapshot_path")
	require.Error(t, err)
	apierr, ok := err.(*ApiError)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, apierr.HttpCode)
}
