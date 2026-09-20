package config

import (
	"testing"

	"github.com/PlakarKorp/kloset/repository"
	"github.com/stretchr/testify/require"
)

// ---------- Execute error path returns status 1 ----------

func TestCov2StoreExecuteErrorStatus2(t *testing.T) {
	ctx, _, _ := newCtx(t)
	cmd := &ConfigStoreCmd{}
	require.NoError(t, cmd.Parse(ctx, []string{"rm", "does-not-exist"}))
	status, err := cmd.Execute(ctx, &repository.Repository{})
	require.Error(t, err)
	require.Equal(t, 1, status)
}

func TestCov2SourceExecuteErrorStatus2(t *testing.T) {
	ctx, _, _ := newCtx(t)
	cmd := &ConfigSourceCmd{}
	require.NoError(t, cmd.Parse(ctx, []string{"rm", "nope"}))
	status, err := cmd.Execute(ctx, &repository.Repository{})
	require.Error(t, err)
	require.Equal(t, 1, status)
}

func TestCov2DestinationExecuteErrorStatus2(t *testing.T) {
	ctx, _, _ := newCtx(t)
	cmd := &ConfigDestinationCmd{}
	require.NoError(t, cmd.Parse(ctx, []string{"rm", "nope"}))
	status, err := cmd.Execute(ctx, &repository.Repository{})
	require.Error(t, err)
	require.Equal(t, 1, status)
}

func TestCov2PolicyExecuteErrorStatus2(t *testing.T) {
	ctx, _, _ := newCtx(t)
	cmd := &ConfigPolicyCmd{}
	require.NoError(t, cmd.Parse(ctx, []string{"rm", "nope"}))
	status, err := cmd.Execute(ctx, &repository.Repository{})
	require.Error(t, err)
	require.Equal(t, 1, status)
}

func TestCov2StoreExecuteSuccessStatus2(t *testing.T) {
	ctx, _, _ := newCtx(t)
	cmd := &ConfigStoreCmd{}
	require.NoError(t, cmd.Parse(ctx, []string{"add", "r", "fs://" + t.TempDir()}))
	status, err := cmd.Execute(ctx, &repository.Repository{})
	require.NoError(t, err)
	require.Equal(t, 0, status)
}
