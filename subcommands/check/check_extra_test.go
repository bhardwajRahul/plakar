package check

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/PlakarKorp/plakar/exitcodes"
	"github.com/PlakarKorp/plakar/ui/stdio"
	"github.com/stretchr/testify/require"
)

func TestCheckParseFastFlag(t *testing.T) {
	_, snap, ctx := generateSnapshot(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil))
	defer snap.Close()
	cmd := &Check{}
	require.NoError(t, cmd.Parse(ctx, []string{"-fast"}))
	require.True(t, cmd.FastCheck)
}

func TestCheckParseNoVerifyFlag(t *testing.T) {
	_, snap, ctx := generateSnapshot(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil))
	defer snap.Close()
	cmd := &Check{}
	require.NoError(t, cmd.Parse(ctx, []string{"-no-verify"}))
	require.True(t, cmd.NoVerify)
}

func TestCheckParseSnapshotsPositional(t *testing.T) {
	_, snap, ctx := generateSnapshot(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil))
	defer snap.Close()
	cmd := &Check{}
	require.NoError(t, cmd.Parse(ctx, []string{"abcd", "ef01"}))
	require.Equal(t, []string{"abcd", "ef01"}, cmd.Snapshots)
}

func TestCheckParseSnapshotWithFilterIsAccepted(t *testing.T) {
	// Combining filters with a positional snapshot is allowed (a warning is
	// emitted asynchronously by the event bus, but pin down Parse's contract
	// of accepting the combination here).
	_, snap, ctx := generateSnapshot(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil))
	defer snap.Close()
	cmd := &Check{}
	require.NoError(t, cmd.Parse(ctx, []string{"-name", "x", "abcd"}))
	require.Equal(t, []string{"abcd"}, cmd.Snapshots)
	require.Equal(t, "x", cmd.LocateOptions.Filters.Name)
}

func TestCheckExecuteFastMode(t *testing.T) {
	bufOut := bytes.NewBuffer(nil)
	bufErr := bytes.NewBuffer(nil)
	repo, snap, ctx := generateSnapshot(t, bufOut, bufErr)
	defer snap.Close()

	renderer := stdio.New(ctx)
	renderer.Run()
	t.Cleanup(func() { renderer.Wait() })
	t.Cleanup(ctx.Close)

	cmd := &Check{}
	require.NoError(t, cmd.Parse(ctx, []string{"-fast"}))
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)
}

func TestCheckExecuteNoVerifyMode(t *testing.T) {
	bufOut := bytes.NewBuffer(nil)
	bufErr := bytes.NewBuffer(nil)
	repo, snap, ctx := generateSnapshot(t, bufOut, bufErr)
	defer snap.Close()

	renderer := stdio.New(ctx)
	renderer.Run()
	t.Cleanup(func() { renderer.Wait() })
	t.Cleanup(ctx.Close)

	cmd := &Check{}
	require.NoError(t, cmd.Parse(ctx, []string{"-no-verify"}))
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)
}

func TestCheckInvalidSnapshotPrefixIsRejected(t *testing.T) {
	bufOut := bytes.NewBuffer(nil)
	bufErr := bytes.NewBuffer(nil)
	repo, snap, ctx := generateSnapshot(t, bufOut, bufErr)
	defer snap.Close()
	t.Cleanup(ctx.Close)

	cmd := &Check{}
	// "zz" is not valid hex — Execute should refuse before scanning.
	require.NoError(t, cmd.Parse(ctx, []string{"zz"}))
	status, err := cmd.Execute(ctx, repo)
	require.Error(t, err)
	require.Equal(t, 1, status)
	require.Contains(t, err.Error(), "invalid snapshot prefix")
}

func TestCheckSubpathRestriction(t *testing.T) {
	bufOut := bytes.NewBuffer(nil)
	bufErr := bytes.NewBuffer(nil)
	repo, snap, ctx := generateSnapshot(t, bufOut, bufErr)
	defer snap.Close()

	renderer := stdio.New(ctx)
	renderer.Run()
	t.Cleanup(func() { renderer.Wait() })
	t.Cleanup(ctx.Close)

	indexId := snap.Header.GetIndexID()
	arg := hex.EncodeToString(indexId[:]) + ":/subdir"

	cmd := &Check{}
	require.NoError(t, cmd.Parse(ctx, []string{arg}))
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)
}

func TestCheckExitCodeOnIntegrityFailureConstant(t *testing.T) {
	// Sanity: the package promises to surface exitcodes.IntegrityFailure when
	// at least one snapshot fails. Pin the constant down so a future refactor
	// can't quietly change the contract.
	require.Equal(t, 65, exitcodes.IntegrityFailure)
}

func TestCheckSnapshotPrefixWithNoMatchIsNoOp(t *testing.T) {
	// Pin down current behavior: a valid hex prefix that matches no snapshot
	// neither errors nor changes the exit code — the loop simply has no work
	// to do.
	bufOut := bytes.NewBuffer(nil)
	bufErr := bytes.NewBuffer(nil)
	repo, snap, ctx := generateSnapshot(t, bufOut, bufErr)
	defer snap.Close()

	renderer := stdio.New(ctx)
	renderer.Run()
	t.Cleanup(func() { renderer.Wait() })
	t.Cleanup(ctx.Close)

	cmd := &Check{}
	require.NoError(t, cmd.Parse(ctx, []string{"deadbeef"}))
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)
}
