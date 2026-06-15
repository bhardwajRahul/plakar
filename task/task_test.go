package task

import (
	"bytes"
	"os"
	"testing"

	_ "github.com/PlakarKorp/integrations/fs/exporter"
	_ "github.com/PlakarKorp/integrations/fs/importer"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/appcontext"
	ptesting "github.com/PlakarKorp/plakar/testing"
	"github.com/PlakarKorp/plakar/ui/stdio"
	"github.com/stretchr/testify/require"

	"github.com/PlakarKorp/plakar/subcommands/check"
	"github.com/PlakarKorp/plakar/subcommands/ls"
	"github.com/PlakarKorp/plakar/subcommands/maintenance"
	"github.com/PlakarKorp/plakar/subcommands/restore"
	"github.com/PlakarKorp/plakar/subcommands/rm"
)

func init() {
	os.Setenv("TZ", "UTC")
}

// newRepoWithSnapshot returns a repository preloaded with a single snapshot and
// a context whose stdio renderer is draining the event bus, so commands that
// emit events (and the reporter) don't deadlock.
func newRepoWithSnapshot(t *testing.T) (*appcontext.AppContext, *repository.Repository) {
	t.Helper()
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockFile("subdir/dummy.txt", 0644, "hello dummy"),
	})
	snap.Close()

	startRenderer(t, ctx)
	return ctx, repo
}

func startRenderer(t *testing.T, ctx *appcontext.AppContext) {
	t.Helper()
	renderer := stdio.New(ctx)
	require.NoError(t, renderer.Run())
	t.Cleanup(func() { renderer.Wait() })
	t.Cleanup(ctx.Close)
}

func TestRunCommandCheck(t *testing.T) {
	ctx, repo := newRepoWithSnapshot(t)

	cmd := &check.Check{}
	require.NoError(t, cmd.Parse(ctx, []string{}))

	status, err := RunCommand(ctx, cmd, repo, "task-check")
	require.NoError(t, err)
	require.Equal(t, 0, status)
}

func TestRunCommandRm(t *testing.T) {
	ctx, repo := newRepoWithSnapshot(t)

	cmd := &rm.Rm{}
	require.NoError(t, cmd.Parse(ctx, []string{"-latest", "-apply"}))

	status, err := RunCommand(ctx, cmd, repo, "task-rm")
	require.NoError(t, err)
	require.Equal(t, 0, status)
}

// NOTE: the *backup.Backup branch of RunCommand (DoBackup + WithSnapshotID) is
// intentionally not tested here. backup wires a package-level stateRefresher
// that connects to the `cached` daemon; the backup package's own tests override
// that unexported hook with a stub, but it isn't reachable from this package, so
// a real backup deadlocks waiting on the daemon. Covering that branch would
// require the same override mechanism to be exported.

func TestRunCommandRestore(t *testing.T) {
	ctx, repo := newRepoWithSnapshot(t)

	cmd := &restore.Restore{}
	require.NoError(t, cmd.Parse(ctx, []string{"-to", t.TempDir()}))

	status, err := RunCommand(ctx, cmd, repo, "task-restore")
	require.NoError(t, err)
	require.Equal(t, 0, status)
}

func TestRunCommandMaintenance(t *testing.T) {
	ctx, repo := newRepoWithSnapshot(t)

	cmd := &maintenance.Maintenance{}
	require.NoError(t, cmd.Parse(ctx, []string{}))

	status, err := RunCommand(ctx, cmd, repo, "task-maintenance")
	require.NoError(t, err)
	require.Equal(t, 0, status)
}

func TestRunCommandDefaultKindIsIgnored(t *testing.T) {
	// ls is not one of the recognized task kinds, so the report is ignored.
	ctx, repo := newRepoWithSnapshot(t)

	cmd := &ls.Ls{}
	require.NoError(t, cmd.Parse(ctx, []string{}))

	status, err := RunCommand(ctx, cmd, repo, "task-ls")
	require.NoError(t, err)
	require.Equal(t, 0, status)
}

func TestRunCommandFailureReportsTaskFailed(t *testing.T) {
	// A recognized task kind (check) that fails drives the TaskFailed branch
	// (status != 0). Pointing check at a non-hex snapshot prefix makes Execute
	// return status 1.
	ctx, repo := newRepoWithSnapshot(t)

	cmd := &check.Check{}
	require.NoError(t, cmd.Parse(ctx, []string{"nothex!:/"}))

	status, err := RunCommand(ctx, cmd, repo, "task-check-fail")
	require.Error(t, err)
	require.NotEqual(t, 0, status)
}
