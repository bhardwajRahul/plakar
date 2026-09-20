package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// fsLoc returns an absolute fs:/// location pointing at a fresh temp dir.
// ---------- source add/set/unset/rm/show lifecycle ----------

func TestCovSourceLifecycle(t *testing.T) {
	ctx, bufOut, _ := newCtx(t)
	require.NoError(t, dispatchSubcommand(ctx, "source", "add", []string{"s", "fs:/tmp/s", "k=v"}))
	require.True(t, ctx.Config.HasSource("s"))
	require.Equal(t, "v", ctx.Config.Sources["s"]["k"])

	require.NoError(t, dispatchSubcommand(ctx, "source", "set", []string{"s", "k2=v2"}))
	require.Equal(t, "v2", ctx.Config.Sources["s"]["k2"])

	require.NoError(t, dispatchSubcommand(ctx, "source", "unset", []string{"s", "k2"}))
	_, ok := ctx.Config.Sources["s"]["k2"]
	require.False(t, ok)

	bufOut.Reset()
	require.NoError(t, dispatchSubcommand(ctx, "source", "show", []string{"s"}))
	require.Contains(t, bufOut.String(), "s")

	require.NoError(t, dispatchSubcommand(ctx, "source", "rm", []string{"s"}))
	require.False(t, ctx.Config.HasSource("s"))
}

// ---------- destination add/set/unset/rm/show lifecycle ----------

func TestCovDestinationLifecycle(t *testing.T) {
	ctx, bufOut, _ := newCtx(t)
	require.NoError(t, dispatchSubcommand(ctx, "destination", "add", []string{"d", "fs:/tmp/d", "k=v"}))
	require.True(t, ctx.Config.HasDestination("d"))

	require.NoError(t, dispatchSubcommand(ctx, "destination", "set", []string{"d", "k2=v2"}))
	require.Equal(t, "v2", ctx.Config.Destinations["d"]["k2"])

	require.NoError(t, dispatchSubcommand(ctx, "destination", "unset", []string{"d", "k2"}))
	_, ok := ctx.Config.Destinations["d"]["k2"]
	require.False(t, ok)

	bufOut.Reset()
	require.NoError(t, dispatchSubcommand(ctx, "destination", "show", []string{"-yaml", "d"}))
	require.Contains(t, bufOut.String(), "d")

	require.NoError(t, dispatchSubcommand(ctx, "destination", "rm", []string{"d"}))
	require.False(t, ctx.Config.HasDestination("d"))
}
