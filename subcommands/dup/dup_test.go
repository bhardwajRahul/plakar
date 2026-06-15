package dup

import (
	"bytes"
	"encoding/hex"
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

func TestDupRegisteredFactory(t *testing.T) {
	cmd, _, _ := subcommands.Lookup([]string{"dup"})
	require.NotNil(t, cmd)
	require.IsType(t, &Dup{}, cmd)
}

func TestDupParseNoArgs(t *testing.T) {
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	_ = repo
	cmd := &Dup{}
	err := cmd.Parse(ctx, []string{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one parameter")
}

func TestDupParseArgs(t *testing.T) {
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	_ = repo
	cmd := &Dup{}
	require.NoError(t, cmd.Parse(ctx, []string{"a", "b"}))
	require.Equal(t, []string{"a", "b"}, cmd.SnapshotIDS)
}

func TestDupExecute(t *testing.T) {
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("subdir"),
		ptesting.NewMockFile("subdir/a.txt", 0644, "hello"),
	})
	defer snap.Close()

	id := snap.Header.GetIndexID()
	cmd := &Dup{}
	require.NoError(t, cmd.Parse(ctx, []string{hex.EncodeToString(id[:])}))
	// Execute duplicates the snapshot in-memory (snap.Dup) and closes the copy;
	// it completes successfully with no errors logged.
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)
}

func TestDupExecuteBadSnapshot(t *testing.T) {
	bufErr := bytes.NewBuffer(nil)
	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bufErr, nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockFile("a.txt", 0644, "x"),
	})
	defer snap.Close()

	cmd := &Dup{}
	require.NoError(t, cmd.Parse(ctx, []string{"deadbeefdeadbeef"}))
	// An unresolvable snapshot is logged and counted, but Execute still returns 0.
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)
	require.Contains(t, bufErr.String(), "digest:")
}
