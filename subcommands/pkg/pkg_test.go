package pkg

import (
	"bytes"
	"os"
	"testing"

	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/subcommands"
	"github.com/stretchr/testify/require"
)

func init() {
	os.Setenv("TZ", "UTC")
}

func newCtx(t *testing.T) *appcontext.AppContext {
	t.Helper()
	ctx := appcontext.NewAppContext()
	ctx.Stdout = bytes.NewBuffer(nil)
	ctx.Stderr = bytes.NewBuffer(nil)
	ctx.CWD = t.TempDir()
	return ctx
}

func TestPkgRegisteredFactories(t *testing.T) {
	cases := []struct {
		args []string
		typ  interface{}
	}{
		{[]string{"pkg", "add"}, &PkgAdd{}},
		{[]string{"pkg", "rm"}, &PkgRm{}},
		{[]string{"pkg", "create"}, &PkgCreate{}},
		{[]string{"pkg", "build"}, &PkgBuild{}},
		{[]string{"pkg", "list"}, &PkgList{}},
		{[]string{"pkg", "show"}, &PkgList{}},
	}
	for _, c := range cases {
		cmd, _, _ := subcommands.Lookup(c.args)
		require.NotNil(t, cmd, "args=%v", c.args)
		require.IsType(t, c.typ, cmd)
	}
}

func TestPkgListParse(t *testing.T) {
	ctx := newCtx(t)
	cmd := &PkgList{}
	require.NoError(t, cmd.Parse(ctx, []string{}))

	cmd = &PkgList{}
	require.NoError(t, cmd.Parse(ctx, []string{"-long", "-available"}))
	require.True(t, cmd.LongName)
	require.True(t, cmd.ListAll)

	// extra positional argument is rejected
	require.Error(t, (&PkgList{}).Parse(ctx, []string{"extra"}))
}

func TestPkgCreateParseErrors(t *testing.T) {
	ctx := newCtx(t)

	// wrong arg count
	require.Error(t, (&PkgCreate{}).Parse(ctx, []string{"manifest.yaml"}))

	// bad version string
	require.Error(t, (&PkgCreate{}).Parse(ctx, []string{"manifest.yaml", "not-a-semver"}))

	// manifest filename must be exactly "manifest.yaml"
	err := (&PkgCreate{}).Parse(ctx, []string{"wrong-name.yaml", "v1.2.3"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "manifest.yaml")

	// correctly-named but missing manifest file: open fails
	err = (&PkgCreate{}).Parse(ctx, []string{"manifest.yaml", "v1.2.3"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "can't open")
}

func TestPkgRmParse(t *testing.T) {
	ctx := newCtx(t)
	// rm accepts a list of plugin names (empty is allowed at parse time).
	cmd := &PkgRm{}
	require.NoError(t, cmd.Parse(ctx, []string{"plugin-a", "plugin-b"}))
	require.Equal(t, []string{"plugin-a", "plugin-b"}, cmd.Args)
}

func TestPkgAddParse(t *testing.T) {
	ctx := newCtx(t)
	// add with no package name should error.
	require.Error(t, (&PkgAdd{}).Parse(ctx, []string{}))
}
