package locate

import (
	"bytes"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	_ "github.com/PlakarKorp/integrations/fs/exporter"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/kloset/snapshot"
	"github.com/PlakarKorp/plakar/appcontext"
	ptesting "github.com/PlakarKorp/plakar/testing"
	"github.com/stretchr/testify/require"
)

func init() {
	os.Setenv("TZ", "UTC")
}

func generateSnapshot(t *testing.T, bufOut *bytes.Buffer, bufErr *bytes.Buffer) (*repository.Repository, *snapshot.Snapshot, *appcontext.AppContext) {
	repo, ctx := ptesting.GenerateRepository(t, bufOut, bufErr, nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockDir("another_subdir"),
		ptesting.NewMockFile("subdir/dummy.txt", 0644, "hello dummy"),
		ptesting.NewMockFile("subdir/foo.txt", 0644, "hello foo"),
		ptesting.NewMockFile("subdir/to_exclude", 0644, "*/subdir/to_exclude\n"),
		ptesting.NewMockFile("another_subdir/bar.txt", 0644, "hello bar"),
	})
	return repo, snap, ctx
}

func TestExecuteCmdLocateDefault(t *testing.T) {
	bufOut := bytes.NewBuffer(nil)
	bufErr := bytes.NewBuffer(nil)

	repo, snap, ctx := generateSnapshot(t, bufOut, bufErr)
	defer snap.Close()

	args := []string{"dummy.txt"}

	subcommand := &Locate{}
	err := subcommand.Parse(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, subcommand)

	status, err := subcommand.Execute(ctx, repo)
	require.NoError(t, err)
	require.NotNil(t, status)

	// output should look like this
	// d92a4c73:/subdir/dummy.txt

	output := bufOut.String()
	lines := strings.Split(strings.Trim(output, "\n"), "\n")
	require.Equal(t, 1, len(lines))
}

func TestLocateParseSnapshotWithFiltersWarns(t *testing.T) {
	bufOut := bytes.NewBuffer(nil)
	bufErr := bytes.NewBuffer(nil)
	repo, snap, ctx := generateSnapshot(t, bufOut, bufErr)
	defer snap.Close()
	_ = repo

	cmd := &Locate{}
	// Both -snapshot and a filter (-name) set: Parse warns that filters are
	// ignored.
	require.NoError(t, cmd.Parse(ctx, []string{"-snapshot", "abc", "-name", "x", "dummy.txt"}))
	require.Contains(t, bufErr.String(), "filters will be ignored")
}

func TestLocateGlobMatch(t *testing.T) {
	// A glob pattern goes through path.Match rather than the exact-base branch.
	bufOut := bytes.NewBuffer(nil)
	repo, snap, ctx := generateSnapshot(t, bufOut, bytes.NewBuffer(nil))
	defer snap.Close()

	cmd := &Locate{}
	require.NoError(t, cmd.Parse(ctx, []string{"*.txt"}))
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)
	// dummy.txt, foo.txt and bar.txt all match *.txt
	require.Contains(t, bufOut.String(), "dummy.txt")
	require.Contains(t, bufOut.String(), "bar.txt")
}

func TestLocateBadGlobPattern(t *testing.T) {
	// A malformed glob pattern surfaces a path.Match error.
	repo, snap, ctx := generateSnapshot(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil))
	defer snap.Close()

	cmd := &Locate{}
	require.NoError(t, cmd.Parse(ctx, []string{"[invalid"}))
	status, err := cmd.Execute(ctx, repo)
	require.Error(t, err)
	require.Equal(t, 1, status)
	require.Contains(t, err.Error(), "match pattern")
}

func TestLocateCancelledContext(t *testing.T) {
	repo, snap, ctx := generateSnapshot(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil))
	defer snap.Close()

	cmd := &Locate{}
	require.NoError(t, cmd.Parse(ctx, []string{"dummy.txt"}))
	ctx.GetInner().Cancel(nil)
	status, err := cmd.Execute(ctx, repo)
	require.Error(t, err)
	require.Equal(t, 1, status)
}

func TestExecuteCmdLocateWithSnapshotId(t *testing.T) {
	bufOut := bytes.NewBuffer(nil)
	bufErr := bytes.NewBuffer(nil)

	repo, snap, ctx := generateSnapshot(t, bufOut, bufErr)
	defer snap.Close()

	args := []string{"-snapshot", hex.EncodeToString(snap.Header.GetIndexShortID()), "dummy.txt"}

	subcommand := &Locate{}
	err := subcommand.Parse(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, subcommand)

	status, err := subcommand.Execute(ctx, repo)
	require.NoError(t, err)
	require.NotNil(t, status)

	// output should look like this
	// d92a4c73:/tmp/tmp_to_backup1424943315/subdir/dummy.txt

	output := bufOut.String()
	lines := strings.Split(strings.Trim(output, "\n"), "\n")
	require.Equal(t, 1, len(lines))
}
