package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	ptesting "github.com/PlakarKorp/plakar/testing"
	"github.com/stretchr/testify/require"
)

// TestRepositoryEmptyStateOK confirms the snapshot-listing handlers
// serve an empty result (200) over a valid but empty repository,
// exercising the no-snapshot loop tails.
func TestRepositoryEmptyStateOK(t *testing.T) {
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	mux := http.NewServeMux()
	SetupRoutes(mux, repo, ctx, "", true)

	for _, path := range []string{
		"/api/repository/snapshots",
		"/api/repository/importer-types",
		"/api/repository/locate-pathname",
		"/api/repository/info",
	} {
		w := get(t, mux, path)
		require.Equal(t, http.StatusOK, w.Code, "path=%s body=%s", path, w.Body.String())
	}
}

func TestRepositoryLocatePathnameExactWindow(t *testing.T) {
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

func TestRepositoryLocatePathnameDefaultSortAsc(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	// No explicit sort -> default ascending Timestamp sortFunc branch.
	w := get(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "total")
}

func TestRepositoryLocatePathnameFilters(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	// Resource that exists in the snapshot.
	w := get(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt&sort=-Timestamp")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "total")

	// importerType filter that matches nothing.
	w = get(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt&importerType=nope")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// importerOrigin filter that matches nothing.
	w = get(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt&importerOrigin=nope")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// importerDirectory filter that matches nothing.
	w = get(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt&importerDirectory=nope")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// Resource that does not resolve in any snapshot -> empty result, 200.
	w = get(t, mux, "/api/repository/locate-pathname?resource=/no/such/file")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// invalid sort key -> 400.
	w = get(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt&sort=Bogus")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// offset beyond results.
	w = get(t, mux, "/api/repository/locate-pathname?resource=/subdir/dummy.txt&offset=1000")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}

// TestRepositorySnapshotsParamErrors covers the parameter-validation error
// returns in repositorySnapshots (offset/limit/since/sort) which short-circuit
// before any storage access.
func TestRepositorySnapshotsParamErrors(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

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
			w := get(t, mux, fmt.Sprintf("/api/repository/snapshots?%s", c.query))
			require.Equal(t, c.status, w.Code, "body=%s", w.Body.String())
		})
	}
}

// TestRepositoryLocatePathnameParamErrors covers the parameter-validation
// error returns in repositoryLocatePathname.
func TestRepositoryLocatePathnameParamErrors(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	for _, q := range []string{"offset=abc", "limit=abc", "sort=NoSuchKey"} {
		t.Run(q, func(t *testing.T) {
			w := get(t, mux, "/api/repository/locate-pathname?"+q)
			require.GreaterOrEqual(t, w.Code, 400, "body=%s", w.Body.String())
		})
	}
}

func TestRepositorySnapshots(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	w := get(t, mux, "/api/repository/snapshots")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "total")
}

func TestAPIRepositorySnapshotsBadSince(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	w := get(t, mux, "/api/repository/snapshots?since=not-a-date")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRepositorySnapshotsExactWindow(t *testing.T) {
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

func TestRepositorySnapshotsImporterMatch(t *testing.T) {
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

func TestCovRepositorySnapshotsFilters(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	// importer filter that matches nothing -> total counts only matching ones.
	w := get(t, mux, "/api/repository/snapshots?importer=nonexistent")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// since in the future -> no matching snapshots but still 200.
	w = get(t, mux, "/api/repository/snapshots?since=2999-01-01T00:00:00Z")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// descending sort.
	w = get(t, mux, "/api/repository/snapshots?sort=-Timestamp")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// offset beyond the number of headers -> empty items.
	w = get(t, mux, "/api/repository/snapshots?offset=1000")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// invalid sort key -> 400.
	w = get(t, mux, "/api/repository/snapshots?sort=Bogus")
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// invalid offset -> error.
	w = get(t, mux, "/api/repository/snapshots?offset=abc")
	require.NotEqual(t, http.StatusOK, w.Code)
}

// TestRepositoryInfo exercises the populated-efficiency
// branch (logicalSize > 0) and asserts the response shape.
func TestRepositoryInfo(t *testing.T) {
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

func TestRepositoryInfoEmptyEfficiency(t *testing.T) {
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

func TestRepositoryImporterTypes(t *testing.T) {
	mux, _, snap, _ := server(t, "")
	defer snap.Close()

	w := get(t, mux, "/api/repository/importer-types")
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var items Items[map[string]string]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Equal(t, items.Total, len(items.Items))
}
