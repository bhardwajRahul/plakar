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

// ---------- show: aggregate missing-name error & default masking ----------

func TestCov2ShowMissingNameAggregateError2(t *testing.T) {
	ctx, _, bufErr := newCtx(t)
	require.NoError(t, dispatchSubcommand(ctx, "store", "add", []string{"r", "fs:///x"}))
	err := dispatchSubcommand(ctx, "store", "show", []string{"r", "ghost"})
	require.Error(t, err)
	require.Contains(t, bufErr.String(), "does not exist")
}

func TestCov2ShowMasksSecretsByDefault2(t *testing.T) {
	ctx, bufOut, _ := newCtx(t)
	require.NoError(t, dispatchSubcommand(ctx, "store", "add",
		[]string{"r", "fs:///x", "secret_access_key=topsecret", "x_token=abc"}))
	bufOut.Reset()
	require.NoError(t, dispatchSubcommand(ctx, "store", "show", []string{"r"}))
	out := bufOut.String()
	require.Contains(t, out, "********")
	require.NotContains(t, out, "topsecret")
	require.NotContains(t, out, "abc")
}

func TestCov2ShowJSONFormat2(t *testing.T) {
	ctx, bufOut, _ := newCtx(t)
	require.NoError(t, dispatchSubcommand(ctx, "store", "add", []string{"r", "fs:///x"}))
	bufOut.Reset()
	require.NoError(t, dispatchSubcommand(ctx, "store", "show", []string{"-json", "r"}))
	require.Contains(t, bufOut.String(), "\"location\"")
}

// ---------- unset cannot remove location ----------

func TestCov2UnsetLocationRejected2(t *testing.T) {
	ctx, _, _ := newCtx(t)
	require.NoError(t, dispatchSubcommand(ctx, "store", "add", []string{"r", "fs:///x"}))
	err := dispatchSubcommand(ctx, "store", "unset", []string{"r", "location"})
	require.Error(t, err)
}
