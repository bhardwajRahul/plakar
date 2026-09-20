package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// import of a config that contains no usable sections must error with "no
// valid ...s found".
// ---------- policy add/set with an invalid value (config.Set error) ----------

func TestDispatchPolicyAddSetErrorCov80(t *testing.T) {
	ctx, _, _ := newCtx(t)
	// "minutes" expects a non-negative int; a garbage value makes config.Set
	// fail inside the policy add handler.
	err := dispatchPolicy(ctx, "policy", "add", []string{"p", "minutes=notanint"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to set key")
}

func TestDispatchPolicySetErrorCov80(t *testing.T) {
	ctx, _, _ := newCtx(t)
	require.NoError(t, dispatchPolicy(ctx, "policy", "add", []string{"p"}))
	err := dispatchPolicy(ctx, "policy", "set", []string{"p", "minutes=notanint"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to set key")
}

// ---------- policy show in both formats over a populated policy ----------

func TestDispatchPolicyShowYAMLAndJSONCov80(t *testing.T) {
	ctx, bufOut, _ := newCtx(t)
	require.NoError(t, dispatchPolicy(ctx, "policy", "add", []string{"keep", "days=7"}))

	// YAML (default)
	bufOut.Reset()
	require.NoError(t, dispatchPolicy(ctx, "policy", "show", []string{"keep"}))
	require.Contains(t, bufOut.String(), "keep")

	// JSON
	bufOut.Reset()
	require.NoError(t, dispatchPolicy(ctx, "policy", "show", []string{"-json", "keep"}))
	require.True(t, strings.HasPrefix(strings.TrimSpace(bufOut.String()), "{"))
}

// ---------- policy default subcommand (unknown action) ----------

func TestDispatchPolicyUnknownActionCov80(t *testing.T) {
	ctx, _, _ := newCtx(t)
	err := dispatchPolicy(ctx, "policy", "bogus", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage:")
}

// ---------- store show -ini format over a populated store ----------

func TestDispatchShowINIFormatCov80(t *testing.T) {
	ctx, bufOut, _ := newCtx(t)
	require.NoError(t, dispatchSubcommand(ctx, "store", "add",
		[]string{"r", "fs://" + t.TempDir(), "extra=val"}))
	bufOut.Reset()
	require.NoError(t, dispatchSubcommand(ctx, "store", "show", []string{"-ini", "r"}))
	out := bufOut.String()
	require.Contains(t, out, "[r]")
	require.Contains(t, out, "extra")
}
