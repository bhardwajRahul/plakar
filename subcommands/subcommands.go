package subcommands

import (
	goflag "flag"
	"fmt"
	"slices"
	"strings"

	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/spf13/pflag"
)

type CommandFlags uint32

const (
	NeedRepositoryKey CommandFlags = 1 << iota
	BeforeRepositoryWithStorage
	BeforeRepositoryOpen
)

type Subcommand interface {
	Parse(ctx *appcontext.AppContext, args []string) error
	Execute(ctx *appcontext.AppContext, repo *repository.Repository) (int, error)
	GetRepositorySecret() []byte
	GetFlags() CommandFlags
	setFlags(CommandFlags)
}

type SubcommandBase struct {
	RepositorySecret []byte
	Flags            CommandFlags
}

func (cmd *SubcommandBase) setFlags(flags CommandFlags) {
	cmd.Flags = flags
}

func (cmd *SubcommandBase) GetFlags() CommandFlags {
	return cmd.Flags
}

func (cmd *SubcommandBase) GetRepositorySecret() []byte {
	return cmd.RepositorySecret
}

// SingleDash rewrites our single-dash options into the double-dash form pflag
// expects, which it would otherwise read as bundles of shorthands.  Length is
// not the criterion: -o, -k and -u are declared with StringVar and friends, so
// they are one-letter long options rather than shorthands.
//
// Like the flag package it stops at the first positional argument, so a path
// starting with a dash is never taken for an option.
func SingleDash(flags *pflag.FlagSet, args []string) ([]string, error) {
	out := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			return append(out, args[i:]...), nil
		}

		if !strings.HasPrefix(arg, "-") {
			return append(out, args[i:]...), nil
		}

		if len(arg) > 1 && arg[1] != '-' {
			name, _, explicit := strings.Cut(arg[1:], "=")
			f := flags.Lookup(name)
			if f == nil {
				// A real shorthand is pflag's to resolve.
				if len(name) == 1 && flags.ShorthandLookup(name) != nil {
					out = append(out, arg)
					continue
				}
				return nil, fmt.Errorf("flag provided but not defined: -%s", name)
			}
			out = append(out, "-"+arg)

			// "-name value" takes the next argument with it.
			if !explicit && f.Value.Type() != "bool" && i+1 < len(args) {
				out = append(out, args[i+1])
				i++
			}
			continue
		}

		out = append(out, arg)
	}

	return out, nil
}

// InstallGoFlags adopts the options kloset's LocateOptions installs into a
// flag.FlagSet.
func InstallGoFlags(flags *pflag.FlagSet, install func(*goflag.FlagSet)) {
	gofs := goflag.NewFlagSet("", goflag.ContinueOnError)
	install(gofs)
	flags.AddGoFlagSet(gofs)
}

type goValue struct {
	goflag.Value
}

func (goValue) Type() string { return "value" }

// GoValue wraps a flag.Value for a pflag set, which additionally wants Type().
func GoValue(v goflag.Value) pflag.Value {
	return goValue{v}
}

// ErrUsage marks an option-parsing error, which exits 2 like the flag package
// did rather than 1.
type ErrUsage struct{ err error }

func (e ErrUsage) Error() string { return e.err.Error() }
func (e ErrUsage) Unwrap() error { return e.err }

type CmdFactory func() Subcommand
type subcmd struct {
	args    []string
	nargs   int
	flags   CommandFlags
	factory CmdFactory
}

var subcommands []subcmd = make([]subcmd, 0)

func Register(factory CmdFactory, flags CommandFlags, args ...string) {
	if len(args) == 0 {
		panic("can't register commands with zero arguments")
	}

	subcommands = append(subcommands, subcmd{
		args:    args,
		nargs:   len(args),
		flags:   flags,
		factory: factory,
	})
}

func Lookup(arguments []string) (Subcommand, []string, []string) {
	nargs := len(arguments)
	for _, subcmd := range subcommands {
		if nargs < subcmd.nargs {
			continue
		}

		if !slices.Equal(subcmd.args, arguments[:subcmd.nargs]) {
			continue
		}

		cmd := subcmd.factory()
		cmd.setFlags(subcmd.flags)
		return cmd, arguments[:subcmd.nargs], arguments[subcmd.nargs:]
	}

	return nil, nil, arguments
}

func List() [][]string {
	var list [][]string
	slices.SortFunc(subcommands, func(a, b subcmd) int {
		var i int
		for {
			n := strings.Compare(a.args[i], b.args[i])
			if n != 0 {
				return n
			}

			i++
			if i == len(a.args) {
				return -1
			}
			if i == len(b.args) {
				return +1
			}
		}
	})
	for _, command := range subcommands {
		list = append(list, command.args)
	}
	return list
}
