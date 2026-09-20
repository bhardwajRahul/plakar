package config

import (
	"testing"

	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/stretchr/testify/require"
)

func configure(ctx *appcontext.AppContext, cmd string, args []string) error {
	subcmd := "show"
	if len(args) > 0 {
		subcmd = args[0]
		args = args[1:]
	}

	err := dispatchSubcommand(ctx, cmd, subcmd, args)
	if err != nil {
		return err
	}
	return nil
}

func TestValidAliasName(t *testing.T) {
	suite := map[string]bool{
		"":         false,
		"foo":      true,
		"Fo0_B47":  true,
		"b@r":      false,
		"/foo/":    false,
		"-bar":     false,
		"b-ar":     true,
		"s3://foo": false,
	}

	for name, expect := range suite {
		got := validAliasName(name)
		if got != expect {
			t.Errorf("%s: got %v but expected %v", name, got, expect)
		}
	}
}

func TestConfigEmpty(t *testing.T) {
	ctx, bufOut, bufErr := newCtx(t)
	repo := &repository.Repository{}
	args := []string{}

	subcommand := &ConfigStoreCmd{}
	err := subcommand.Parse(ctx, args)
	require.Error(t, err, "no action specified")

	subcommand = &ConfigStoreCmd{}
	args = []string{"show"}
	err = subcommand.Parse(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, subcommand)

	status, err := subcommand.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)

	output := bufOut.String()
	expectedOutput := ""
	require.Equal(t, expectedOutput, output)

	bufOut.Reset()
	bufErr.Reset()

	args = []string{"add", "my-remote", "s3://foobar"}
	subcommandr := &ConfigSourceCmd{}
	err = subcommandr.Parse(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, subcommandr)

	status, err = subcommandr.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)

	args = []string{"add", "my-repo", "fs:/tmp/foobar"}
	subcommandk := &ConfigStoreCmd{}
	err = subcommandk.Parse(ctx, args)
	require.NoError(t, err)
	require.NotNil(t, subcommandk)

	status, err = subcommandk.Execute(ctx, repo)
	require.NoError(t, err)
	require.Equal(t, 0, status)

	output = bufOut.String()
	expectedOutput = ``
	require.Equal(t, expectedOutput, output)
}

func TestCmdRemote(t *testing.T) {
	ctx, _, _ := newCtx(t)

	args := []string{}
	err := configure(ctx, "source", args)
	require.NoError(t, err)

	args = []string{"unknown"}
	err = configure(ctx, "source", args)
	require.EqualError(t, err, "usage: plakar source [add|check|import|ping|rm|set|show|unset]")

	args = []string{"add", "my-remote", "invalid://my-remote"}
	err = configure(ctx, "source", args)
	require.NoError(t, err)

	args = []string{"add", "my-remote2", "invalid://my-remote2"}
	err = configure(ctx, "source", args)
	require.NoError(t, err)

	args = []string{"set", "my-remote", "option=value"}
	err = configure(ctx, "source", args)
	require.NoError(t, err)

	args = []string{"set", "my-remote2", "option2=value2"}
	err = configure(ctx, "source", args)
	require.NoError(t, err)

	args = []string{"unset", "my-remote2", "option2"}
	err = configure(ctx, "source", args)
	require.NoError(t, err)

	args = []string{"check", "my-remote"}
	err = configure(ctx, "source", args)
	require.EqualError(t, err, "unsupported importer protocol")
}

func TestCmdRepository(t *testing.T) {
	ctx, _, _ := newCtx(t)

	args := []string{"unknown"}
	err := configure(ctx, "store", args)
	require.EqualError(t, err, "usage: plakar store [add|check|import|ping|rm|set|show|unset]")

	args = []string{"add", "my-repo", "fs:/tmp/my-repo"}
	err = configure(ctx, "store", args)
	require.NoError(t, err)

	args = []string{"set", "my-repo", "location=invalid://place"}
	err = configure(ctx, "store", args)
	require.NoError(t, err)

	args = []string{"add", "my-repo2", "invalid://place2"}
	err = configure(ctx, "store", args)
	require.NoError(t, err)

	args = []string{"set", "my-repo", "option=value"}
	err = configure(ctx, "store", args)
	require.NoError(t, err)

	args = []string{"set", "my-repo2", "option2=value2"}
	err = configure(ctx, "store", args)
	require.NoError(t, err)

	args = []string{"unset", "my-repo2", "option2"}
	err = configure(ctx, "store", args)
	require.NoError(t, err)

	args = []string{"check", "my-repo2"}
	err = configure(ctx, "store", args)
	require.EqualError(t, err, "backend 'invalid' does not exist")
}

// TestDispatchCheckPing covers the check and ping actions of dispatchSubcommand
// for every entity kind. Both verbs resolve the named entry and open its
// backend, so they share one table.
func TestDispatchCheckPing(t *testing.T) {
	entities := []struct{ kind, name string }{
		{"store", "r"},
		{"source", "s"},
		{"destination", "d"},
	}

	for _, verb := range []string{"check", "ping"} {
		for _, e := range entities {
			t.Run(verb+"/"+e.kind+"/ok", func(t *testing.T) {
				ctx, _, _ := newCtx(t)
				// the fs backend opens cleanly even when uninitialized.
				require.NoError(t, dispatchSubcommand(ctx, e.kind, "add", []string{e.name, fsLoc(t)}))
				require.NoError(t, dispatchSubcommand(ctx, e.kind, verb, []string{e.name}))
			})

			t.Run(verb+"/"+e.kind+"/unknown-scheme", func(t *testing.T) {
				ctx, _, _ := newCtx(t)
				require.NoError(t, dispatchSubcommand(ctx, e.kind, "add", []string{e.name, "no-such-scheme://nowhere"}))
				require.Error(t, dispatchSubcommand(ctx, e.kind, verb, []string{e.name}))
			})

			t.Run(verb+"/"+e.kind+"/unknown-name", func(t *testing.T) {
				ctx, _, _ := newCtx(t)
				require.Error(t, dispatchSubcommand(ctx, e.kind, verb, []string{"ghost"}))
			})

			// wrong arg count is checked before the entity kind is looked at.
			t.Run(verb+"/"+e.kind+"/no-args", func(t *testing.T) {
				ctx, _, _ := newCtx(t)
				require.Error(t, dispatchSubcommand(ctx, e.kind, verb, []string{}))
			})
		}
	}
}
