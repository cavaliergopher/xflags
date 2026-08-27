package xflags

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
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
			HandleFunc(func(ctx context.Context, inv *Invocation) error {
				ranCommands |= 1 << (n - 1)
				return nil
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
		inv, err := cmd.Parse(args)
		if err != nil {
			t.Error(err)
			return
		}
		if err := inv.Cmd.handlerFunc(context.Background(), inv); err != nil {
			t.Error(err)
			return
		}

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
	RunWithArgs(context.Background(), cmd, "--help")
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
		HandleFunc(func(ctx context.Context, inv *Invocation) error {
			fmt.Println(*message)
			return nil
		})

	ctx := context.Background()

	// Print the help page
	fmt.Println("+ helloworld --help")
	RunWithArgs(ctx, cmd, "--help")

	// Run the command
	fmt.Println()
	fmt.Println("+ helloworld")
	RunWithArgs(ctx, cmd)
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
		HandleFunc(func(ctx context.Context, inv *Invocation) error {
			fmt.Printf("Created %d widget(s)\n", n)
			return nil
		})

	// configure a "destroy" subcommand
	destroy := NewCommand("destroy", "Destroy widgets").
		HandleFunc(func(ctx context.Context, inv *Invocation) error {
			fmt.Printf("Destroyed %d widget(s)\n", n)
			return nil
		})

	// configure the main command with two subcommands and a global "n" flag.
	cmd := NewCommand("widgets", "").
		Flags(Int(&n, "n", 1, "Affect n widgets")).
		Subcommands(create, destroy)

	ctx := context.Background()

	// Print the help page
	fmt.Println("+ widgets --help")
	RunWithArgs(ctx, cmd, "--help")

	// Invoke the "create" subcommand
	fmt.Println()
	fmt.Println("+ widgets create -n=3")
	RunWithArgs(ctx, cmd, "create", "-n=3")
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

func ExampleCommand_Description() {
	var n int
	cmd := NewCommand("helloworld", "Say \"Hello, World!\"").
		// Configure a description to print detailed information on the help
		// page.
		Description(
			"This utility prints \"Hello, World!\" to the standard output.\n" +
				"Print more than once with -n.",
		).
		Flags(Int(&n, "n", 1, "Print n times"))

	// Print the help page
	RunWithArgs(context.Background(), cmd, "--help")
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

func ExampleCommand_ForwardArgs() {
	var verbose bool

	// create a command that forwards arguments to another program
	cmd := NewCommand("echo_wrapper", "wraps the echo command").
		Flags(
			Bool(&verbose, "v", false, "Print verbose output"),
		).
		ForwardArgs(). // enable the "--" terminator
		HandleFunc(func(ctx context.Context, inv *Invocation) error {
			// read verbose argument which was parsed by xflags
			if verbose {
				fmt.Printf("+ echo %s\n", strings.Join(inv.Forwarded, " "))
			}

			// inv.Forwarded holds everything after the "--" terminator,
			// untouched by the parser, ready to hand to the wrapped
			// program
			fmt.Println(strings.Join(inv.Forwarded, " "))
			return nil
		})

	// run in verbose mode and pass ["Hello,", "World!"] through the terminator
	RunWithArgs(context.Background(), cmd, "-v", "--", "Hello,", "World!")
	// Output:
	// + echo Hello, World!
	// Hello, World!
}

func TestDescribeRoot(t *testing.T) {
	sub := NewCommand("sub", "Sub command summary")
	root := NewCommand("root", "Root command summary").
		Description("Root description").
		Subcommands(sub)

	node, err := root.Describe()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := node.Name, "root"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
	if got, want := node.Summary, "Root command summary"; got != want {
		t.Errorf("Summary = %q, want %q", got, want)
	}
	if got, want := node.Description, "Root description"; got != want {
		t.Errorf("Description = %q, want %q", got, want)
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
	foo := NewCommand("foo", "Foo summary")
	bar := NewCommand("bar", "Bar summary")
	NewCommand("root", "Root summary").Subcommands(foo, bar)

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

// TestValidateAncestorShadowing asserts the path-scoped naming rule: a
// command may not redeclare a name an ancestor already claimed, by either
// spelling, however far up the path the ancestor is. See
// docs/adr/path-scoped-flag-names.md.
func TestValidateAncestorShadowing(t *testing.T) {
	for _, tt := range []struct {
		name string
		cmd  *Command
		want string
	}{
		{
			name: "LongName",
			cmd: NewCommand("root", "").
				Flags(Bool(new(bool), "force", false, "")).
				Subcommands(NewCommand("sub", "").Flags(
					Bool(new(bool), "force", false, ""),
				)),
			want: `sub: flag already declared by ancestor "root": --force`,
		},
		{
			name: "ShortName",
			cmd: NewCommand("root", "").
				Flags(String(new(string), "file", "", "").ShortName("f")).
				Subcommands(NewCommand("sub", "").Flags(
					String(new(string), "output", "", "").ShortName("f"),
				)),
			want: `sub: flag already declared by ancestor "root": -f`,
		},
		{
			name: "GrandparentClaim",
			cmd: NewCommand("root", "").
				Flags(Bool(new(bool), "force", false, "")).
				Subcommands(NewCommand("mid", "").Subcommands(
					NewCommand("leaf", "").Flags(
						Bool(new(bool), "force", false, ""),
					),
				)),
			want: `leaf: flag already declared by ancestor "root": --force`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.cmd.Parse(nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got, want := errorOrString(err), tt.want; got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
		})
	}
}

// TestSiblingFlagReuse asserts the freedom the path-scoped rule buys:
// commands in different subtrees may declare the same names, and each
// spelling binds the variable of whichever sibling was invoked.
func TestSiblingFlagReuse(t *testing.T) {
	var deleteForce, pushForce bool
	app := NewCommand("app", "").Subcommands(
		NewCommand("delete", "").Flags(
			Bool(&deleteForce, "force", false, "").ShortName("f"),
		),
		NewCommand("push", "").Flags(
			Bool(&pushForce, "force", false, "").ShortName("f"),
		),
	)

	inv, err := app.Parse([]string{"delete", "--force"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := inv.Cmd.name, "delete"; got != want {
		t.Errorf("Cmd = %q, want %q", got, want)
	}
	assertBool(t, true, deleteForce)
	assertBool(t, false, pushForce)

	inv, err = app.Parse([]string{"push", "-f"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := inv.Cmd.name, "push"; got != want {
		t.Errorf("Cmd = %q, want %q", got, want)
	}
	assertBool(t, true, pushForce)
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

// TestArgumentErrorNamesTheFlag asserts that every parse error a user can
// provoke names the flag it is about. The flag is carried on the error either
// way, but a human reading stderr only sees Message.
func TestArgumentErrorNamesTheFlag(t *testing.T) {
	// Each case builds its own command, since checkNArgs reports the first
	// unsatisfied flag and a shared one would let cases mask each other.
	for _, tt := range []struct {
		name string
		flag *Flag
		args []string
		want string
	}{
		{
			"MissingRequired",
			String(new(string), "req", "", "").Required(),
			nil,
			"missing required argument: --req",
		},
		{
			"TooFewExactCount",
			Strings(&[]string{}, "pair", nil, "").NArgs(2, 2),
			[]string{"--pair", "a"},
			"expected 2 arguments, got 1: --pair",
		},
		{
			"TooFewAtLeast",
			Strings(&[]string{}, "least", nil, "").NArgs(2, 0),
			[]string{"--least", "a"},
			"expected at least 2 arguments, got 1: --least",
		},
		{
			"TooManyOccurrences",
			Strings(&[]string{}, "many", nil, "").NArgs(0, 2),
			[]string{"--many", "a", "--many", "b", "--many", "c"},
			"argument specified too many times: --many",
		},
		{
			"OptionNeedsValue",
			String(new(string), "opt", "", ""),
			[]string{"--opt"},
			"option requires an argument: --opt",
		},
		{
			"UnrecognizedOption",
			String(new(string), "opt", "", ""),
			[]string{"--nope"},
			"unrecognized option: --nope",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCommand("test", "").Flags(tt.flag).Parse(tt.args)
			if err == nil {
				t.Fatalf("expected error for %v, got nil", tt.args)
			}
			if got, want := errorOrString(err), tt.want; got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
		})
	}
}

// TestArgumentErrorNamesPositional asserts the same for a positional, which
// renders as its upper-cased name rather than with a leading dash.
func TestArgumentErrorNamesPositional(t *testing.T) {
	var files []string
	cmd := NewCommand("test", "").Flags(
		Strings(&files, "file", nil, "").Positional().NArgs(1, 0),
	)
	_, err := cmd.Parse(nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := errorOrString(err), "missing required argument: FILE"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func TestValidateInvalidNArgs(t *testing.T) {
	for _, tt := range []struct {
		name     string
		min, max int
		want     string
	}{
		{"MinExceedsMax", 2, 1, "minimum count 2 exceeds maximum count 1"},
		{"NegativeMin", -1, 1, "minimum count must not be negative: -1"},
		{"NegativeMax", 0, -1, "maximum count must not be negative: -1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var a string
			cmd := NewCommand("test", "").Flags(
				String(&a, "foo", "", "").NArgs(tt.min, tt.max),
			)
			_, err := cmd.Parse(nil)
			if err == nil {
				t.Fatalf("NArgs(%d, %d): expected error, got nil", tt.min, tt.max)
			}
			if got, want := errorOrString(err), "--foo: "+tt.want; got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
		})
	}
}

// TestValidateUnboundedMaxIsNotExceeded asserts that a max of 0 means
// unbounded rather than a ceiling the min can breach, so required-and-
// repeatable is a valid configuration.
func TestValidateUnboundedMaxIsNotExceeded(t *testing.T) {
	var a []string
	cmd := NewCommand("test", "").Flags(
		Strings(&a, "foo", nil, "").NArgs(1, 0),
	)
	if _, err := cmd.Parse([]string{"--foo", "x"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValidateShortName asserts POSIX guideline 3: a short name is one
// character from [A-Za-z0-9].
func TestValidateShortName(t *testing.T) {
	for _, shortName := range []string{
		"xx", // more than one character
		"!",  // outside the portable character set
		"=",  // ... and this one the parser reads as a delimiter
		"-",
		" ",
		"é", // one character, but not one byte, and still not portable
	} {
		t.Run(shortName, func(t *testing.T) {
			var a string
			assertParseError(t, NewCommand("test", "").Flags(
				String(&a, "foo", "", "").ShortName(shortName),
			), "illegal short name")
		})
	}
	for _, shortName := range []string{"x", "X", "0"} {
		t.Run(shortName, func(t *testing.T) {
			var a string
			cmd := NewCommand("test", "").Flags(
				String(&a, "foo", "", "").ShortName(shortName),
			)
			if _, err := cmd.Parse(nil); err != nil {
				t.Errorf("expected %q to be a legal short name: %v", shortName, err)
			}
		})
	}
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

// *exec.ExitError implements ExitCoder without any help from this package,
// so a handler that shells out can return its error unchanged.
var _ ExitCoder = (*exec.ExitError)(nil)

// TestInvocationPath asserts that Parse reports the whole path of commands
// named by the arguments, root first, and the command they reached.
func TestInvocationPath(t *testing.T) {
	leaf := NewCommand("leaf", "")
	branch := NewCommand("branch", "").Subcommands(leaf)
	root := NewCommand("root", "").Subcommands(branch)

	inv, err := root.Parse([]string{"branch", "leaf"})
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, []string{"root", "branch", "leaf"}, inv.Path)
	if got, want := inv.Cmd, leaf; got != want {
		t.Errorf("Cmd = %v, want %v", got, want)
	}
}

// TestParseIsNotWrittenBack asserts that a parse leaves nothing behind on
// the command tree, so parsing twice yields two independent results.
func TestParseIsNotWrittenBack(t *testing.T) {
	cmd := NewCommand("test", "").ForwardArgs()
	first, err := cmd.Parse([]string{"--", "one"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := cmd.Parse([]string{"--", "two"})
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, []string{"one"}, first.Forwarded)
	assertStrings(t, []string{"two"}, second.Forwarded)
}

// TestRunExitCodes asserts the contract Run documents: 0 for success or
// help, 1 for a handler that failed, 2 for a command line that was wrong,
// and whatever an ExitCoder asks for. It also asserts which stream each
// outcome is reported on -- help on stdout, everything else on stderr.
func TestRunExitCodes(t *testing.T) {
	// handles returns a command whose handler returns err.
	handles := func(err error) *Command {
		return NewCommand("test", "").
			HandleFunc(func(ctx context.Context, inv *Invocation) error {
				return err
			})
	}
	tests := []struct {
		name     string
		cmd      *Command
		args     []string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{
			name:     "Success",
			cmd:      handles(nil),
			wantCode: 0,
		},
		{
			name:     "Help",
			cmd:      handles(nil),
			args:     []string{"--help"},
			wantCode: 0,
			wantOut:  "Usage: test\n",
		},
		{
			name:     "HandlerError",
			cmd:      handles(errors.New("boom")),
			wantCode: 1,
			wantErr:  "Error: boom\n",
		},
		{
			name:     "UnrecognizedArgument",
			cmd:      handles(nil),
			args:     []string{"--nope"},
			wantCode: 2,
			wantErr:  "Argument error: unrecognized option: --nope\n",
		},
		{
			name:     "NoHandler",
			cmd:      NewCommand("test", ""),
			wantCode: 2,
			wantErr:  "Usage: test\n",
		},
		{
			name: "ConfigError",
			cmd: NewCommand("test", "").
				Flags(
					String(new(string), "foo", "", ""),
					String(new(string), "foo", "", ""),
				),
			wantCode: 2,
			wantErr:  "Program error: test: flag already declared: --foo\n",
		},
		{
			name:     "Exit",
			cmd:      handles(Exit(3, errors.New("boom"))),
			wantCode: 3,
			wantErr:  "Error: boom\n",
		},
		{
			name:     "ExitWithoutError",
			cmd:      handles(Exit(3, nil)),
			wantCode: 3,
			wantErr:  "Error: exit status 3\n",
		},
		{
			name:     "Exitf",
			cmd:      handles(Exitf(3, "boom: %w", errors.New("kaboom"))),
			wantCode: 3,
			wantErr:  "Error: boom: kaboom\n",
		},
		{
			name:     "UsageError",
			cmd:      handles(Exitf(ExitCodeUsage, "--foo and --bar are exclusive")),
			wantCode: 2,
			wantErr:  "Error: --foo and --bar are exclusive\n",
		},
		{
			name:     "WrappedExitCoder",
			cmd:      handles(fmt.Errorf("child failed: %w", Exit(7, nil))),
			wantCode: 7,
			wantErr:  "Error: child failed: exit status 7\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCaptured(tt.cmd, tt.args...)
			if got, want := code, tt.wantCode; got != want {
				t.Errorf("exit code = %d, want %d", got, want)
			}
			assertOutput(t, "stdout", stdout, tt.wantOut)
			assertOutput(t, "stderr", stderr, tt.wantErr)
		})
	}
}

// assertOutput asserts that a captured stream starts with want, or is empty
// if want is empty. Only the first line of a help message is worth
// asserting here; format_test covers the rest.
func assertOutput(t *testing.T, name, got, want string) bool {
	t.Helper()
	if want == "" {
		if got != "" {
			t.Errorf("%s = %q, want nothing", name, got)
			return false
		}
		return true
	}
	if !strings.HasPrefix(got, want) {
		t.Errorf("%s = %q, want it to start with %q", name, got, want)
		return false
	}
	return true
}

// TestRunReportsOutputFailure asserts that a command whose own output cannot
// be written to reports the failure on os.Stderr and exits non-zero, rather
// than panicking. Both paths that write a help message are covered.
func TestRunReportsOutputFailure(t *testing.T) {
	tests := []struct {
		name string
		cmd  *Command
		args []string
	}{
		{
			name: "Help",
			cmd: NewCommand("test", "").
				HandleFunc(func(ctx context.Context, inv *Invocation) error {
					return nil
				}),
			args: []string{"--help"},
		},
		{
			name: "NoHandler",
			cmd:  NewCommand("test", ""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cmd.Stdout(errWriter{}).Stderr(errWriter{})
			var code int
			stderr := captureStderr(t, func() {
				code = tt.cmd.Run(context.Background(), tt.args)
			})
			if code == 0 {
				t.Errorf("exit code = 0, want non-zero")
			}
			if want := "xflags: write failed\n"; stderr != want {
				t.Errorf("os.Stderr = %q, want %q", stderr, want)
			}
		})
	}
}

// TestHandlerReceivesInvocation asserts that a handler is told how it was
// called: which command ran, the path it was reached by, and the arguments
// after the terminator.
func TestHandlerReceivesInvocation(t *testing.T) {
	var got *Invocation
	add := NewCommand("add", "").
		ForwardArgs().
		HandleFunc(func(ctx context.Context, inv *Invocation) error {
			got = inv
			return nil
		})
	app := NewCommand("myapp", "").
		Subcommands(NewCommand("remote", "").Subcommands(add))

	args := []string{"remote", "add", "--", "origin"}
	if code := app.Run(context.Background(), args); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got == nil {
		t.Fatal("handler was not called")
	}
	assertStrings(t, []string{"myapp", "remote", "add"}, got.Path)
	assertStrings(t, []string{"origin"}, got.Forwarded)
	if want := add; got.Cmd != want {
		t.Errorf("Cmd = %v, want %v", got.Cmd, want)
	}
}

// TestParseReportsHelpRequested asserts that asking for help is reported on
// the Invocation rather than as an error, naming the subcommand whose usage
// was asked for, so a caller doing its own dispatch can tell help apart from
// a failure.
func TestParseReportsHelpRequested(t *testing.T) {
	add := NewCommand("add", "")
	app := NewCommand("myapp", "").Subcommands(add)

	inv, err := app.Parse([]string{"add", "--help"})
	if err != nil {
		t.Fatalf("Parse() = %v, want no error", err)
	}
	if !inv.HelpRequested {
		t.Error("HelpRequested = false, want true")
	}
	if want := add; inv.Cmd != want {
		t.Errorf("Cmd = %v, want %v", inv.Cmd, want)
	}
}

// TestHelpSkipsFlagRules asserts that help is reported for a command line the
// user has not finished writing. Parsing stops at -h or --help, so a required
// flag that was never given is not held against them -- help is most useful
// to someone who does not yet know what to type.
func TestHelpSkipsFlagRules(t *testing.T) {
	var stdout strings.Builder
	cmd := NewCommand("test", "").
		Stdout(&stdout).
		Flags(String(new(string), "name", "", "").Required()).
		HandleFunc(func(ctx context.Context, inv *Invocation) error {
			t.Error("handler was called")
			return nil
		})

	if got, want := cmd.Run(context.Background(), []string{"--help"}), 0; got != want {
		t.Errorf("exit code = %d, want %d", got, want)
	}
	assertOutput(t, "stdout", stdout.String(), "Usage: test")
}

// TestHandlerStreams asserts that a handler reads and writes the streams on
// its invocation, and that they are resolved from wherever the command is
// mounted: redirecting the root captures what a subcommand's handler prints,
// which is most of what a CLI emits.
func TestHandlerStreams(t *testing.T) {
	echo := NewCommand("echo", "").
		HandleFunc(func(ctx context.Context, inv *Invocation) error {
			if _, err := io.Copy(inv.Stdout, inv.Stdin); err != nil {
				return err
			}
			fmt.Fprint(inv.Stderr, "echoed")
			return nil
		})
	app := NewCommand("app", "").
		Stdin(strings.NewReader("hello")).
		Subcommands(echo)

	var stdout, stderr strings.Builder
	app.Stdout(&stdout).Stderr(&stderr)
	if code := app.Run(context.Background(), []string{"echo"}); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	assertString(t, "hello", stdout.String())
	assertString(t, "echoed", stderr.String())
}

// TestStreamsResolveIndependently asserts that redirecting one stream leaves
// the others at the process defaults. They used to be inherited all or
// nothing, so redirecting stdout alone resolved a nil stderr and the first
// error message panicked.
func TestStreamsResolveIndependently(t *testing.T) {
	var stdout strings.Builder
	cmd := NewCommand("test", "").
		Stdout(&stdout).
		HandleFunc(func(ctx context.Context, inv *Invocation) error {
			return errors.New("boom")
		})

	var code int
	stderr := captureStderr(t, func() {
		code = cmd.Run(context.Background(), nil)
	})
	if got, want := code, 1; got != want {
		t.Errorf("exit code = %d, want %d", got, want)
	}
	if got, want := stderr, "Error: boom\n"; got != want {
		t.Errorf("os.Stderr = %q, want %q", got, want)
	}
	assertString(t, "", stdout.String())
}

// TestStreamsDefaultToProcess asserts that a command nobody has redirected
// hands its handler the process streams, rather than nil.
func TestStreamsDefaultToProcess(t *testing.T) {
	var got *Invocation
	cmd := NewCommand("test", "").
		HandleFunc(func(ctx context.Context, inv *Invocation) error {
			got = inv
			return nil
		})
	if code := cmd.Run(context.Background(), nil); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got.Stdin != os.Stdin {
		t.Errorf("Stdin = %v, want os.Stdin", got.Stdin)
	}
	if got.Stdout != os.Stdout {
		t.Errorf("Stdout = %v, want os.Stdout", got.Stdout)
	}
	if got.Stderr != os.Stderr {
		t.Errorf("Stderr = %v, want os.Stderr", got.Stderr)
	}
}

func ExampleInvocation() {
	// A team writes this command without knowing where it will be mounted,
	// so it reads its own name out of the invocation rather than repeating
	// it in the message.
	add := NewCommand("add", "Add a remote").
		HandleFunc(func(ctx context.Context, inv *Invocation) error {
			return Exitf(ExitCodeUsage,
				"no remote named: try \"%s --help\"",
				strings.Join(inv.Path, " "),
			)
		})

	// Whoever composes the binary decides where it hangs.
	app := NewCommand("myapp", "").
		Stderr(os.Stdout). // for tests
		Subcommands(NewCommand("remote", "Manage remotes").Subcommands(add))

	fmt.Println("exit code:", RunWithArgs(context.Background(), app, "remote", "add"))
	// Output:
	// Error: no remote named: try "myapp remote add --help"
	// exit code: 2
}
