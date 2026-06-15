package diff

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	_ "github.com/PlakarKorp/integrations/fs/exporter"
	ptesting "github.com/PlakarKorp/plakar/testing"
	"github.com/stretchr/testify/require"
)

func init() {
	os.Setenv("TZ", "UTC")
}

func TestDiffName(t *testing.T) {
	require.Equal(t, "diff", (&Diff{}).Name())
}

func TestDiffParseArgCounts(t *testing.T) {
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	_ = repo

	// One arg: Path1 set, Path2 empty.
	cmd := &Diff{}
	require.NoError(t, cmd.Parse(ctx, []string{"a"}))
	require.Equal(t, "a", cmd.Path1)
	require.Equal(t, "", cmd.Path2)

	// Two args: both set.
	cmd = &Diff{}
	require.NoError(t, cmd.Parse(ctx, []string{"a", "b"}))
	require.Equal(t, "a", cmd.Path1)
	require.Equal(t, "b", cmd.Path2)

	// Zero args: error.
	cmd = &Diff{}
	err := cmd.Parse(ctx, []string{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "needs at least a snapshot")
}

func TestDiffExecuteBadSnapshot1(t *testing.T) {
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockFile("a.txt", 0644, "a"),
	})
	defer snap.Close()

	cmd := &Diff{}
	require.NoError(t, cmd.Parse(ctx, []string{"deadbeefdeadbeef:/a.txt", "deadbeefdeadbeef:/a.txt"}))
	status, err := cmd.Execute(ctx, repo)
	require.Error(t, err)
	require.Equal(t, 1, status)
	require.Contains(t, err.Error(), "could not open snapshot")
}

func TestDiffExecuteBadSnapshot2(t *testing.T) {
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockFile("a.txt", 0644, "a"),
	})
	defer snap.Close()

	id := snap.Header.GetIndexShortID()
	cmd := &Diff{}
	require.NoError(t, cmd.Parse(ctx, []string{
		fmt.Sprintf("%s:/a.txt", hex.EncodeToString(id[:])),
		"deadbeefdeadbeef:/a.txt",
	}))
	status, err := cmd.Execute(ctx, repo)
	require.Error(t, err)
	require.Equal(t, 1, status)
}

func TestDiffDirectoriesFlat(t *testing.T) {
	bufOut := bytes.NewBuffer(nil)
	repo, ctx := ptesting.GenerateRepository(t, bufOut, bytes.NewBuffer(nil), nil)

	snap1 := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockFile("subdir/common.txt", 0644, "x"),
		ptesting.NewMockFile("subdir/only1.txt", 0644, "y"),
		ptesting.NewMockDir("subdir/nesteddir"),
	})
	defer snap1.Close()
	snap2 := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockFile("subdir/common.txt", 0644, "x"),
		ptesting.NewMockFile("subdir/only2.txt", 0644, "z"),
		ptesting.NewMockDir("subdir/nesteddir"),
	})
	defer snap2.Close()

	id1 := snap1.Header.GetIndexShortID()
	id2 := snap2.Header.GetIndexShortID()
	cmd := &Diff{}
	require.NoError(t, cmd.Parse(ctx, []string{
		fmt.Sprintf("%s:/subdir", hex.EncodeToString(id1[:])),
		fmt.Sprintf("%s:/subdir", hex.EncodeToString(id2[:])),
	}))
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)

	out := bufOut.String()
	require.Contains(t, out, "Common subdirectories: nesteddir")
	require.Contains(t, out, "only1.txt")
	require.Contains(t, out, "only2.txt")
}

func TestDiffDirectoriesRecursive(t *testing.T) {
	bufOut := bytes.NewBuffer(nil)
	repo, ctx := ptesting.GenerateRepository(t, bufOut, bytes.NewBuffer(nil), nil)

	snap1 := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockFile("subdir/a.txt", 0644, "one"),
	})
	defer snap1.Close()
	snap2 := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockFile("subdir/a.txt", 0644, "two"),
	})
	defer snap2.Close()

	id1 := snap1.Header.GetIndexShortID()
	id2 := snap2.Header.GetIndexShortID()
	cmd := &Diff{}
	require.NoError(t, cmd.Parse(ctx, []string{
		"-recursive",
		fmt.Sprintf("%s:/subdir", hex.EncodeToString(id1[:])),
		fmt.Sprintf("%s:/subdir", hex.EncodeToString(id2[:])),
	}))
	require.True(t, cmd.Recursive)
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)
}

func TestDiffMismatchedTypes(t *testing.T) {
	// One side is a directory, the other a file -> "can't diff different file
	// types".
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockFile("subdir/a.txt", 0644, "a"),
	})
	defer snap.Close()

	id := hex.EncodeToString(snap.Header.GetIndexShortID())
	cmd := &Diff{}
	require.NoError(t, cmd.Parse(ctx, []string{id + ":/subdir", id + ":/subdir/a.txt"}))
	status, err := cmd.Execute(ctx, repo)
	require.Error(t, err)
	require.Equal(t, 1, status)
	require.Contains(t, err.Error(), "different file types")
}

func TestDiffBinaryFilesDiffer(t *testing.T) {
	bufOut := bytes.NewBuffer(nil)
	repo, ctx := ptesting.GenerateRepository(t, bufOut, bytes.NewBuffer(nil), nil)

	// Content with NUL bytes is detected as binary.
	bin1 := string([]byte{0x00, 0x01, 0x02, 'a'})
	bin2 := string([]byte{0x00, 0x01, 0x02, 'b'})
	snap1 := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockFile("blob", 0644, bin1),
	})
	defer snap1.Close()
	snap2 := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockFile("blob", 0644, bin2),
	})
	defer snap2.Close()

	id1 := hex.EncodeToString(snap1.Header.GetIndexShortID())
	id2 := hex.EncodeToString(snap2.Header.GetIndexShortID())
	cmd := &Diff{}
	require.NoError(t, cmd.Parse(ctx, []string{id1 + ":/blob", id2 + ":/blob"}))
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)
	require.Contains(t, bufOut.String(), "Binary files")
}

func TestDiffHighlight(t *testing.T) {
	bufOut := bytes.NewBuffer(nil)
	repo, ctx := ptesting.GenerateRepository(t, bufOut, bytes.NewBuffer(nil), nil)

	snap1 := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockFile("a.txt", 0644, "hello\n"),
	})
	defer snap1.Close()
	snap2 := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockFile("a.txt", 0644, "world\n"),
	})
	defer snap2.Close()

	id1 := hex.EncodeToString(snap1.Header.GetIndexShortID())
	id2 := hex.EncodeToString(snap2.Header.GetIndexShortID())
	cmd := &Diff{}
	require.NoError(t, cmd.Parse(ctx, []string{"-highlight", id1 + ":/a.txt", id2 + ":/a.txt"}))
	require.True(t, cmd.Highlight)
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)
	require.NotEmpty(t, bufOut.String())
}

func TestExecuteCmdDiffIdentical(t *testing.T) {
	bufOut := bytes.NewBuffer(nil)
	bufErr := bytes.NewBuffer(nil)

	repo, ctx := ptesting.GenerateRepository(t, bufOut, bufErr, nil)

	// create one snapshot
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockDir("another_subdir"),
		ptesting.NewMockFile("subdir/dummy.txt", 0644, "hello dummy"),
		ptesting.NewMockFile("subdir/foo.txt", 0644, "hello foo"),
		ptesting.NewMockFile("subdir/to_exclude", 0644, "*/subdir/to_exclude\n"),
		ptesting.NewMockFile("another_subdir/bar", 0644, "hello bar"),
	})
	snap.Close()

	// create second snapshot
	snap2 := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockDir("another_subdir"),
		ptesting.NewMockFile("subdir/dummy.txt", 0644, "hello dummy"),
		ptesting.NewMockFile("subdir/foo.txt", 0644, "hello foo"),
		ptesting.NewMockFile("subdir/to_exclude", 0644, "*/subdir/to_exclude\n"),
		ptesting.NewMockFile("another_subdir/bar", 0644, "hello bar"),
	})
	snap2.Close()

	indexId1 := snap.Header.GetIndexShortID()
	indexId2 := snap2.Header.GetIndexShortID()
	snapPath1 := fmt.Sprintf("%s:/subdir/dummy.txt", hex.EncodeToString(indexId1[:]))
	snapPath2 := fmt.Sprintf("%s:/subdir/dummy.txt", hex.EncodeToString(indexId2[:]))
	args := []string{snapPath1, snapPath2}

	subcommand := &Diff{}
	err := subcommand.Parse(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, subcommand)

	status, err := subcommand.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)

	outputErr := bufErr.String()
	require.Contains(t, outputErr, "")
}

func TestExecuteCmdDiffFiles(t *testing.T) {
	bufOut := bytes.NewBuffer(nil)
	bufErr := bytes.NewBuffer(nil)

	repo, ctx := ptesting.GenerateRepository(t, bufOut, bufErr, nil)

	// create one snapshot
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockDir("another_subdir"),
		ptesting.NewMockFile("subdir/dummy.txt", 0644, "hello dummy"),
		ptesting.NewMockFile("subdir/foo.txt", 0644, "hello foo"),
		ptesting.NewMockFile("subdir/to_exclude", 0644, "*/subdir/to_exclude\n"),
		ptesting.NewMockFile("another_subdir/bar", 0644, "hello bar"),
	})
	defer snap.Close()

	// create second different snapshot
	snap2 := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockDir("another_subdir"),
		ptesting.NewMockFile("subdir/dummy.txt", 0644, "hello dummy!!"), // <- changed
		ptesting.NewMockFile("subdir/foo.txt", 0644, "hello foo"),
		ptesting.NewMockFile("subdir/to_exclude", 0644, "*/subdir/to_exclude\n"),
		ptesting.NewMockFile("another_subdir/bar", 0644, "hello bar"),
	})
	defer snap2.Close()

	indexId1 := snap.Header.GetIndexShortID()
	indexId2 := snap2.Header.GetIndexShortID()
	snapPath1 := fmt.Sprintf("%s:/subdir/dummy.txt", hex.EncodeToString(indexId1[:]))
	snapPath2 := fmt.Sprintf("%s:/subdir/dummy.txt", hex.EncodeToString(indexId2[:]))
	args := []string{snapPath1, snapPath2}

	subcommand := &Diff{}
	err := subcommand.Parse(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, subcommand)

	status, err := subcommand.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)

	output := bufOut.String()
	require.Contains(t, output, `
@@ -1 +1 @@
-hello dummy
+hello dummy!!`)
}
