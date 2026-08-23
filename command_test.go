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
	var newCommand func(n, of uint64) *Command
	newCommand = func(n, of uint64) *Command {
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
		Subcommands(newCommand(1, cmdDepth))
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
		subcommand.handlerFunc(nil)

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
	getFixture := func(flags ...*Flag) *Command {
		return NewCommand("test", "").Flags(flags...)
	}
	successCases := []*Command{
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
	for i, cmd := range successCases {
		t.Run(fmt.Sprintf("SuccessCase%02d", i+1), func(t *testing.T) {
			if err := cmd.validate(); err != nil {
				t.Errorf("expected nil error, got: %v", err)
			}
		})
	}
	errorCases := []*Command{
		getFixture(
			String(&sink, "one", "", "").Positional().NArgs(0, 0),
			String(&sink, "two", "", "").Positional(),
		),
		getFixture(
			String(&sink, "one", "", "").Positional().NArgs(1, 0),
			String(&sink, "two", "", "").Positional(),
		),
	}
	for i, cmd := range errorCases {
		t.Run(fmt.Sprintf("ErrorCase%02d", i+1), func(t *testing.T) {
			if err := cmd.validate(); err == nil {
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
	)
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
		FlagSet(flagSet)
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
	assertString(t, "a", a.name)
	assertString(t, "b", a.subcommands[0].name)
	assertString(t, "a", a.subcommands[0].parent.name)
	assertString(t, "c", a.subcommands[0].subcommands[0].name)
	assertString(t, "b", a.subcommands[0].subcommands[0].parent.name)
}

func ExampleCommand_FlagGroup() {
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

func ExampleCommand_FlagSet() {
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

func ExampleCommand_Subcommands() {
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

func ExampleCommand_Synopsis() {
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

func ExampleCommand_WithTerminator() {
	var verbose bool

	// create a command that passes arguments to /bin/echo
	cmd := NewCommand("echo_wrapper", "calls /bin/echo").
		Flags(
			Bool(&verbose, "v", false, "Print verbose output"),
		).
		WithTerminator(). // enable the "--" terminator
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
		})

	// run in verbose mode and pass ["Hello", "World!"] to /bin/echo.
	RunWithArgs(cmd, "-v", "--", "Hello,", "World!")
	// Output:
	// + /bin/echo Hello, World!
	// Hello, World!
}

func TestDescribeRoot(t *testing.T) {
	sub := NewCommand("sub", "Sub command usage")
	root := NewCommand("root", "Root command usage").
		Synopsis("Root synopsis").
		Subcommands(sub)

	node, err := root.Describe()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := node.Name, "root"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
	if got, want := node.Usage, "Root command usage"; got != want {
		t.Errorf("Usage = %q, want %q", got, want)
	}
	if got, want := node.Synopsis, "Root synopsis"; got != want {
		t.Errorf("Synopsis = %q, want %q", got, want)
	}
	if node.Parent != nil {
		t.Errorf("Parent = %v, want nil", node.Parent)
	}
	if got, want := len(node.Subcommands), 1; got != want {
		t.Fatalf("len(Subcommands) = %d, want %d", got, want)
	}
	if got, want := node.Subcommands[0].Name, "sub"; got != want {
		t.Errorf("Subcommands[0].Name = %q, want %q", got, want)
	}
	if got, want := node.Subcommands[0].Parent, node; got != want {
		t.Errorf("Subcommands[0].Parent = %v, want %v", got, want)
	}
}

func TestDescribeSubcommand(t *testing.T) {
	foo := NewCommand("foo", "Foo usage")
	bar := NewCommand("bar", "Bar usage")
	NewCommand("root", "Root usage").Subcommands(foo, bar)

	node, err := foo.Describe()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := node.Name, "foo"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
	if node.Parent == nil {
		t.Fatal("Parent = nil, want root")
	}
	if got, want := node.Parent.Name, "root"; got != want {
		t.Errorf("Parent.Name = %q, want %q", got, want)
	}
	if node.Parent.Parent != nil {
		t.Errorf("Parent.Parent = %v, want nil", node.Parent.Parent)
	}
	var names []string
	for _, c := range node.Parent.Subcommands {
		names = append(names, c.Name)
	}
	assertStrings(t, []string{"foo", "bar"}, names)
}

// TestDescribeValidationError asserts that Describe returns the same
// configuration error that Parse would for a misconfigured tree.
func TestDescribeValidationError(t *testing.T) {
	var a, b string
	cmd := NewCommand("test", "").Flags(
		String(&a, "foo", "", ""),
		String(&b, "foo", "", ""), // duplicate name: invalid
	)

	_, describeErr := cmd.Describe()
	if describeErr == nil {
		t.Fatal("expected error from Describe for duplicate flag name, got nil")
	}

	_, parseErr := cmd.Parse(nil)
	if parseErr == nil {
		t.Fatal("expected error from Parse for duplicate flag name, got nil")
	}
	if got, want := describeErr.Error(), parseErr.Error(); got != want {
		t.Errorf("Describe error %q, want the Parse error %q", got, want)
	}
}

// TestDescribeIsPure asserts that Describe does not mutate the command tree
// or the variables flags are bound to: it must reflect neither a Parse that
// ran before it, nor any bookkeeping of its own.
func TestDescribeIsPure(t *testing.T) {
	var s string
	cmd := NewCommand("test", "").Flags(
		String(&s, "name", "default-value", "").NArgs(0, 1),
	)

	if _, err := cmd.Parse([]string{"--name=parsed-value"}); err != nil {
		t.Fatalf("unexpected error from Parse: %v", err)
	}
	if got, want := s, "parsed-value"; got != want {
		t.Fatalf("s = %q, want %q after Parse", got, want)
	}

	node, err := cmd.Describe()
	if err != nil {
		t.Fatalf("unexpected error from Describe: %v", err)
	}

	// The bound variable must still hold the parsed value: Describe must
	// not have written back to it.
	if got, want := s, "parsed-value"; got != want {
		t.Errorf("s = %q, want %q after Describe", got, want)
	}
	// The projected default must still show the value captured at
	// construction, not the live/parsed value.
	df := node.FlagGroups[0].Flags[0]
	if got, want := df.Default, "default-value"; got != want {
		t.Errorf("Default = %q, want %q", got, want)
	}
}

// assertParseError asserts that parsing cmd fails, naming the invalid
// configuration under test in the failure message.
func assertParseError(t *testing.T, cmd *Command, reason string) bool {
	t.Helper()
	if _, err := cmd.Parse(nil); err == nil {
		t.Errorf("expected error for %s, got nil", reason)
		return false
	}
	return true
}

func TestValidateDuplicateFlagName(t *testing.T) {
	var a, b string
	assertParseError(t, NewCommand("test", "").Flags(
		String(&a, "foo", "", ""),
		String(&b, "foo", "", ""),
	), "duplicate flag name")
}

func TestValidateDuplicateShortName(t *testing.T) {
	var a, b string
	assertParseError(t, NewCommand("test", "").Flags(
		String(&a, "x", "", ""),
		String(&b, "x", "", ""),
	), "duplicate short name")
}

func TestValidatePositionalWithSubcommands(t *testing.T) {
	var a string
	cmd := NewCommand("test", "").
		Flags(String(&a, "foo", "", "").Positional()).
		Subcommands(NewCommand("sub", ""))
	assertParseError(t, cmd, "positional flag alongside subcommands")
}

func TestValidatePositionalAfterUnbounded(t *testing.T) {
	var a, b string
	assertParseError(t, NewCommand("test", "").Flags(
		String(&a, "one", "", "").Positional().NArgs(0, 0),
		String(&b, "two", "", "").Positional(),
	), "positional after unbounded positional")
}

func TestValidateInvalidNArgs(t *testing.T) {
	var a string
	assertParseError(t, NewCommand("test", "").Flags(
		String(&a, "foo", "", "").NArgs(2, 1),
	), "invalid NArgs")
}

func TestValidateShortNameTooLong(t *testing.T) {
	var a string
	assertParseError(t, NewCommand("test", "").Flags(
		String(&a, "foo", "", "").ShortName("xx"),
	), "short name too long")
}

// TestValidateErrorsSurfaceAtParse asserts that a misconfigured tree does not
// error, or panic, at construction time -- only once Parse is called.
func TestValidateErrorsSurfaceAtParse(t *testing.T) {
	var a, b string
	cmd := NewCommand("test", "").Flags(
		String(&a, "foo", "", ""),
		String(&b, "foo", "", ""), // duplicate name: invalid
	)
	if cmd == nil {
		t.Fatal("expected non-nil command from construction")
	}
	assertParseError(t, cmd, "a tree only validated at Parse")
}

// TestValidateRunsOverSubcommands asserts that validation walks the whole
// tree: an invalid flag on a subcommand must fail a Parse issued on the
// root.
func TestValidateRunsOverSubcommands(t *testing.T) {
	var a, b string
	sub := NewCommand("sub", "").Flags(
		String(&a, "foo", "", ""),
		String(&b, "foo", "", ""), // duplicate name: invalid
	)
	root := NewCommand("root", "").Subcommands(sub)
	assertParseError(t, root, "an invalid subcommand reached from the root")
}
