package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/PlakarKorp/kloset/caching"
	"github.com/PlakarKorp/kloset/caching/pebble"
	"github.com/PlakarKorp/kloset/connectors/storage"
	"github.com/PlakarKorp/kloset/hashing"
	"github.com/PlakarKorp/kloset/logging"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/kloset/resources"
	"github.com/PlakarKorp/kloset/versioning"
	"github.com/PlakarKorp/plakar/appcontext"
	ptesting "github.com/PlakarKorp/plakar/testing"
	"github.com/stretchr/testify/require"
)

// XXX: re-add once we move to non-mocked state object.

func TestSnapshotHeaderErrors(t *testing.T) {
	testCases := []struct {
		name       string
		params     string
		location   string
		snapshotId string
		expected   string
		status     int
	}{
		{
			name:       "wrong snapshot id format",
			location:   "mock:///test/location",
			snapshotId: "abc",
			status:     http.StatusBadRequest,
		},
		{
			name:       "snapshot id valid but not found",
			location:   "mock:///test/location",
			snapshotId: "7e0e6e24a6e29faf11d022dca77826fe8b8a000aff5ea27e16650d03acefc93c",
			status:     http.StatusNotFound,
		},
	}

	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			tmpCacheDir, err := os.MkdirTemp("", "tmp_cache")
			require.NoError(t, err)
			t.Cleanup(func() {
				os.RemoveAll(tmpCacheDir)
			})

			config := ptesting.NewConfiguration()

			serializedConfig, err := config.ToBytes()
			require.NoError(t, err)

			hasher := hashing.GetHasher(hashing.DEFAULT_HASHING_ALGORITHM)
			wrappedConfigRd, err := storage.Serialize(hasher, resources.RT_CONFIG, versioning.GetCurrentVersion(resources.RT_CONFIG), bytes.NewReader(serializedConfig))
			require.NoError(t, err)

			wrappedConfig, err := io.ReadAll(wrappedConfigRd)
			require.NoError(t, err)

			ctx := appcontext.NewAppContext()
			cache := caching.NewManager(pebble.Constructor(tmpCacheDir))
			defer cache.Close()
			ctx.SetCache(cache)
			ctx.CacheDir = tmpCacheDir
			ctx.SetLogger(logging.NewLogger(os.Stdout, os.Stderr))
			ctx.Client = "plakar-test/1.0.0"

			lstore, err := storage.Create(ctx.GetInner(), map[string]string{"location": c.location}, wrappedConfig)
			require.NoError(t, err, "creating storage")
			repo, err := repository.New(ctx.GetInner(), nil, lstore, wrappedConfig)
			require.NoError(t, err, "creating repository")

			var noToken string
			mux := http.NewServeMux()
			SetupRoutes(mux, repo, ctx, noToken, false)

			req, err := http.NewRequest("GET", fmt.Sprintf("/api/snapshot/%s", c.snapshotId), nil)
			require.NoError(t, err, "creating request")

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			require.Equal(t, c.status, w.Code, fmt.Sprintf("expected status code %d", c.status))
		})
	}
}

// XXX: re-add once we move to non-mocked state object.
