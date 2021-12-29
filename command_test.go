package xflags

import (
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestSubcommands(t *testing.T) {
	// ranCommands is a bit mask to identify which subcommand handlers were
	// invoked
	var ranCommands uint64
	var setFlags uint64

	// newCommand is a function to recursively create subcommands
	var newCommand func(n, of uint64) Commander
	newCommand = func(n, of uint64) Commander {
		c := NewCommand(fmt.Sprintf("command%02d", n), "").
			Flags(
				BitField(
					&setFlags,
					uint64(1)<<(n-1),
					fmt.Sprintf("x%02d", n),
					false,
					"",
				),
			).
			HandleFunc(func(args []string) int {
				ranCommands |= 1 << (n - 1)
				return 0
			})
		if n < of {
			c.Subcommands(newCommand(n+1, of))
		}
		return c
	}

	// call each subcommand
	cmdDepth := uint64(64)
	cmd := NewCommand("test", "").
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
		subcommand.HandlerFunc(nil)

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
	getFixture := func(flags ...Flagger) *CommandBuilder {
		return NewCommand("test", "").Flags(flags...)
	}
	successCases := []*CommandBuilder{
		getFixture(
			String(&sink, "one", "", "").Positional(),
		),
		getFixture(
			String(&sink, "one", "", "").Positional(),
			String(&sink, "two", "", "").Positional(),
		),
		getFixture(
			String(&sink, "one", "", "").Positional().NArgs(0, 1),
			String(&sink, "two", "", "").Positional(),
		),
		getFixture(
			String(&sink, "one", "", "").Positional().NArgs(1, 1),
			String(&sink, "two", "", "").Positional(),
		),
		getFixture(
			String(&sink, "one", "", "").Positional().NArgs(1, 1),
			String(&sink, "two", "", "").Positional().NArgs(2, 2),
			String(&sink, "three", "", "").Positional().NArgs(3, 3),
			String(&sink, "four", "", "").Positional(),
		),
	}
	for i, builder := range successCases {
		t.Run(fmt.Sprintf("SuccessCase%02d", i+1), func(t *testing.T) {
			if _, err := builder.Command(); err != nil {
				t.Errorf("expected nil error, got: %v", err)
			}
		})
	}
	errorCases := []*CommandBuilder{
		getFixture(
			String(&sink, "one", "", "").Positional().NArgs(0, 0),
			String(&sink, "two", "", "").Positional(),
		),
		getFixture(
			String(&sink, "one", "", "").Positional().NArgs(1, 0),
			String(&sink, "two", "", "").Positional(),
		),
	}
	for i, builder := range errorCases {
		t.Run(fmt.Sprintf("ErrorCase%02d", i+1), func(t *testing.T) {
			if _, err := builder.Command(); err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

func TestPositionalFlags(t *testing.T) {
	var foo, bar string
	var baz, qux []string
	cmd := NewCommand("test", "").Flags(
		String(&foo, "foo", "", "").Positional().Required(),
		String(&bar, "bar", "", "").Positional().Required(),
		Strings(&baz, "baz", nil, "").Positional().NArgs(2, 2),
		Strings(&qux, "qux", nil, "").Positional().NArgs(0, 0),
	).Must()
	_, err := cmd.Parse([]string{"one", "two", "three", "four", "five", "six"})
	if err != nil {
		t.Error(err)
		return
	}
	assertString(t, "one", foo)
	assertString(t, "two", bar)
	assertStrings(t, []string{"three", "four"}, baz)
	assertStrings(t, []string{"five", "six"}, qux)
}

func TestFlagSet(t *testing.T) {
	var foo, bar string
	var baz, qux bool
	flagSet := flag.NewFlagSet("native", flag.ContinueOnError)
	flagSet.StringVar(&foo, "foo", "", "")
	flagSet.BoolVar(&baz, "baz", false, "")
	c := NewCommand("test", "").
		Flags(
			String(&bar, "bar", "", ""),
			Bool(&qux, "qux", false, ""),
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

func TestCommandLineage(t *testing.T) {
	a, b, c := NewCommand("a", ""), NewCommand("b", ""), NewCommand("c", "")
	a.Subcommands(b)
	b.Subcommands(c)
	cmd := a.Must()
	assertString(t, "a", cmd.Name)
	assertString(t, "b", cmd.Subcommands[0].Name)
	assertString(t, "a", cmd.Subcommands[0].Parent.Name)
	assertString(t, "c", cmd.Subcommands[0].Subcommands[0].Name)
	assertString(t, "b", cmd.Subcommands[0].Subcommands[0].Parent.Name)
}

func ExampleCommandBuilder_FlagGroup() {
	var n int
	var rightToLeft bool
	var endcoding string

	cmd := NewCommand("helloworld", "").
		// n flag defines how many times to print "Hello, World!".
		Flags(Int(&n, "n", 1, "Print n times")).

		// Create a flag group for language-related flags.
		FlagGroup(
			"language",
			"Language options",
			String(&endcoding, "encoding", "utf-8", "Text encoding"),
			Bool(&rightToLeft, "rtl", false, "Print right-to-left"),
		)

	// Print the help page
	RunWithArgs(cmd, "--help")
	// Output:
	// Usage: helloworld [OPTIONS]
	//
	// Options:
	//   -n   Print n times
	//
	// Language options:
	//    --encoding  Text encoding
	//    --rtl       Print right-to-left
}

func ExampleCommandBuilder_FlagSet() {
	// create a Go-native flag set
	flagSet := flag.NewFlagSet("native", flag.ExitOnError)
	message := flagSet.String("m", "Hello, World!", "Message to print")

	// import the flagset into an xflags command
	cmd := NewCommand("helloworld", "").
		FlagSet(flagSet).
		HandleFunc(func(args []string) (exitCode int) {
			fmt.Println(*message)
			return
		})

	// Print the help page
	fmt.Println("+ helloworld --help")
	RunWithArgs(cmd, "--help")

	// Run the command
	fmt.Println()
	fmt.Println("+ helloworld")
	RunWithArgs(cmd)
	// Output:
	// + helloworld --help
	// Usage: helloworld [OPTIONS]
	//
	// Options:
	//   -m   Message to print
	//
	// + helloworld
	// Hello, World!
}

func ExampleCommandBuilder_Subcommands() {
	var n int

	// configure a "create" subcommand
	create := NewCommand("create", "Make new widgets").
		HandleFunc(func(args []string) (exitCode int) {
			fmt.Printf("Created %d widget(s)\n", n)
			return
		})

	// configure a "destroy" subcommand
	destroy := NewCommand("destroy", "Destroy widgets").
		HandleFunc(func(args []string) (exitCode int) {
			fmt.Printf("Destroyed %d widget(s)\n", n)
			return
		})

	// configure the main command with two subcommands and a global "n" flag.
	cmd := NewCommand("widgets", "").
		Flags(Int(&n, "n", 1, "Affect n widgets")).
		Subcommands(create, destroy)

	// Print the help page
	fmt.Println("+ widgets --help")
	RunWithArgs(cmd, "--help")

	// Invoke the "create" subcommand
	fmt.Println()
	fmt.Println("+ widgets create -n=3")
	RunWithArgs(cmd, "create", "-n=3")
	// Output:
	// + widgets --help
	// Usage: widgets [OPTIONS] COMMAND
	//
	// Options:
	//   -n   Affect n widgets
	//
	// Commands:
	//   create   Make new widgets
	//   destroy  Destroy widgets
	//
	// + widgets create -n=3
	// Created 3 widget(s)
}

func ExampleCommandBuilder_Synopsis() {
	var n int
	cmd := NewCommand("helloworld", "Say \"Hello, World!\"").
		// Configure a synopsis to print detailed usage information on the help
		// page.
		Synopsis(
			"This utility prints \"Hello, World!\" to the standard output.\n" +
				"Print more than once with -n.",
		).
		Flags(Int(&n, "n", 1, "Print n times"))

	// Print the help page
	RunWithArgs(cmd, "--help")
	// Output:
	// Usage: helloworld [OPTIONS]
	//
	// Say "Hello, World!"
	//
	// Options:
	//   -n   Print n times
	//
	// This utility prints "Hello, World!" to the standard output.
	// Print more than once with -n.
}

func ExampleCommandBuilder_WithTerminator() {
	var verbose bool

	// create a command that passes arguments to /bin/echo
	cmd := NewCommand("echo_wrapper", "calls /bin/echo").
		Flags(
			Bool(&verbose, "v", false, "Print verbose output"),
		).
		HandleFunc(func(args []string) (exitCode int) {
			// read verbose argument which was parsed by xflags
			if verbose {
				fmt.Printf("+ /bin/echo %s\n", strings.Join(args, " "))
			}

			// pass unparsed arguments after the "--" terminator to /bin/echo
			output, err := exec.Command("/bin/echo", args...).Output()
			if err != nil {
				fmt.Println(err)
				return 1
			}
			fmt.Println(string(output))
			return
		}).
		WithTerminator() // enable the "--" terminator

	// run in verbose mode and pass ["Hello", "World!"] to /bin/echo.
	RunWithArgs(cmd, "-v", "--", "Hello,", "World!")
	// Output:
	// + /bin/echo Hello, World!
	// Hello, World!
}
