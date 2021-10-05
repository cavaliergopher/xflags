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
			Handler(func() int {
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
			args = append(args, fmt.Sprintf("command%02d", j+1), fmt.Sprintf("--x%02d", j+1))
		}

		// invoke the subcommand handler
		handler, err := cmd.Parse(args)
		if err != nil {
			t.Error(err)
			return
		}
		handler()

		// make sure it was the right one
		// TODO: test correct flags were set
		var expect uint64 = 1 << i
		if ranCommands != expect {
			t.Fail()
		}

	}
}
