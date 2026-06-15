package repair

import (
	"bytes"
	"os"
	"testing"

	_ "github.com/PlakarKorp/integrations/fs/exporter"
	ptesting "github.com/PlakarKorp/plakar/testing"
	"github.com/PlakarKorp/plakar/subcommands"
	"github.com/stretchr/testify/require"
)

func init() {
	os.Setenv("TZ", "UTC")
}

func TestRepairRegisteredFactory(t *testing.T) {
	cmd, _, _ := subcommands.Lookup([]string{"repair"})
	require.NotNil(t, cmd)
	require.IsType(t, &Repair{}, cmd)
}

func TestRepairParse(t *testing.T) {
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	_ = repo

	cmd := &Repair{}
	require.NoError(t, cmd.Parse(ctx, []string{}))
	require.False(t, cmd.Apply)

	cmd = &Repair{}
	require.NoError(t, cmd.Parse(ctx, []string{"-apply"}))
	require.True(t, cmd.Apply)
}

func TestRepairExecuteDryRun(t *testing.T) {
	// Without -apply, repair rebuilds and validates state against the live
	// repository (no lock, no daemon) and reports success.
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockFile("subdir/a.txt", 0644, "hello"),
	})
	defer snap.Close()

	cmd := &Repair{}
	require.NoError(t, cmd.Parse(ctx, []string{}))
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)
}
