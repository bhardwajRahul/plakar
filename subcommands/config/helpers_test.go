package config

import (
	"bytes"
	"path/filepath"
	"testing"

	_ "github.com/PlakarKorp/integrations/fs/exporter"
	_ "github.com/PlakarKorp/integrations/fs/importer"
	_ "github.com/PlakarKorp/integrations/fs/storage"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/config"
	"github.com/stretchr/testify/require"
)

// newCtx builds an AppContext backed by an empty on-disk config in a temp dir,
// with stdout and stderr captured.
func newCtx(t *testing.T) (*appcontext.AppContext, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	tmpDir := t.TempDir()
	cfg, err := config.LoadOldConfigIfExists(filepath.Join(tmpDir, "config.yaml"))
	require.NoError(t, err)

	bufOut := bytes.NewBuffer(nil)
	bufErr := bytes.NewBuffer(nil)

	ctx := appcontext.NewAppContext()
	ctx.Config = cfg
	ctx.ConfigDir = tmpDir
	ctx.Stdout = bufOut
	ctx.Stderr = bufErr

	return ctx, bufOut, bufErr
}

// fsLoc returns an fs:// location backed by a fresh temp dir.
func fsLoc(t *testing.T) string {
	t.Helper()
	return "fs://" + t.TempDir()
}
