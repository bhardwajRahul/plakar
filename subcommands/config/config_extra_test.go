package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/subcommands"
	"github.com/stretchr/testify/require"
)

func TestConfigRegisteredFactories(t *testing.T) {
	// Look each command up through the registry to invoke the factory closures
	// registered in init().
	cases := []struct {
		name string
		typ  interface{}
	}{
		{"store", &ConfigStoreCmd{}},
		{"source", &ConfigSourceCmd{}},
		{"destination", &ConfigDestinationCmd{}},
		{"policy", &ConfigPolicyCmd{}},
	}
	for _, c := range cases {
		cmd, _, _ := subcommands.Lookup([]string{c.name})
		require.NotNil(t, cmd, "command %q not registered", c.name)
		require.IsType(t, c.typ, cmd)
	}
}

// newConfigCtx returns a context with an empty on-disk config rooted in a temp
// dir, plus buffered stdout/stderr.
// ---------- helpers ----------

func TestNormalizeHelpers(t *testing.T) {
	require.Equal(t, "name", normalizeName("@name"))
	require.Equal(t, "name", normalizeName("name"))
	require.Equal(t, "fs:/x", normalizeLocation("location=fs:/x"))
	require.Equal(t, "fs:/x", normalizeLocation("fs:/x"))
}

func TestMarshalINISections(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, MarshalINISections("mysection", map[string]string{"key": "val"}, &buf))
	out := buf.String()
	require.Contains(t, out, "[mysection]")
	require.Contains(t, out, "key")
	require.Contains(t, out, "val")
}

func TestDispatchUnknownCmd(t *testing.T) {
	ctx, _, _ := newCtx(t)
	err := dispatchSubcommand(ctx, "bogus", "show", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown cmd")
}

// ---------- store / source / destination entity wrappers ----------

func TestEntityParseNoAction(t *testing.T) {
	ctx, _, _ := newCtx(t)

	require.Error(t, (&ConfigStoreCmd{}).Parse(ctx, []string{}))
	require.Error(t, (&ConfigSourceCmd{}).Parse(ctx, []string{}))
	require.Error(t, (&ConfigDestinationCmd{}).Parse(ctx, []string{}))
	require.Error(t, (&ConfigPolicyCmd{}).Parse(ctx, []string{}))
}

func TestDestinationParseExecute(t *testing.T) {
	ctx, _, _ := newCtx(t)
	repo := &repository.Repository{}

	cmd := &ConfigDestinationCmd{}
	require.NoError(t, cmd.Parse(ctx, []string{"add", "mydest", "fs:/tmp/dst"}))
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)
	require.True(t, ctx.Config.HasDestination("mydest"))

	// A failing dispatch (rm of unknown) propagates status 1.
	cmd = &ConfigDestinationCmd{}
	require.NoError(t, cmd.Parse(ctx, []string{"rm", "ghost"}))
	status, err = cmd.Execute(ctx, repo)
	require.Error(t, err)
	require.Equal(t, 1, status)
}

func TestSourceParseExecute(t *testing.T) {
	ctx, _, _ := newCtx(t)
	repo := &repository.Repository{}

	cmd := &ConfigSourceCmd{}
	require.NoError(t, cmd.Parse(ctx, []string{"add", "mysrc", "fs:/tmp/src"}))
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)
	require.True(t, ctx.Config.HasSource("mysrc"))
}

// ---------- dispatchSubcommand actions ----------

func TestPolicyParseExecute(t *testing.T) {
	ctx, _, _ := newCtx(t)
	repo := &repository.Repository{}

	cmd := &ConfigPolicyCmd{}
	require.NoError(t, cmd.Parse(ctx, []string{"add", "nightly"}))
	status, err := cmd.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)

	// policies.yml was written.
	_, err = os.Stat(filepath.Join(ctx.ConfigDir, "policies.yml"))
	require.NoError(t, err)
}
