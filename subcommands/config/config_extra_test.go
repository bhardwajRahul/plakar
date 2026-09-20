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

func TestDispatchAddDuplicateAndMalformed(t *testing.T) {
	ctx, _, _ := newCtx(t)

	require.NoError(t, dispatchSubcommand(ctx, "store", "add", []string{"r", "fs:/tmp/r"}))

	// duplicate name
	err := dispatchSubcommand(ctx, "store", "add", []string{"r", "fs:/tmp/r2"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")

	// too few args
	err = dispatchSubcommand(ctx, "store", "add", []string{"only-name"})
	require.Error(t, err)

	// malformed key=value
	err = dispatchSubcommand(ctx, "store", "add", []string{"r2", "fs:/tmp/r2", "noequalsign"})
	require.Error(t, err)
}

func TestDispatchAddRejectsNameWithSlash(t *testing.T) {
	const (
		configCommand     = "destination"
		configSubcommand  = "add"
		invalidConfigName = "s3://xxxx"
		configLocation    = "access_key=yyy"
		expectedError     = "invalid configuration name"
	)

	ctx, _, _ := newCtx(t)

	err := dispatchSubcommand(ctx, configCommand, configSubcommand, []string{invalidConfigName, configLocation})
	require.Error(t, err)
	require.Contains(t, err.Error(), expectedError)
	require.False(t, ctx.Config.HasDestination(invalidConfigName))
}

func TestDispatchAddWithOptions(t *testing.T) {
	ctx, _, _ := newCtx(t)
	require.NoError(t, dispatchSubcommand(ctx, "store", "add", []string{"r", "fs:/tmp/r", "key=val", "k2=v2"}))
	require.Equal(t, "val", ctx.Config.Repositories["r"]["key"])
	require.Equal(t, "v2", ctx.Config.Repositories["r"]["k2"])
}

func TestDispatchSetUnset(t *testing.T) {
	ctx, _, _ := newCtx(t)
	require.NoError(t, dispatchSubcommand(ctx, "store", "add", []string{"r", "fs:/tmp/r"}))

	// set on unknown name
	require.Error(t, dispatchSubcommand(ctx, "store", "set", []string{"ghost", "k=v"}))
	// set malformed
	require.Error(t, dispatchSubcommand(ctx, "store", "set", []string{"r", "bad"}))
	// set ok
	require.NoError(t, dispatchSubcommand(ctx, "store", "set", []string{"r", "k=v"}))
	require.Equal(t, "v", ctx.Config.Repositories["r"]["k"])

	// unset too few args
	require.Error(t, dispatchSubcommand(ctx, "store", "unset", []string{"r"}))
	// unset unknown name
	require.Error(t, dispatchSubcommand(ctx, "store", "unset", []string{"ghost", "k"}))
	// unset location is forbidden
	require.Error(t, dispatchSubcommand(ctx, "store", "unset", []string{"r", "location"}))
	// unset ok
	require.NoError(t, dispatchSubcommand(ctx, "store", "unset", []string{"r", "k"}))
	_, ok := ctx.Config.Repositories["r"]["k"]
	require.False(t, ok)
}

func TestDispatchRm(t *testing.T) {
	ctx, _, _ := newCtx(t)
	require.NoError(t, dispatchSubcommand(ctx, "store", "add", []string{"r", "fs:/tmp/r"}))

	// rm unknown
	require.Error(t, dispatchSubcommand(ctx, "store", "rm", []string{"ghost"}))
	// rm too few args
	require.Error(t, dispatchSubcommand(ctx, "store", "rm", []string{}))
	// rm ok
	require.NoError(t, dispatchSubcommand(ctx, "store", "rm", []string{"r"}))
	require.False(t, ctx.Config.HasRepository("r"))
}

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
