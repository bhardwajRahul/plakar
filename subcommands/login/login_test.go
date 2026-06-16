package login

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
	return ctx
}

func TestLoginRegisteredFactories(t *testing.T) {
	cases := []struct {
		args []string
		typ  interface{}
	}{
		{[]string{"login"}, &Login{}},
		{[]string{"logout"}, &Logout{}},
		{[]string{"token", "create"}, &TokenCreate{}},
	}
	for _, c := range cases {
		cmd, _, _ := subcommands.Lookup(c.args)
		require.NotNil(t, cmd, "args=%v", c.args)
		require.IsType(t, c.typ, cmd)
	}
}

func TestLoginParseTooManyArgs(t *testing.T) {
	require.Error(t, (&Login{}).Parse(newCtx(t), []string{"extra"}))
}

func TestLoginParseDefaultsToGithub(t *testing.T) {
	ctx := newCtx(t)
	cmd := &Login{}
	require.NoError(t, cmd.Parse(ctx, []string{}))
	require.True(t, cmd.Github, "with no method, login defaults to GitHub")
}

func TestLoginParseGithub(t *testing.T) {
	ctx := newCtx(t)
	cmd := &Login{}
	require.NoError(t, cmd.Parse(ctx, []string{"-github", "-no-spawn"}))
	require.True(t, cmd.Github)
	require.True(t, cmd.NoSpawn)
}

func TestLoginParseEmailValidated(t *testing.T) {
	ctx := newCtx(t)
	cmd := &Login{}
	require.NoError(t, cmd.Parse(ctx, []string{"-email", "user@example.com"}))
	require.Equal(t, "user@example.com", cmd.Email)
}

func TestLoginParseInvalidEmail(t *testing.T) {
	ctx := newCtx(t)
	err := (&Login{}).Parse(ctx, []string{"-email", "not-an-email"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid email")
}

func TestLoginParseStatusMustBeAlone(t *testing.T) {
	ctx := newCtx(t)
	err := (&Login{}).Parse(ctx, []string{"-status", "-github"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be used alone")
}

func TestLoginParseStatusAlone(t *testing.T) {
	ctx := newCtx(t)
	cmd := &Login{}
	require.NoError(t, cmd.Parse(ctx, []string{"-status"}))
	require.True(t, cmd.Status)
}

func TestLoginParseGithubEmailConflict(t *testing.T) {
	ctx := newCtx(t)
	err := (&Login{}).Parse(ctx, []string{"-github", "-email", "user@example.com"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be used with")
}

func TestLoginParseEmailEnvConflict(t *testing.T) {
	ctx := newCtx(t)
	err := (&Login{}).Parse(ctx, []string{"-email", "user@example.com", "-env"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be used with -env")
}

func TestLoginParseNoSpawnRequiresGithub(t *testing.T) {
	ctx := newCtx(t)
	// -no-spawn with -email (not github) is rejected.
	err := (&Login{}).Parse(ctx, []string{"-email", "user@example.com", "-no-spawn"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no-spawn")
}

func TestLogoutParse(t *testing.T) {
	ctx := newCtx(t)
	require.NoError(t, (&Logout{}).Parse(ctx, []string{}))
	require.Error(t, (&Logout{}).Parse(ctx, []string{"extra"}))
}

func TestTokenCreateParse(t *testing.T) {
	ctx := newCtx(t)
	require.NoError(t, (&TokenCreate{}).Parse(ctx, []string{}))
	require.Error(t, (&TokenCreate{}).Parse(ctx, []string{"extra"}))
}
