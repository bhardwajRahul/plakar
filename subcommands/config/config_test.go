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

// TestDispatchImport covers the import action of dispatchSubcommand: whole-file
// and selected-section imports, the skip/overwrite rules for entries that
// already exist, and the -rclone INI variant.
func TestDispatchImport(t *testing.T) {
	const twoSections = "alpha:\n  location: fs:///a\nbeta:\n  location: fs:///b\n"

	t.Run("all-sections", func(t *testing.T) {
		ctx, _, _ := newCtx(t)
		conf := writeConf(t, "stores.yaml", twoSections)

		require.NoError(t, dispatchSubcommand(ctx, "store", "import", []string{"-config", conf}))
		require.True(t, ctx.Config.HasRepository("alpha"))
		require.True(t, ctx.Config.HasRepository("beta"))
	})

	t.Run("selected-section-renamed", func(t *testing.T) {
		ctx, _, _ := newCtx(t)
		conf := writeConf(t, "stores.yaml", twoSections)

		require.NoError(t, dispatchSubcommand(ctx, "store", "import", []string{"-config", conf, "alpha:gamma"}))
		require.True(t, ctx.Config.HasRepository("gamma"))
		require.False(t, ctx.Config.HasRepository("beta"))
	})

	t.Run("selected-missing-and-empty-name", func(t *testing.T) {
		ctx, _, bufErr := newCtx(t)
		conf := writeConf(t, "import.yml", twoSections)

		// rename alpha, request a missing section, and an empty target
		args := []string{"-config", conf, "alpha:renamed", "ghost", ":bad"}
		require.NoError(t, dispatchSubcommand(ctx, "source", "import", args))

		require.Equal(t, "fs:///a", ctx.Config.Sources["renamed"]["location"])
		require.Contains(t, bufErr.String(), "does not exist in config")
		require.Contains(t, bufErr.String(), "empty section name")
	})

	t.Run("existing-skipped", func(t *testing.T) {
		ctx, _, bufErr := newCtx(t)
		require.NoError(t, dispatchSubcommand(ctx, "source", "add", []string{"alpha", "fs:///old"}))
		conf := writeConf(t, "import.yml", twoSections)

		// without -overwrite alpha is skipped, beta is still added
		require.NoError(t, dispatchSubcommand(ctx, "source", "import", []string{"-config", conf}))
		require.Contains(t, bufErr.String(), "already exists, skipping")
		require.Equal(t, "fs:///old", ctx.Config.Sources["alpha"]["location"])
		require.Equal(t, "fs:///b", ctx.Config.Sources["beta"]["location"])
	})

	t.Run("selected-existing-skipped", func(t *testing.T) {
		ctx, _, bufErr := newCtx(t)
		require.NoError(t, dispatchSubcommand(ctx, "source", "add", []string{"alpha", "fs:///old"}))
		conf := writeConf(t, "import.yml", "alpha:\n  location: fs:///new\n")

		require.NoError(t, dispatchSubcommand(ctx, "source", "import", []string{"-config", conf, "alpha"}))
		require.Contains(t, bufErr.String(), "already exists, skipping")
		require.Equal(t, "fs:///old", ctx.Config.Sources["alpha"]["location"])
	})

	t.Run("overwrite", func(t *testing.T) {
		ctx, _, _ := newCtx(t)
		require.NoError(t, dispatchSubcommand(ctx, "source", "add", []string{"alpha", "fs:///old"}))
		conf := writeConf(t, "import.yml", "alpha:\n  location: fs:///new\n")

		require.NoError(t, dispatchSubcommand(ctx, "source", "import", []string{"-overwrite", "-config", conf}))
		require.Equal(t, "fs:///new", ctx.Config.Sources["alpha"]["location"])
	})

	t.Run("missing-file", func(t *testing.T) {
		ctx, _, _ := newCtx(t)

		err := dispatchSubcommand(ctx, "store", "import", []string{"-config", "/nonexistent/x.yaml"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to open file")
	})

	t.Run("no-valid-sections", func(t *testing.T) {
		ctx, _, _ := newCtx(t)
		// a scalar-only top level yields no sections once empties are stripped
		conf := writeConf(t, "empty.yml", "location: fs:///x\n")

		require.Error(t, dispatchSubcommand(ctx, "source", "import", []string{"-config", conf}))
	})

	t.Run("rclone", func(t *testing.T) {
		ctx, _, _ := newCtx(t)
		// with -rclone the importer synthesizes an "rclone://" location and
		// prefixes each key.
		conf := writeConf(t, "rclone.conf", "[remote1]\ntype = s3\nprovider = AWS\n")

		require.NoError(t, dispatchSubcommand(ctx, "store", "import", []string{"-rclone", "-config", conf}))
		require.True(t, ctx.Config.HasRepository("remote1"))
		require.Equal(t, "rclone://", ctx.Config.Repositories["remote1"]["location"])
		require.Equal(t, "s3", ctx.Config.Repositories["remote1"]["rclone_type"])
	})

	t.Run("rclone-empty", func(t *testing.T) {
		ctx, _, _ := newCtx(t)
		// an empty INI synthesizes nothing, so dispatch finds no valid entries
		conf := writeConf(t, "empty.conf", "\n")

		require.Error(t, dispatchSubcommand(ctx, "store", "import", []string{"-rclone", "-config", conf}))
	})
}
