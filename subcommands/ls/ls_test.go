package ls

import (
	"bytes"
	"encoding/hex"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/kloset/snapshot"
	"github.com/PlakarKorp/plakar/appcontext"
	ptesting "github.com/PlakarKorp/plakar/testing"
	"github.com/stretchr/testify/require"
)

func init() {
	os.Setenv("TZ", "UTC")
}

func generateSnapshot(t *testing.T) (*repository.Repository, *snapshot.Snapshot, *appcontext.AppContext) {
	repo, ctx := ptesting.GenerateRepository(t, nil, nil, nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockFile("subdir/dummy.txt", 0644, "hello dummy"),
	})
	return repo, snap, ctx
}

func TestExecuteCmdLsDefault(t *testing.T) {
	// Create a pipe to capture stdout
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	repo, snap, ctx := generateSnapshot(t)
	defer snap.Close()

	args := []string{}

	subcommand := &Ls{}
	err = subcommand.Parse(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, subcommand)

	status, err := subcommand.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)

	// Close the write end of the pipe and restore stdout
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	output := buf.String()
	lines := strings.Split(strings.Trim(output, "\n"), "\n")
	require.Equal(t, 1, len(lines))
	fields := strings.Fields(lines[0])
	require.Equal(t, 6, len(fields))
	require.Equal(t, snap.Header.Timestamp.Local().Format(time.RFC3339), fields[0])
	require.Equal(t, hex.EncodeToString(snap.Header.GetIndexShortID()), fields[1])
	require.Equal(t, snap.Header.GetSource(0).Importer.Directory, fields[len(fields)-1])
}

func TestExecuteCmdLsFilterByIDAndRecursive(t *testing.T) {
	// Create a pipe to capture stdout
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	repo, snap, ctx := generateSnapshot(t)
	defer snap.Close()

	args := []string{"-recursive", hex.EncodeToString(snap.Header.GetIndexShortID())}

	subcommand := &Ls{}
	err = subcommand.Parse(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, subcommand)

	status, err := subcommand.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)

	// Close the write end of the pipe and restore stdout
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	output := buf.String()
	lines := strings.Split(strings.Trim(output, "\n"), "\n")
	require.Equal(t, 2, len(lines))
	// last line should have the filename we backed up
	lastline := lines[len(lines)-1]
	fields := strings.Fields(lastline)
	require.Equal(t, 7, len(fields))
	// disable timestamp testing because it can make the test flaky if the test ran in the last second
	// require.Equal(t, snap.Header.Timestamp.Local().Format(time.RFC3339), fields[0])
	require.Equal(t, "/subdir/dummy.txt", fields[len(fields)-1])
}

func TestLsParseTooManyArguments(t *testing.T) {
	_, snap, ctx := generateSnapshot(t)
	defer snap.Close()

	cmd := &Ls{}
	err := cmd.Parse(ctx, []string{"a", "b"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "too many arguments")
}

func TestLsListSnapshotBadPath(t *testing.T) {
	// Listing a snapshot:path that resolves to no snapshot surfaces an error
	// (and a status of 1) from Execute.
	repo, snap, ctx := generateSnapshot(t)
	defer snap.Close()

	cmd := &Ls{}
	require.NoError(t, cmd.Parse(ctx, []string{"deadbeefdeadbeef:/subdir"}))
	status, err := cmd.Execute(ctx, repo)
	require.Error(t, err)
	require.Equal(t, 1, status)
}

func TestLsListSnapshotNonRecursive(t *testing.T) {
	// Non-recursive listing of a snapshot path exercises the entryname=Name()
	// branch and the SkipDir return for nested directories.
	bufOut := bytes.NewBuffer(nil)
	repo, ctx := ptesting.GenerateRepository(t, bufOut, bytes.NewBuffer(nil), nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockDir("subdir/nested"),
		ptesting.NewMockFile("subdir/dummy.txt", 0644, "hello dummy"),
		ptesting.NewMockFile("subdir/nested/deep.txt", 0644, "deep"),
	})
	defer snap.Close()

	cmd := &Ls{}
	require.NoError(t, cmd.Parse(ctx, []string{hex.EncodeToString(snap.Header.GetIndexShortID()) + ":/subdir"}))
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)

	out := bufOut.String()
	// dummy.txt is listed by base name; the nested directory's contents are not
	// descended into (SkipDir), so deep.txt must be absent.
	require.Contains(t, out, "dummy.txt")
	require.NotContains(t, out, "deep.txt")
}

func TestLsListSnapshotCancelledContext(t *testing.T) {
	// A cancelled context makes the WalkDir callback bail out with the context
	// error.
	repo, snap, ctx := generateSnapshot(t)
	defer snap.Close()

	cmd := &Ls{}
	require.NoError(t, cmd.Parse(ctx, []string{hex.EncodeToString(snap.Header.GetIndexShortID()) + ":/"}))

	ctx.GetInner().Cancel(nil)
	status, err := cmd.Execute(ctx, repo)
	require.Error(t, err)
	require.Equal(t, 1, status)
}

func TestExecuteCmdLsFilterUuid(t *testing.T) {
	// Create a pipe to capture stdout
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	repo, snap, ctx := generateSnapshot(t)
	defer snap.Close()

	args := []string{"-uuid"}

	subcommand := &Ls{}
	err = subcommand.Parse(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, subcommand)

	status, err := subcommand.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)

	// Close the write end of the pipe and restore stdout
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	output := buf.String()
	lines := strings.Split(strings.Trim(output, "\n"), "\n")
	require.Equal(t, 1, len(lines))
	fields := strings.Fields(lines[0])
	require.Equal(t, 6, len(fields))
	require.Equal(t, snap.Header.Timestamp.Local().Format(time.RFC3339), fields[0])
	indexId := snap.Header.GetIndexID()
	require.Equal(t, hex.EncodeToString(indexId[:]), fields[1])
	require.Equal(t, snap.Header.GetSource(0).Importer.Directory, fields[len(fields)-1])
}
