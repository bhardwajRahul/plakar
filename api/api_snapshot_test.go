package api

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestSnapshotPathParamMissingID(t *testing.T) {
	_, repo, snap, _ := server(t, "")
	defer snap.Close()

	req, _ := http.NewRequest("GET", "/x/{snapshot_path}", nil)
	req.SetPathValue("snapshot_path", "")
	_, _, err := SnapshotPathParam(req, repo, "snapshot_path")
	require.Error(t, err)
	apierr, ok := err.(*ApiError)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, apierr.HttpCode)
}

func TestSnapshotHeader(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	id := snap.Header.GetIndexID()
	w := get(t, mux, "/api/snapshot/"+hex.EncodeToString(id[:]))
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotHeaderBadID(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	w := get(t, mux, "/api/snapshot/nothexnothex")
	require.NotEqual(t, http.StatusOK, w.Code)
}

func TestSnapshotHeaderNotFound(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	// mangle the snapid so that it does not match
	snapid := snap.Header.GetIndexID()

	// cheating a bit, but the meaning here is to just mangle the
	// id to generate a non-existent one.
	if snapid[1] = snapid[1] + 1; snapid[1] > '9' {
		snapid[1] = '0'
	}

	// Valid hex, valid length, but no such snapshot -> 404.
	w := get(t, mux, "/api/snapshot/"+hex.EncodeToString(snapid[:]))
	require.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotHeaderBadParam(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	w := get(t, mux, "/api/snapshot/zz")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotVFSSearch(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	indexID := snap.Header.GetIndexID()
	id := hex.EncodeToString(indexID[:])
	w := get(t, mux, "/api/snapshot/vfs/search/"+id+":/subdir?recursive=true")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotVFSSearchHasNext(t *testing.T) {
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

func TestSnapshotVFSSearchMimeFilter(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// A single mime filter exercises the Mimes pass-through (not the >20 cap).
	w := get(t, mux, "/api/snapshot/vfs/search/"+id+":/subdir?recursive=true&mime=text/plain")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "has_next")
}

func TestSnapshotVFSSearchNonRecursive(t *testing.T) {
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

func TestSnapshotVFSSearchTooManyMimes(t *testing.T) {
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

func TestSnapshotVFSSearchVariants(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// recursive search with offset/limit and a name pattern.
	w := get(t, mux, "/api/snapshot/vfs/search/"+id+":/subdir?recursive=true&offset=0&limit=1&pattern=txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "has_next")

	// bad offset -> 400.
	w = get(t, mux, "/api/snapshot/vfs/search/"+id+":/subdir?offset=abc")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// bad limit -> 400.
	w = get(t, mux, "/api/snapshot/vfs/search/"+id+":/subdir?limit=abc")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

func TestVFSDownloaderSignedDefaultName(t *testing.T) {
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

func TestVFSDownloaderBadSnapshotID(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	// Empty snapshot id segment -> SnapshotPathParam returns a 400.
	body := `{"name":"dl","items":[{"pathname":"/subdir/dummy.txt"}]}`
	req, _ := http.NewRequest("POST", "/api/snapshot/vfs/downloader/:/subdir", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

func TestVFSChildrenDescSort(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	w := get(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir?sort=-Name&offset=0&limit=2")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Greater(t, len(items.Items), 0)
}

func TestVFSBrowseRootAndFile(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// Root directory (path empty -> "/").
	w := get(t, mux, "/api/snapshot/vfs/"+id+":/")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// directory -> loadEntrySummaries path runs.
	w = get(t, mux, "/api/snapshot/vfs/"+id+":/subdir")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// A regular file entry.
	w = get(t, mux, "/api/snapshot/vfs/"+id+":/top.txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// Non-existent path -> error (not 200).
	w = get(t, mux, "/api/snapshot/vfs/"+id+":/does/not/exist")
	require.NotEqual(t, http.StatusOK, w.Code)
}

func TestVFSBrowseUnknownPrefix(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	// A well-formed but unmatched snapshot prefix -> LocateSnapshotByPrefix
	// error surfaced as a 400 invalid_params.
	w := get(t, mux, "/api/snapshot/vfs/deadbeef:/subdir")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

func TestVFSChildrenSecondPage(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// offset>0 takes the branch that decrements offset for the implicit "..".
	w := get(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir?offset=1&limit=5")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
}

func TestVFSChildrenLimitDecrementToZero(t *testing.T) {
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

// TestVFSChildrenRootNoParent exercises the root-directory path where
// no ".." entry is prepended (fsinfo.Path() == "/").
func TestVFSChildrenRoot(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	w := get(t, mux, "/api/snapshot/vfs/children/"+id+":/")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Greater(t, len(items.Items), 0)
}

func TestSnapshotVFSChildren(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	indexID := snap.Header.GetIndexID()
	id := hex.EncodeToString(indexID[:])
	w := get(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestVFSChildrenPaging(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// Default listing of a directory with several entries.
	w := get(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[json.RawMessage]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Greater(t, len(items.Items), 0)

	// offset>0 path: exercises the ".." offset-decrement branch.
	w = get(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir?offset=1&limit=2")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// Explicit sort key.
	w = get(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir?sort=Name")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestVFSChildrenErrors(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// children of a regular file -> 400 "not a directory".
	w := get(t, mux, "/api/snapshot/vfs/children/"+id+":/top.txt")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// invalid sort key -> 400.
	w = get(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir?sort=Bogus")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// invalid offset -> error.
	w = get(t, mux, "/api/snapshot/vfs/children/"+id+":/subdir?offset=abc")
	require.NotEqual(t, http.StatusOK, w.Code)
}

func TestSnapshotVFSErrors(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	indexID := snap.Header.GetIndexID()
	id := hex.EncodeToString(indexID[:])
	w := get(t, mux, "/api/snapshot/vfs/errors/"+id+":/")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestVFSErrorsWindowBreak(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// offset 0 / limit 1 over a clean directory exercises the i>=offset+limit
	// break path in the error iterator window arithmetic.
	w := get(t, mux, "/api/snapshot/vfs/errors/"+id+":/subdir?offset=0&limit=1")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "total")
}

func TestVFSErrorsPaging(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// Exercise the offset/limit window arithmetic on a clean (no-error) dir.
	w := get(t, mux, "/api/snapshot/vfs/errors/"+id+":/subdir?offset=0&limit=1")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// bad offset -> error from QueryParamToInt64.
	w = get(t, mux, "/api/snapshot/vfs/errors/"+id+":/subdir?offset=-1")
	require.NotEqual(t, http.StatusOK, w.Code)

	// errors on a missing dir -> 404.
	w = get(t, mux, "/api/snapshot/vfs/errors/"+id+":/no-such-dir")
	require.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

func TestVFSErrorsSortAndPaging(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// explicit Name sort + a paging window over an error-free directory.
	w := get(t, mux, "/api/snapshot/vfs/errors/"+id+":/subdir?sort=Name&offset=0&limit=5")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "total")
}

func TestVFSErrorsHandler(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// happy path on a dir.
	w := get(t, mux, "/api/snapshot/vfs/errors/"+id+":/subdir?offset=0&limit=10")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// invalid sort key -> 400.
	w = get(t, mux, "/api/snapshot/vfs/errors/"+id+":/?sort=Bogus")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// -Name sort key is accepted.
	w = get(t, mux, "/api/snapshot/vfs/errors/"+id+":/?sort=-Name")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// errors of a regular file -> 400 not a directory.
	w = get(t, mux, "/api/snapshot/vfs/errors/"+id+":/top.txt")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotReaderSignedReaderValid(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := server(t, token)
	defer snap.Close()
	id := snapid(snap)

	sig := signReader(t, mux, token, id+":/subdir/dummy.txt")

	// A valid signature lets the (otherwise token-protected) reader through.
	w := get(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?signature="+sig)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "hello dummy")
}

func TestSnapshotReaderSignedReaderTamperedPathAndSnapshot(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := server(t, token)
	defer snap.Close()
	id := snapid(snap)

	sig := signReader(t, mux, token, id+":/subdir/dummy.txt")

	// Same valid signature but requesting a different path -> rejected.
	w := get(t, mux, "/api/snapshot/reader/"+id+":/top.txt?signature="+sig)
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotReaderSignedReaderBadSignature(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := server(t, token)
	defer snap.Close()
	id := snapid(snap)

	// Garbage signature -> JWT parse failure -> 401.
	w := get(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?signature=not-a-jwt")
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotReaderSignedReaderExpired(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := server(t, token)
	defer snap.Close()
	id := snapid(snap)

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

	w := get(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?signature="+sig)
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "expired")
}

func TestSnapshotReaderSignedReaderWrongSigningMethod(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := server(t, token)
	defer snap.Close()
	id := snapid(snap)

	// A token with the "none" alg should be rejected by the method check.
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodNone, SnapshotSignedURLClaims{
		SnapshotID: id,
		Path:       "/subdir/dummy.txt",
	})
	sig, err := jwtToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	w := get(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?signature="+sig)
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotReaderNoSignatureRequiresToken(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := server(t, token)
	defer snap.Close()
	id := snapid(snap)

	// No signature and no Authorization header -> token middleware rejects.
	w := get(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt")
	require.Equal(t, http.StatusUnauthorized, w.Code, "body=%s", w.Body.String())

	// With the correct Bearer token it succeeds.
	req, _ := http.NewRequest("GET", "/api/snapshot/reader/"+id+":/subdir/dummy.txt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotReaderSignURL(t *testing.T) {
	mux, _, snap, _ := server(t, "")
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

func TestSnapshotReaderSignReaderNonexistentPath(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := server(t, token)
	defer snap.Close()
	id := snapid(snap)

	// Signing a path that does not exist in the snapshot returns an error.
	req, _ := http.NewRequest("POST", "/api/snapshot/reader-sign-url/"+id+":/no/such/file", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.NotEqual(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotReaderSignReaderBadSnapshotID(t *testing.T) {
	const token = "verify-token"
	mux, _, snap, _ := server(t, token)
	defer snap.Close()

	// Empty snapshot id segment -> SnapshotPathParam missing-arg error.
	req, _ := http.NewRequest("POST", "/api/snapshot/reader-sign-url/:/subdir/dummy.txt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotReaderMissingFileNotFound(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// GetEntry on a missing path bubbles up an fs.ErrNotExist which handleError
	// maps to a 404.
	w := get(t, mux, "/api/snapshot/reader/"+id+":/missing.txt")
	require.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotReaderRenderVariants(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	indexID := snap.Header.GetIndexID()
	id := hex.EncodeToString(indexID[:])
	base := "/api/snapshot/reader/" + id + ":"

	for _, render := range []string{"", "auto", "text", "text_styled", "code"} {
		u := base + "/subdir/dummy.txt"
		if render != "" {
			u += "?render=" + render
		}
		w := get(t, mux, u)
		require.Equal(t, http.StatusOK, w.Code, "render=%s body=%s", render, w.Body.String())

		switch render {
		case "", "auto":
			require.Contains(t, w.Header().Get("Content-type"), "text/plain")
			require.Contains(t, w.Body.String(), "hello dummy")
		case "text":
			require.Contains(t, w.Header().Get("Content-type"), "text/plain")
			require.Contains(t, w.Body.String(), "hello dummy")
		case "text_styled":
			require.Contains(t, w.Header().Get("Content-type"), "text/html")
			require.Contains(t, w.Body.String(), "<pre>")
		case "code":
			require.Contains(t, w.Header().Get("Content-type"), "text/html")
			require.Contains(t, w.Body.String(), "<!DOCTYPE html>")
		}

		// "noext" has no file extension
		w = get(t, mux, strings.Replace(u, "dummy.txt", "noext", 1))
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	}

	base += "/subdir/dummy.txt"

	// download=true sets a Content-Disposition attachment header.
	w := get(t, mux, base+"?download=true")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Disposition"), "attachment")

	// an invalid render value is rejected.
	w = get(t, mux, base+"?render=bogus")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSnapshotReaderRenderAuto(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// No render param defaults to "auto".
	w := get(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "hello dummy")
}

// TestSnapshotReaderDownloadDisposition exercises the download=true branch which
// sets a Content-Disposition attachment header.
func TestSnapshotReaderDownloadDisposition(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	w := get(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?download=true")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	require.Contains(t, w.Header().Get("Content-Disposition"), "dummy.txt")
}

func TestSnapshotReaderInvalidRender(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	w := get(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?render=invalid")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// code render path.
	w = get(t, mux, "/api/snapshot/reader/"+id+":/subdir/dummy.txt?render=code")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// reader for a non-existent file -> error.
	w = get(t, mux, "/api/snapshot/reader/"+id+":/nope.txt")
	require.NotEqual(t, http.StatusOK, w.Code)
}

func TestSnapshotDownloaderSignedCustomName(t *testing.T) {
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

	// name already carries an extension -> the ext-appending branch is skipped.
	w = get(t, mux, "/api/snapshot/vfs/downloader-sign-url/"+resp.Id+"?format=zip&name=custom.zip")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Header().Get("Content-Disposition"), "custom.zip")
}

func TestSnapshotDownloaderSignedNotFound(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	// Unknown download id -> 404.
	w := get(t, mux, "/api/snapshot/vfs/downloader-sign-url/does-not-exist?format=zip")
	require.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotDownloaderFormats(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

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
	w := get(t, mux, "/api/snapshot/vfs/downloader-sign-url/"+post()+"?format=tar")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Header().Get("Content-Type"), "tar")

	// tarball format.
	w = get(t, mux, "/api/snapshot/vfs/downloader-sign-url/"+post()+"?format=tarball")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// unknown format -> 400.
	w = get(t, mux, "/api/snapshot/vfs/downloader-sign-url/"+post()+"?format=bogus")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotDownloaderBadBody(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	// malformed JSON body -> 400.
	req, _ := http.NewRequest("POST", "/api/snapshot/vfs/downloader/"+id+":/subdir", bytes.NewBufferString("{not-json"))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotDownloaderFlow(t *testing.T) {
	mux, _, snap, _ := server(t, "")
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
	w = get(t, mux, "/api/snapshot/vfs/downloader-sign-url/"+resp.Id)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// With a valid archive format it serves the bundle. (Re-POST first, as the
	// id is single-use once consumed.)
	req, err = http.NewRequest("POST", "/api/snapshot/vfs/downloader/"+id+":/subdir", bytes.NewBufferString(body))
	require.NoError(t, err)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	w = get(t, mux, "/api/snapshot/vfs/downloader-sign-url/"+resp.Id+"?format=zip")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotVFSChunks(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	indexID := snap.Header.GetIndexID()
	id := hex.EncodeToString(indexID[:])
	w := get(t, mux, "/api/snapshot/vfs/chunks/"+id+":/subdir/dummy.txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

func TestSnapshotVFSChunksOffset(t *testing.T) {
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

func TestSnapshotVFSChunksPaging(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()
	id := snapid(snap)

	w := get(t, mux, "/api/snapshot/vfs/chunks/"+id+":/subdir/dummy.txt?offset=0&limit=10")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "total")

	// bad limit -> error.
	w = get(t, mux, "/api/snapshot/vfs/chunks/"+id+":/subdir/dummy.txt?limit=abc")
	require.NotEqual(t, http.StatusOK, w.Code)
}

// TestHandleErrorPassesApiError verifies an existing *ApiError is
// forwarded verbatim (its HttpCode and ErrCode preserved) rather than
// remapped.
func TestHandleErrorPassesApiError(t *testing.T) {
	req, _ := http.NewRequest("GET", "/x", nil)
	w := httptest.NewRecorder()
	handleError(w, req, &ApiError{HttpCode: http.StatusTeapot, ErrCode: "teapot", Message: "short and stout"})
	require.Equal(t, http.StatusTeapot, w.Code)

	var body ApiErrorRes
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "teapot", body.Error.ErrCode)
}
