package xflags

import (
	"fmt"
	"testing"
)

func TestSubcommands(t *testing.T) {
	// ranCommands is a bit mask to identify which subcommand handlers were
	// invoked
	var ranCommands uint64
	var setFlags uint64

	// newCommand is a function to recursively create subcommands
	var newCommand func(n, of int) *CommandInfo
	newCommand = func(n, of int) *CommandInfo {
		c := Command(fmt.Sprintf("command%02d", n)).
			Flags(
				Var(
					ValueFunc(func(s string) error {
						setFlags |= 1 << (n - 1)
						return nil
					}),
					fmt.Sprintf("x%02d", n),
				).
					Boolean().
					MustBuild(),
			).
			Handler(func(args []string) int {
				ranCommands |= 1 << (n - 1)
				return 0
			})
		if n < of {
			c.Subcommands(newCommand(n+1, of))
		}
		return c.MustBuild()
	}

	// call each subcommand
	cmdDepth := 64
	cmd := Command("test").
		Subcommands(newCommand(1, cmdDepth)).
		MustBuild()
	for i := 0; i < cmdDepth; i++ {
		// build args to call subcommand i
		ranCommands = 0
		args := make([]string, 0)
		for j := 0; j < i+1; j++ {
			args = append(
				args,
				fmt.Sprintf("command%02d", j+1), fmt.Sprintf("--x%02d", j+1),
			)
		}

		// invoke the subcommand handler
		handler, err := cmd.Parse(args)
		if err != nil {
			t.Error(err)
			return
		}
		handler(nil)

		// make sure it was the right one
		// TODO: test correct flags were set
		var expect uint64 = 1 << i
		if ranCommands != expect {
			t.Fail()
		}

	}
}

// TestPosFlagOrdering enforces the rule that no positional arguments may be
// specified after another variable length positional argument as this would
// create ambiguity as to which flag a given argument belongs to. Fixed length
// positional arguments do not exhibit this problem.
func TestPosFlagOrdering(t *testing.T) {
	var sink string
	getFixture := func(flags ...*FlagInfo) *CommandBuilder {
		return Command("test").Flags(flags...)
	}

	successCases := []*CommandBuilder{
		getFixture(
			String(&sink, "one").Positional().MustBuild(),
		),
		getFixture(
			String(&sink, "one").Positional().NArgs(1, 1).MustBuild(),
			String(&sink, "two").Positional().MustBuild(),
		),
		getFixture(
			String(&sink, "one").Positional().NArgs(1, 1).MustBuild(),
			String(&sink, "two").Positional().NArgs(2, 2).MustBuild(),
			String(&sink, "three").Positional().NArgs(3, 3).MustBuild(),
			String(&sink, "four").Positional().MustBuild(),
		),
	}
	for i, builder := range successCases {
		t.Run(fmt.Sprintf("SuccessCase%02d", i+1), func(t *testing.T) {
			if _, err := builder.Build(); err != nil {
				t.Errorf("expected nil error, got: %v", err)
			}
		})
	}

	// test for errors
	errorCases := []*CommandBuilder{
		getFixture(
			String(&sink, "one").Positional().MustBuild(),
			String(&sink, "two").Positional().MustBuild(),
		),
		getFixture(
			String(&sink, "one").Positional().NArgs(0, 0).MustBuild(),
			String(&sink, "two").Positional().MustBuild(),
		),
		getFixture(
			String(&sink, "one").Positional().NArgs(1, 0).MustBuild(),
			String(&sink, "two").Positional().MustBuild(),
		),
		getFixture(
			String(&sink, "one").Positional().NArgs(0, 1).MustBuild(),
			String(&sink, "two").Positional().MustBuild(),
		),
	}
	for i, builder := range errorCases {
		t.Run(fmt.Sprintf("ErrorCase%02d", i+1), func(t *testing.T) {
			if _, err := builder.Build(); err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

func TestPositionalFlags(t *testing.T) {
	var foo, bar string
	var baz, qux []string
	cmd := Command("test").Flags(
		String(&foo, "foo").Positional().Required().MustBuild(),
		String(&bar, "bar").Positional().Required().MustBuild(),
		StringSlice(&baz, "baz").Positional().NArgs(2, 2).MustBuild(),
		StringSlice(&qux, "qux").Positional().NArgs(0, 0).MustBuild(),
	).MustBuild()
	_, err := cmd.Parse([]string{"one", "two", "three", "four", "five", "six"})
	if err != nil {
		t.Error(err)
		return
	}
	assertString(t, "one", foo)
	assertString(t, "two", bar)
	assertStringSlice(t, []string{"three", "four"}, baz)
	assertStringSlice(t, []string{"five", "six"}, qux)
}
