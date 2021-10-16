package xflags

import (
	"flag"
	"fmt"
	"testing"
)

func TestSubcommands(t *testing.T) {
	// ranCommands is a bit mask to identify which subcommand handlers were
	// invoked
	var ranCommands uint64
	var setFlags uint64

	// newCommand is a function to recursively create subcommands
	var newCommand func(n, of uint64) *CommandInfo
	newCommand = func(n, of uint64) *CommandInfo {
		c := Command(fmt.Sprintf("command%02d", n)).
			Flags(
				BitFieldVar(
					&setFlags,
					uint64(1)<<(n-1),
					fmt.Sprintf("x%02d", n),
					false,
					"",
				).Must(),
			).
			Handler(func(args []string) int {
				ranCommands |= 1 << (n - 1)
				return 0
			})
		if n < of {
			c.Subcommands(newCommand(n+1, of))
		}
		return c.Must()
	}

	// call each subcommand
	cmdDepth := uint64(64)
	cmd := Command("test").
		Subcommands(newCommand(1, cmdDepth)).
		Must()
	for i := uint64(0); i < cmdDepth; i++ {
		// build args to call subcommand i
		ranCommands = 0
		args := make([]string, 0)
		for j := uint64(0); j < i+1; j++ {
			args = append(
				args,
				fmt.Sprintf("command%02d", j+1), fmt.Sprintf("--x%02d", j+1),
			)
		}

		// invoke the subcommand handler
		subcommand, err := cmd.Parse(args)
		if err != nil {
			t.Error(err)
			return
		}
		subcommand.Handler(nil)

		// check which commands run and flags were set
		assertUint64(t, 1<<i, ranCommands)
		expectFlags := uint64(0)
		for j := uint64(0); j < i+1; j++ {
			expectFlags |= 1 << j
		}
		assertUint64(t, expectFlags, setFlags)
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
			StringVar(&sink, "one", "", "").Positional().Must(),
		),
		getFixture(
			StringVar(&sink, "one", "", "").Positional().NArgs(1, 1).Must(),
			StringVar(&sink, "two", "", "").Positional().Must(),
		),
		getFixture(
			StringVar(&sink, "one", "", "").Positional().NArgs(1, 1).Must(),
			StringVar(&sink, "two", "", "").Positional().NArgs(2, 2).Must(),
			StringVar(&sink, "three", "", "").Positional().NArgs(3, 3).Must(),
			StringVar(&sink, "four", "", "").Positional().Must(),
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
			StringVar(&sink, "one", "", "").Positional().Must(),
			StringVar(&sink, "two", "", "").Positional().Must(),
		),
		getFixture(
			StringVar(&sink, "one", "", "").Positional().NArgs(0, 0).Must(),
			StringVar(&sink, "two", "", "").Positional().Must(),
		),
		getFixture(
			StringVar(&sink, "one", "", "").Positional().NArgs(1, 0).Must(),
			StringVar(&sink, "two", "", "").Positional().Must(),
		),
		getFixture(
			StringVar(&sink, "one", "", "").Positional().NArgs(0, 1).Must(),
			StringVar(&sink, "two", "", "").Positional().Must(),
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
		StringVar(&foo, "foo", "", "").Positional().Required().Must(),
		StringVar(&bar, "bar", "", "").Positional().Required().Must(),
		StringSliceVar(&baz, "baz", nil, "").Positional().NArgs(2, 2).Must(),
		StringSliceVar(&qux, "qux", nil, "").Positional().NArgs(0, 0).Must(),
	).Must()
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

func TestFlagSet(t *testing.T) {
	var foo, bar string
	var baz, qux bool
	flagSet := flag.NewFlagSet("native", flag.ContinueOnError)
	flagSet.StringVar(&foo, "foo", "", "")
	flagSet.BoolVar(&baz, "baz", false, "")
	c := Command("test").
		Flags(
			StringVar(&bar, "bar", "", "").Must(),
			BoolVar(&qux, "qux", false, "").Must(),
		).
		FlagSet(flagSet).
		Must()
	_, err := c.Parse([]string{"--foo", "foo", "--bar", "bar", "--baz", "--qux"})
	if err != nil {
		t.Fatal(err)
	}
	assertString(t, "foo", foo)
	assertString(t, "bar", bar)
	assertBool(t, true, baz)
	assertBool(t, true, qux)
}
