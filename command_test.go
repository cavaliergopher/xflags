package xflags

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/cavaliergopher/xflags/ir"
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
		if err := cmd.Dispatch(context.Background(), args); err != nil {
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

func TestFromFlagSet(t *testing.T) {
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
		FlagGroups(FromFlagSet("native", "Native options", flagSet))
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

// TestSubcommandAlreadyParented asserts that Subcommands does not steal an
// already-parented command -- such as a shared registry like
// xflags.CommandLine -- and that the mismatch is reported as a
// ConfigError rather than silently corrupting the original relationship.
func TestSubcommandAlreadyParented(t *testing.T) {
	a, b, shared := NewCommand("a", ""), NewCommand("b", ""), NewCommand("shared", "")
	a.Subcommands(shared)
	b.Subcommands(shared)

	assertString(t, "a", shared.parent.name)
	if _, err := a.Parse(nil); err != nil {
		t.Errorf("a.Parse: expected nil error, got: %v", err)
	}
	assertConfigError(t, b, "a subcommand already parented elsewhere")
}

// TestSubcommandCycle asserts that a command tree that leads back into
// itself is reported as a ConfigError rather than walked forever. Every
// shape here wedged the process before the tree could be validated: the
// first three walking parent links to find a root that is not there, the
// last two descending subcommand links that lead back up.
func TestSubcommandCycle(t *testing.T) {
	tests := []struct {
		name string
		cmd  func() *Command
	}{
		{
			// A command mounted under itself, which Subcommands accepts
			// because its parent is nil at the time.
			name: "Self",
			cmd: func() *Command {
				a := NewCommand("a", "")
				return a.Subcommands(a)
			},
		},
		{
			name: "Mutual",
			cmd: func() *Command {
				a, b := NewCommand("a", ""), NewCommand("b", "")
				a.Subcommands(b)
				b.Subcommands(a)
				return a
			},
		},
		{
			name: "Deep",
			cmd: func() *Command {
				a, b, c := NewCommand("a", ""), NewCommand("b", ""), NewCommand("c", "")
				a.Subcommands(b)
				b.Subcommands(c)
				c.Subcommands(a)
				return a
			},
		},
		{
			// The parent links are acyclic here -- Subcommands leaves b's
			// parent alone, since a already claimed it -- so only the
			// descent through subcommands leads back up.
			name: "SubcommandsOnly",
			cmd: func() *Command {
				a, b, c := NewCommand("a", ""), NewCommand("b", ""), NewCommand("c", "")
				a.Subcommands(b)
				b.Subcommands(c)
				c.Subcommands(b)
				return a
			},
		},
		{
			name: "MountedTwice",
			cmd: func() *Command {
				a, b := NewCommand("a", ""), NewCommand("b", "")
				return a.Subcommands(b, b)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertConfigError(t, tt.cmd(), "a cycle in the command tree")
		})
	}
}

// TestSubcommandCycleFromDescendant asserts that the cycle is reported
// wherever Compile is called from, not only from the command that closes
// it.
func TestSubcommandCycleFromDescendant(t *testing.T) {
	a, b := NewCommand("a", ""), NewCommand("b", "")
	a.Subcommands(b)
	b.Subcommands(a)
	assertConfigError(t, b, "a cycle reached from a descendant")
}

func ExampleCommand_FlagGroups() {
	var n int
	var rightToLeft bool
	var endcoding string

	cmd := NewCommand("helloworld", "").
		// n flag defines how many times to print "Hello, World!".
		Flags(Int(&n, "n", 1, "Print n times")).

		// Mount a flag group for language-related flags.
		FlagGroups(NewFlagGroup(
			"language",
			"Language options",
			String(&endcoding, "encoding", "utf-8", "Text encoding"),
			Bool(&rightToLeft, "rtl", false, "Print right-to-left"),
		))

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

func ExampleFromFlagSet() {
	// create a Go-native flag set
	flagSet := flag.NewFlagSet("native", flag.ExitOnError)
	message := flagSet.String("m", "Hello, World!", "Message to print")

	// import the flagset into an xflags command as a flag group
	cmd := NewCommand("helloworld", "").
		FlagGroups(FromFlagSet("native", "Native options", flagSet)).
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
	// Native options:
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

func TestCompileRoot(t *testing.T) {
	sub := NewCommand("sub", "Sub command summary")
	root := NewCommand("root", "Root command summary").
		Description("Root description").
		Subcommands(sub)

	node, err := root.Compile()
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
	if got, want := len(node.Ancestry), 1; got != want {
		t.Errorf("len(Ancestry) = %d, want %d for a root", got, want)
	}
	if got, want := len(node.Subcommands), 1; got != want {
		t.Fatalf("len(Subcommands) = %d, want %d", got, want)
	}
	if got, want := node.Subcommands[0].Name, "sub"; got != want {
		t.Errorf("Subcommands[0].Name = %q, want %q", got, want)
	}
	subNode := node.Subcommands[0]
	if got, want := subNode.Ancestry, []*ir.Command{node, subNode}; !slices.Equal(got, want) {
		t.Errorf("Subcommands[0].Ancestry = %v, want %v", got, want)
	}
}

func TestCompileSubcommand(t *testing.T) {
	foo := NewCommand("foo", "Foo summary")
	bar := NewCommand("bar", "Bar summary")
	NewCommand("root", "Root summary").Subcommands(foo, bar)

	node, err := foo.Compile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := node.Name, "foo"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
	if got, want := len(node.Ancestry), 2; got != want {
		t.Fatalf("len(Ancestry) = %d, want %d", got, want)
	}
	root := node.Ancestry[0]
	if got, want := root.Name, "root"; got != want {
		t.Errorf("Ancestry[0].Name = %q, want %q", got, want)
	}
	if got, want := root, node.Root; got != want {
		t.Errorf("Ancestry[0] = %v, want it to be Root %v", got, want)
	}
	var names []string
	for _, c := range root.Subcommands {
		names = append(names, c.Name)
	}
	assertStrings(t, []string{"foo", "bar"}, names)
}

// TestCompileValidationError asserts that Compile returns the same
// configuration error that Parse would for a misconfigured tree.
func TestCompileValidationError(t *testing.T) {
	var a, b string
	cmd := NewCommand("test", "").Flags(
		String(&a, "foo", "", ""),
		String(&b, "foo", "", ""), // duplicate name: invalid
	)

	_, compileErr := cmd.Compile()
	if compileErr == nil {
		t.Fatal("expected error from Compile for duplicate flag name, got nil")
	}

	_, parseErr := cmd.Parse(nil)
	if parseErr == nil {
		t.Fatal("expected error from Parse for duplicate flag name, got nil")
	}
	if got, want := compileErr.Error(), parseErr.Error(); got != want {
		t.Errorf("Compile error %q, want the Parse error %q", got, want)
	}
}

// TestCompileIsPure asserts that Compile does not mutate the command tree
// or the variables flags are bound to: it must reflect neither a Parse that
// ran before it, nor any bookkeeping of its own.
func TestCompileIsPure(t *testing.T) {
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

	node, err := cmd.Compile()
	if err != nil {
		t.Fatalf("unexpected error from Compile: %v", err)
	}

	// The bound variable must still hold the parsed value: Compile must
	// not have written back to it.
	if got, want := s, "parsed-value"; got != want {
		t.Errorf("s = %q, want %q after Compile", got, want)
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

// TestValidateDuplicatePositionalName asserts that a duplicate name
// between two positional flags is reported as a duplicate operand, in
// the vocabulary a user of the command line would recognize, rather than
// the "flag" wording that fits an option.
func TestValidateDuplicatePositionalName(t *testing.T) {
	var a, b string
	_, err := NewCommand("test", "").Flags(
		String(&a, "file", "", "").Positional(),
		String(&b, "file", "", "").Positional(),
	).Parse(nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := humanMessage(err), "test: operand declared more than once: FILE"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

// TestConfigErrorNamesGrandchildByPath asserts that a ConfigError on a
// deep subcommand reports where it lives: the bare name "add" could be
// any command called "add", but "app remote add" is not.
func TestConfigErrorNamesGrandchildByPath(t *testing.T) {
	var a, b string
	add := NewCommand("add", "").Flags(
		String(&a, "name", "", ""),
		String(&b, "name", "", ""),
	)
	remote := NewCommand("remote", "").Subcommands(add)
	app := NewCommand("app", "").Subcommands(remote)

	_, err := app.Parse(nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := `app remote add: flag declared more than once: --name`
	if got := humanMessage(err); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

// TestValidateAncestorShadowing asserts the path-scoped naming rule: one
// option may not be claimed twice along an ancestor-descendant chain, by
// either spelling, however far up the path the ancestor is. See
// docs/adr/path-scoped-flag-names.md.
//
// The error names both commands, since ancestry is what tells a reader
// which end to change, and neither is called the offender: which was
// declared first is an accident of mount order. The command the error is
// reported against is still named by its full path, since a bare "sub" or
// "leaf" would not say which among possibly many.
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
			want: `root sub: flag declared on both "root" and "sub": --force`,
		},
		{
			name: "ShortName",
			cmd: NewCommand("root", "").
				Flags(String(new(string), "file", "", "").Aliases("f")).
				Subcommands(NewCommand("sub", "").Flags(
					String(new(string), "output", "", "").Aliases("f"),
				)),
			want: `root sub: flag declared on both "root" and "sub": -f`,
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
			want: `root mid leaf: flag declared on both "root" and "leaf": --force`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.cmd.Parse(nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got, want := humanMessage(err), tt.want; got != want {
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
			Bool(&deleteForce, "force", false, "").Aliases("f"),
		),
		NewCommand("push", "").Flags(
			Bool(&pushForce, "force", false, "").Aliases("f"),
		),
	)

	inv, err := app.Parse([]string{"delete", "--force"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := inv.Cmd.Name, "delete"; got != want {
		t.Errorf("Cmd = %q, want %q", got, want)
	}
	assertBool(t, true, deleteForce)
	assertBool(t, false, pushForce)

	inv, err = app.Parse([]string{"push", "-f"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := inv.Cmd.Name, "push"; got != want {
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
	// Each case builds its own command, since validateNArgs reports the
	// first unsatisfied flag and a shared one would let cases mask each
	// other.
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
			if got, want := humanMessage(err), tt.want; got != want {
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
	if got, want := humanMessage(err), "missing required argument: FILE"; got != want {
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
			if got, want := humanMessage(err), "--foo: "+tt.want; got != want {
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
		"!", // outside the portable character set
		"=", // ... and this one the parser reads as a delimiter
		"-",
		" ",
		"é", // one character, but not one byte, and still not portable
	} {
		t.Run(shortName, func(t *testing.T) {
			var a string
			assertParseError(t, NewCommand("test", "").Flags(
				String(&a, "foo", "", "").Aliases(shortName),
			), "illegal short name")
		})
	}
	for _, shortName := range []string{"x", "X", "0"} {
		t.Run(shortName, func(t *testing.T) {
			var a string
			cmd := NewCommand("test", "").Flags(
				String(&a, "foo", "", "").Aliases(shortName),
			)
			if _, err := cmd.Parse(nil); err != nil {
				t.Errorf("expected %q to be a legal short name: %v", shortName, err)
			}
		})
	}
}

// TestValidateCollectsAllErrors asserts that a malformed tree reports
// every configuration error in one run -- they surface in a batch at
// startup -- and that Run prints each on its own prefixed line.
func TestValidateCollectsAllErrors(t *testing.T) {
	var a, b, c string
	cmd := NewCommand("test", "").Flags(
		String(&a, "foo", "", ""),
		String(&b, "foo", "", ""),              // duplicate name
		String(&c, "bar", "", "").Aliases("!"), // illegal short name
	)
	_, err := cmd.Parse(nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var cfgErr *ir.ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected a *ConfigError in %v", err)
	}
	var code int
	stderr := captureStderr(t, func() {
		code = cmd.Run(context.Background(), nil)
	})
	if got, want := code, 2; got != want {
		t.Errorf("exit code = %d, want %d", got, want)
	}
	// Model errors precede spelling errors: Compile runs ir's validation,
	// which reads each flag on its own terms, before argv's, which reads
	// the spellings they render to. Order within a batch is not part of
	// the contract; that every error appears exactly once is.
	want := "Program error: --bar: short name must be one character from [A-Za-z0-9]: \"!\"\n" +
		"Program error: test: flag declared more than once: --foo\n"
	if got := stderr; got != want {
		t.Errorf("os.Stderr = %q, want %q", got, want)
	}
}

// TestConfigErrorIgnoresConfiguredStreams asserts that a malformed tree is
// reported on os.Stderr rather than on any stream the tree names, because
// those streams are part of what failed to validate. The error names a
// subcommand that redirected its own stderr -- a stream the composer of
// the binary neither chose nor reads -- which used to swallow the report
// whole.
func TestConfigErrorIgnoresConfiguredStreams(t *testing.T) {
	var rootErr, subErr strings.Builder
	sub := NewCommand("sub", "").
		Stderr(&subErr).
		Flags(
			String(new(string), "foo", "", ""),
			String(new(string), "foo", "", ""),
		)
	cmd := NewCommand("test", "").Stderr(&rootErr).Subcommands(sub)

	var code int
	stderr := captureStderr(t, func() {
		code = cmd.Run(context.Background(), []string{"sub"})
	})
	if got, want := code, 2; got != want {
		t.Errorf("exit code = %d, want %d", got, want)
	}
	want := "Program error: test sub: flag declared more than once: --foo\n"
	if got := stderr; got != want {
		t.Errorf("os.Stderr = %q, want %q", got, want)
	}
	assertString(t, "", rootErr.String())
	assertString(t, "", subErr.String())
}

// TestStreamsAreInherited asserts that a command with no stream of its
// own takes each one from the nearest ancestor that named it, and that
// the three resolve independently, so redirecting one leaves the others
// where they were.
func TestStreamsAreInherited(t *testing.T) {
	var rootOut, midErr strings.Builder
	leaf := NewCommand("leaf", "")
	mid := NewCommand("mid", "").Stderr(&midErr).Subcommands(leaf)
	NewCommand("root", "").Stdout(&rootOut).Subcommands(mid)

	node, err := leaf.Compile()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := node.Stdout, io.Writer(&rootOut); got != want {
		t.Errorf("Stdout = %v, want the root's", got)
	}
	if got, want := node.Stderr, io.Writer(&midErr); got != want {
		t.Errorf("Stderr = %v, want the middle command's", got)
	}
	if got, want := node.Stdin, io.Reader(os.Stdin); got != want {
		t.Errorf("Stdin = %v, want os.Stdin", got)
	}

	// A command that names its own wins over the ancestor it would
	// otherwise inherit from, which is what makes the inheritance a
	// default rather than a rule.
	var leafOut strings.Builder
	loud := NewCommand("loud", "").Stdout(&leafOut)
	NewCommand("root", "").Stdout(&rootOut).Subcommands(loud)

	own, err := loud.Compile()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := own.Stdout, io.Writer(&leafOut); got != want {
		t.Errorf("Stdout = %v, want the command's own, not the root's", got)
	}
}

// TestArgumentErrorWrapsArgumentErrorOnce asserts that an ArgumentError
// wrapping another, such as Choices reporting a bad value, prints its
// wrapped message plain: Error() tags it "xflags: " for a Go caller, and
// that tag must not leak into the sentence Run prints for a human.
func TestArgumentErrorWrapsArgumentErrorOnce(t *testing.T) {
	cmd := NewCommand("test", "").Flags(
		String(new(string), "foo", "", "").Choices("a", "b"),
	)
	code, _, stderr := runCaptured(cmd, "--foo=c")
	if got, want := code, 2; got != want {
		t.Errorf("exit code = %d, want %d", got, want)
	}
	want := "Argument error: --foo: expected one of: a, b\n" +
		"Usage: test [OPTIONS]\n" +
		"\n" +
		"Options:\n" +
		"   --foo  \n"
	if got := stderr; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

// TestHandlerJoinedErrorReportsWhole asserts that only the batches
// validation collects are split into one line per error: a handler's own
// joined error reports whole, keeping the wrapper text a per-line split
// would drop.
func TestHandlerJoinedErrorReportsWhole(t *testing.T) {
	errA := errors.New("a is stale")
	errB := errors.New("b is stale")
	cmd := NewCommand("test", "").HandleFunc(
		func(ctx context.Context, inv *Invocation) error {
			return fmt.Errorf("syncing a and b failed: %w, %w", errA, errB)
		},
	)
	code, _, stderr := runCaptured(cmd)
	if got, want := code, 1; got != want {
		t.Errorf("exit code = %d, want %d", got, want)
	}
	if got, want := stderr, "Error: syncing a and b failed: a is stale, b is stale\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

// TestValidateFlagName asserts that a long name containing "=" or
// whitespace is rejected -- both break parsing -- while names that merely
// look unusual, such as a hyphenated one, remain legal.
func TestValidateFlagName(t *testing.T) {
	for _, tt := range []struct {
		name string
		want string
	}{
		{"foo=bar", `flag name must not contain '=': "foo=bar"`},
		{"foo bar", `flag name must not contain whitespace: "foo bar"`},
		{"foo\tbar", `flag name must not contain whitespace: "foo\tbar"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var a string
			cmd := NewCommand("test", "").Flags(String(&a, tt.name, "", ""))
			_, err := cmd.Parse(nil)
			if err == nil {
				t.Fatalf("expected error for flag name %q, got nil", tt.name)
			}
			if got, want := humanMessage(err), "--"+tt.name+": "+tt.want; got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
		})
	}
	for _, name := range []string{"dry-run", "helper"} {
		t.Run(name, func(t *testing.T) {
			var a string
			cmd := NewCommand("test", "").Flags(String(&a, name, "", ""))
			if _, err := cmd.Parse(nil); err != nil {
				t.Errorf("expected %q to be a legal flag name: %v", name, err)
			}
		})
	}
}

// TestValidateReservedHelpNames asserts that a flag cannot claim "-h" or
// "--help": the parser resolves both before the flag table, so such a
// declaration would silently never fire.
func TestValidateReservedHelpNames(t *testing.T) {
	var a string
	for _, tt := range []struct {
		name string
		flag *Flag
		want string
	}{
		{
			"LongHelp",
			String(&a, "help", "", ""),
			"--help: flag name is reserved for help: --help",
		},
		{
			"ShortH",
			String(&a, "foo", "", "").Aliases("h"),
			"--foo: flag name is reserved for help: -h",
		},
		{
			"ShortOnlyH", // a one-character name is spelled with one dash
			String(&a, "h", "", ""),
			"-h: flag name is reserved for help: -h",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCommand("test", "").Flags(tt.flag).Parse(nil)
			if err == nil {
				t.Fatal("expected error for a reserved help name, got nil")
			}
			if got, want := humanMessage(err), tt.want; got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
		})
	}
	// Reservation is exact: "H" is not "h", "helper" is not "help".
	t.Run("Legal", func(t *testing.T) {
		var b string
		cmd := NewCommand("test", "").Flags(
			String(&a, "helper", "", "").Aliases("H"),
			String(&b, "no-help", "", ""),
		)
		if _, err := cmd.Parse(nil); err != nil {
			t.Errorf("expected nearby names to remain legal: %v", err)
		}
	})
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
	assertString(t, "root branch leaf", inv.Cmd.FullName)
	if got, want := inv.Cmd.String(), leaf.String(); got != want {
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

// TestParseRestoresDefaults asserts that a second Parse of the same tree
// starts from the declared defaults rather than inheriting the first
// parse's values -- trees get parsed twice mostly in tests.
func TestParseRestoresDefaults(t *testing.T) {
	var name string
	var items []string
	var word uint64
	cmd := NewCommand("test", "").Flags(
		String(&name, "name", "default", ""),
		Strings(&items, "item", []string{"a"}, ""),
		BitField(&word, 0x1, "one", false, ""),
		BitField(&word, 0x2, "two", true, ""),
	)
	args := []string{"--name=x", "--item=p", "--item=q", "--one"}
	for i := 0; i < 2; i++ {
		if _, err := cmd.Parse(args); err != nil {
			t.Fatal(err)
		}
		assertString(t, "x", name)
		assertStrings(t, []string{"p", "q"}, items)
		assertUint64(t, 0x3, word)
	}
	// A parse of nothing restores every default outright.
	if _, err := cmd.Parse(nil); err != nil {
		t.Fatal(err)
	}
	assertString(t, "default", name)
	assertStrings(t, []string{"a"}, items)
	assertUint64(t, 0x2, word)
}

// TestParseResetsEnvironmentValue asserts that a flag filled from its
// environment variable on one parse returns to its default on a later
// parse run without the variable set.
func TestParseResetsEnvironmentValue(t *testing.T) {
	var name string
	cmd := NewCommand("test", "").Flags(
		String(&name, "name", "default", "").Env("XFLAGS_TEST_NAME"),
	)
	t.Setenv("XFLAGS_TEST_NAME", "from-env")
	if _, err := cmd.Parse(nil); err != nil {
		t.Fatal(err)
	}
	assertString(t, "from-env", name)
	os.Unsetenv("XFLAGS_TEST_NAME") // t.Setenv restores it after the test
	if _, err := cmd.Parse(nil); err != nil {
		t.Fatal(err)
	}
	assertString(t, "default", name)
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
		// wantProcErr is what reaches os.Stderr rather than the command's
		// own stream, which only a malformed tree does.
		wantProcErr string
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
			wantErr:  "Argument error: unrecognized option: --nope\nUsage: test\n",
		},
		{
			name:     "NoHandler",
			cmd:      NewCommand("test", ""),
			wantCode: 2,
			wantErr:  "Argument error: missing subcommand\nUsage: test\n",
		},
		{
			name: "ConfigError",
			cmd: NewCommand("test", "").
				Flags(
					String(new(string), "foo", "", ""),
					String(new(string), "foo", "", ""),
				),
			wantCode:    2,
			wantProcErr: "Program error: test: flag declared more than once: --foo\n",
		},
		{
			// A tree that leads back into itself reports like any other
			// malformed tree, rather than wedging with no output at all.
			name: "CycleConfigError",
			cmd: func() *Command {
				cmd, sub := NewCommand("test", ""), NewCommand("sub", "")
				cmd.Subcommands(sub)
				sub.Subcommands(cmd)
				return cmd
			}(),
			wantCode:    2,
			wantProcErr: "Program error: \"test\" is its own ancestor\n",
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
			var code int
			var stdout, stderr string
			procErr := captureStderr(t, func() {
				code, stdout, stderr = runCaptured(tt.cmd, tt.args...)
			})
			if got, want := code, tt.wantCode; got != want {
				t.Errorf("exit code = %d, want %d", got, want)
			}
			assertOutput(t, "stdout", stdout, tt.wantOut)
			assertOutput(t, "stderr", stderr, tt.wantErr)
			assertOutput(t, "os.Stderr", procErr, tt.wantProcErr)
		})
	}
}

// assertOutput asserts that a captured stream starts with want, or is empty
// if want is empty. Only the first line of a help message is worth
// asserting here; usage_test covers the rest.
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

// TestArgumentErrorsPrintUsage asserts that a wrong command line is
// reported with the usage of the command that the error names, on the same
// stderr stream, error line first. Help is not error reporting: it still
// prints on stdout alone and exits 0. See
// docs/adr/argument-errors-print-usage.md.
func TestArgumentErrorsPrintUsage(t *testing.T) {
	newCmd := func() *Command {
		sub := NewCommand("sub", "").
			HandleFunc(func(ctx context.Context, inv *Invocation) error {
				return nil
			})
		return NewCommand("test", "").Subcommands(sub)
	}
	t.Run("BadFlag", func(t *testing.T) {
		code, stdout, stderr := runCaptured(newCmd(), "sub", "--nope")
		if got, want := code, 2; got != want {
			t.Errorf("exit code = %d, want %d", got, want)
		}
		// The usage is the subcommand's, where the error happened, not the
		// root's, where Run was called.
		assertOutput(t, "stderr", stderr,
			"Argument error: unrecognized option: --nope\nUsage: test sub\n")
		assertOutput(t, "stdout", stdout, "")
	})
	t.Run("NoHandler", func(t *testing.T) {
		code, stdout, stderr := runCaptured(newCmd())
		if got, want := code, 2; got != want {
			t.Errorf("exit code = %d, want %d", got, want)
		}
		assertOutput(t, "stderr", stderr,
			"Argument error: missing subcommand\nUsage: test COMMAND\n")
		assertOutput(t, "stdout", stdout, "")
	})
	t.Run("Help", func(t *testing.T) {
		code, stdout, stderr := runCaptured(newCmd(), "--help")
		if got, want := code, 0; got != want {
			t.Errorf("exit code = %d, want %d", got, want)
		}
		assertOutput(t, "stdout", stdout, "Usage: test COMMAND\n")
		assertOutput(t, "stderr", stderr, "")
	})
	// The other two error classes stay one line: a handler error means the
	// command line was right, and a config error means the usage message
	// cannot be trusted.
	t.Run("HandlerError", func(t *testing.T) {
		cmd := NewCommand("test", "").
			HandleFunc(func(ctx context.Context, inv *Invocation) error {
				return errors.New("boom")
			})
		code, _, stderr := runCaptured(cmd)
		if got, want := code, 1; got != want {
			t.Errorf("exit code = %d, want %d", got, want)
		}
		if got, want := stderr, "Error: boom\n"; got != want {
			t.Errorf("stderr = %q, want %q", got, want)
		}
	})
	t.Run("ConfigError", func(t *testing.T) {
		cmd := NewCommand("test", "").Flags(
			String(new(string), "foo", "", ""),
			String(new(string), "foo", "", ""),
		)
		var code int
		var stderr string
		procErr := captureStderr(t, func() {
			code, _, stderr = runCaptured(cmd)
		})
		if got, want := code, 2; got != want {
			t.Errorf("exit code = %d, want %d", got, want)
		}
		want := "Program error: test: flag declared more than once: --foo\n"
		if got := procErr; got != want {
			t.Errorf("os.Stderr = %q, want %q", got, want)
		}
		assertString(t, "", stderr)
	})
}

// TestDispatchReturnsRawError asserts Dispatch's half of the split from
// Run: the error comes back raw, with its Cmd naming the command that
// produced it, and no error text is printed. Help is the exception,
// because it is not error reporting: usage goes to stdout and Dispatch
// returns nil.
func TestDispatchReturnsRawError(t *testing.T) {
	newCmd := func() *Command {
		sub := NewCommand("sub", "").
			HandleFunc(func(ctx context.Context, inv *Invocation) error {
				return nil
			})
		return NewCommand("test", "").Subcommands(sub)
	}
	// dispatchCaptured is runCaptured for Dispatch.
	dispatchCaptured := func(cmd *Command, args ...string) (err error, stdout, stderr string) {
		var out, errOut strings.Builder
		cmd.Stdout(&out).Stderr(&errOut)
		err = cmd.Dispatch(context.Background(), args)
		return err, out.String(), errOut.String()
	}
	// asArgumentError asserts that err carries an *ArgumentError naming the
	// command called wantCmd.
	asArgumentError := func(t *testing.T, err error, wantCmd string) {
		t.Helper()
		var argErr *ir.ArgumentError
		if !errors.As(err, &argErr) {
			t.Fatalf("err = %v, want *ArgumentError", err)
		}
		if argErr.Cmd == nil {
			t.Fatal("err.Cmd = nil, want the command that produced it")
		}
		if got, want := argErr.Cmd.String(), wantCmd; got != want {
			t.Errorf("err.Cmd = %q, want %q", got, want)
		}
	}
	t.Run("BadFlag", func(t *testing.T) {
		err, stdout, stderr := dispatchCaptured(newCmd(), "sub", "--nope")
		asArgumentError(t, err, "sub")
		assertOutput(t, "stdout", stdout, "")
		assertOutput(t, "stderr", stderr, "")
	})
	t.Run("NoHandler", func(t *testing.T) {
		err, stdout, stderr := dispatchCaptured(newCmd())
		asArgumentError(t, err, "test")
		assertOutput(t, "stdout", stdout, "")
		assertOutput(t, "stderr", stderr, "")
	})
	t.Run("Help", func(t *testing.T) {
		err, stdout, stderr := dispatchCaptured(newCmd(), "--help")
		if err != nil {
			t.Errorf("err = %v, want nil", err)
		}
		assertOutput(t, "stdout", stdout, "Usage: test COMMAND\n")
		assertOutput(t, "stderr", stderr, "")
	})
	t.Run("HandlerError", func(t *testing.T) {
		boom := errors.New("boom")
		cmd := NewCommand("test", "").
			HandleFunc(func(ctx context.Context, inv *Invocation) error {
				return boom
			})
		err, stdout, stderr := dispatchCaptured(cmd)
		if got, want := err, boom; got != want {
			t.Errorf("err = %v, want %v", got, want)
		}
		assertOutput(t, "stdout", stdout, "")
		assertOutput(t, "stderr", stderr, "")
	})
}

// TestRunReportsOutputFailure asserts that a command whose own output cannot
// be written to reports the failure on os.Stderr and exits non-zero, rather
// than panicking. Both streams a report can go to are covered: a help
// message that cannot reach stdout, and an error line that cannot reach
// stderr.
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

// TestRunReportsUsageWriteFailure asserts that the usage message following
// an argument error has the same fallback as the error line: a stderr that
// goes away between the two is still reported on os.Stderr.
func TestRunReportsUsageWriteFailure(t *testing.T) {
	cmd := NewCommand("test", "").Stderr(&failAfterWriter{n: 1})
	var code int
	stderr := captureStderr(t, func() {
		code = cmd.Run(context.Background(), nil)
	})
	if code == 0 {
		t.Errorf("exit code = 0, want non-zero")
	}
	if want := "xflags: write failed\n"; stderr != want {
		t.Errorf("os.Stderr = %q, want %q", stderr, want)
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
	assertString(t, "myapp remote add", got.Cmd.FullName)
	assertStrings(t, []string{"origin"}, got.Forwarded)
	if want := add.String(); got.Cmd.String() != want {
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
	if want := add.String(); inv.Cmd.String() != want {
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

// TestUsageFuncIsInherited asserts that a custom renderer set on one
// command serves its subcommands too, and that the command it is handed is
// the one being described rather than the one that set it. Compile
// resolves the renderer the way it resolves the streams, so a subcommand
// that sets none carries its nearest ancestor's.
func TestUsageFuncIsInherited(t *testing.T) {
	var stdout strings.Builder
	root := NewCommand("root", "Root summary").
		Stdout(&stdout).
		UsageFunc(func(w io.Writer, cmd *ir.Command) error {
			_, err := fmt.Fprintf(w, "custom help for %s\n", cmd.FullName)
			return err
		}).
		Subcommands(
			NewCommand("child", "Child summary").
				HandleFunc(func(ctx context.Context, inv *Invocation) error {
					return nil
				}),
		)

	if code := root.Run(context.Background(), []string{"child", "--help"}); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got, want := stdout.String(), "custom help for root child\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

// TestMarshalOmitsBehavior guards the json:"-" tags on ir.Command's and
// ir.Flag's behavior fields. It compiles a tree exercising every one of
// them -- a handler, a custom UsageFunc, all three stream overrides, a
// bound Value, a ValidateFunc, a Choices list, a subcommand and a
// positional -- and marshals it, so a behavior field added later without
// its tag fails here, in a test, rather than leaking into a program's
// machine-readable output.
// TestParseFromSubcommandResetsTheTree asserts that Parse resets every
// flag in the tree it is called on, not merely the subtree below the
// command it was called on: a value an earlier parse left on an ancestor
// would otherwise survive into the next one.
func TestParseFromSubcommandResetsTheTree(t *testing.T) {
	var level string
	sub := NewCommand("sub", "")
	root := NewCommand("app", "").
		Flags(String(&level, "level", "info", "")).
		Subcommands(sub)

	if _, err := root.Parse([]string{"--level=debug", "sub"}); err != nil {
		t.Fatal(err)
	}
	assertString(t, "debug", level)

	if _, err := sub.Parse(nil); err != nil {
		t.Fatal(err)
	}
	assertString(t, "info", level)
}

func TestMarshalOmitsBehavior(t *testing.T) {
	var name, arg string
	sub := NewCommand("sub", "Sub summary").
		HandleFunc(func(ctx context.Context, inv *Invocation) error {
			return nil
		}).
		Flags(
			String(&name, "name", "", "usage").
				Validate(func(s string) error { return nil }).
				Choices("a", "b").
				Complete(func(inv *Invocation, word string) ([]string, ir.CompDirective) {
					return nil, ir.CompDefault
				}),
			String(&arg, "ARG", "", "positional usage").Positional(),
		)
	root := NewCommand("root", "Root summary").
		UsageFunc(func(w io.Writer, cmd *ir.Command) error { return nil }).
		Stdin(strings.NewReader("")).
		Stdout(&strings.Builder{}).
		Stderr(&strings.Builder{}).
		Subcommands(sub)

	node, err := root.Compile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	assertNoBehaviorKeys(t, m)
}

// behaviorKeys names every field ir.Command and ir.Flag tag json:"-".
var behaviorKeys = []string{
	"Handler", "UsageFunc", "Stdin", "Stdout", "Stderr",
	"Value", "ValidateFunc", "CompleteFunc",
}

// assertNoBehaviorKeys walks a value decoded from JSON -- maps and slices,
// since a compiled tree nests flags inside groups and subcommands inside
// subcommands -- and fails t if any behaviorKeys entry appears as a key
// anywhere in it.
func assertNoBehaviorKeys(t *testing.T, v any) {
	t.Helper()
	switch val := v.(type) {
	case map[string]any:
		for _, key := range behaviorKeys {
			if _, ok := val[key]; ok {
				t.Errorf("marshaled JSON contains behavior field %q", key)
			}
		}
		for _, child := range val {
			assertNoBehaviorKeys(t, child)
		}
	case []any:
		for _, child := range val {
			assertNoBehaviorKeys(t, child)
		}
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
				inv.Cmd.FullName,
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

// TestValidateDefaultNotAmongChoices asserts that a default outside the
// declared choices is a configuration error. Defaults bypass Set, so such
// a default would survive parsing and be advertised by help as a value the
// same program rejects on the command line.
func TestValidateDefaultNotAmongChoices(t *testing.T) {
	var env string
	cmd := NewCommand("test", "").Flags(
		String(&env, "env", "bogus", "").Choices("staging", "production"),
	)
	_, err := cmd.Parse(nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := humanMessage(err),
		`--env: default "bogus" is not one of: staging, production`; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

// TestValidateEmptyDefaultWithChoices asserts that an empty default is not
// held to the choices: it is how a flag says it has no default, and it is
// what every Required choice flag declares.
func TestValidateEmptyDefaultWithChoices(t *testing.T) {
	var env string
	cmd := NewCommand("test", "").Flags(
		String(&env, "env", "", "").Choices("staging", "production").Required(),
	)
	if _, err := cmd.Parse([]string{"--env=staging"}); err != nil {
		t.Fatal(err)
	}
	assertString(t, "staging", env)
}

// TestValidateRepeatableDefaultWithChoices asserts that a repeatable flag
// escapes the default-among-choices rule: it accumulates, so its default
// renders as the whole collection rather than as a value any one choice
// could match.
func TestValidateRepeatableDefaultWithChoices(t *testing.T) {
	var tags []string
	cmd := NewCommand("test", "").Flags(
		Strings(&tags, "tag", nil, "").Choices("red", "blue"),
	)
	if _, err := cmd.Parse([]string{"--tag=red", "--tag=blue"}); err != nil {
		t.Fatal(err)
	}
}

// TestValidatePositionalAlias asserts that a positional argument takes no
// alias. One would be a name nothing could match, since a positional never
// enters the option table, so it is reported rather than ignored.
func TestValidatePositionalAlias(t *testing.T) {
	var s string
	_, err := NewCommand("test", "").Flags(
		String(&s, "file", "", "").Aliases("f").Positional(),
	).Parse(nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := humanMessage(err), "FILE: positional arguments do not support aliases"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

// TestValidateFlagWithoutName asserts that a flag of nothing but empty
// slots is rejected: it can never be named on the command line, and has no
// canonical name for an error to report it by.
func TestValidateFlagWithoutName(t *testing.T) {
	var s string
	_, err := NewCommand("test", "").Flags(
		String(&s, "", "", "").Aliases(""),
	).Parse(nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := humanMessage(err), "unknown: flag must declare a name"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

// TestNegationCollision asserts what generating a spelling costs: a
// program can now collide with a name that appears nowhere in its source.
// The error has to say where the spelling came from, or the author is
// left looking for a declaration that does not exist.
func TestNegationCollision(t *testing.T) {
	for _, tt := range []struct {
		name string
		cmd  *Command
		want string
	}{
		{
			name: "GeneratedAgainstDeclared",
			cmd: NewCommand("app", "").Flags(
				Bool(new(bool), "cache", false, ""),
				Bool(new(bool), "no-cache", false, ""),
			),
			want: "app: flag declared more than once: --no-cache (generated from --cache)",
		},
		{
			name: "GeneratedAgainstDeclaredValueFlag",
			cmd: NewCommand("app", "").Flags(
				Bool(new(bool), "cache", false, ""),
				String(new(string), "no-cache", "", ""),
			),
			want: "app: flag declared more than once: --no-cache (generated from --cache)",
		},
		{
			name: "GeneratedAgainstAncestor",
			cmd: NewCommand("root", "").
				Flags(Bool(new(bool), "cache", false, "")).
				Subcommands(NewCommand("sub", "").Flags(
					Bool(new(bool), "no-cache", false, ""),
				)),
			want: `root sub: flag declared on both "root" and "sub": --no-cache (generated from --cache)`,
		},
		{
			// Two booleans that collide on a declared name collide on its
			// negation too, but that is one mistake, so it is reported
			// once and by the name the author actually wrote.
			name: "ShadowCollisionIsReportedOnce",
			cmd: NewCommand("app", "").Flags(
				Bool(new(bool), "force", false, "").Aliases("f"),
				Bool(new(bool), "force", false, "").Aliases("g"),
			),
			want: "app: flag declared more than once: --force",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.cmd.Parse(nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got, want := humanMessage(err), tt.want; got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
		})
	}
}

// TestOperandDoesNotCollideWithOption asserts what a positional stopped
// claiming when the collision check moved to value names: it answers to
// no option, so it cannot shadow one. A command taking a SERVICE operand
// may also declare --service, since the two share no spelling anywhere a
// reader sees them.
func TestOperandDoesNotCollideWithOption(t *testing.T) {
	var operand, option string
	_, err := NewCommand("test", "").Flags(
		String(&operand, "service", "", "").Positional(),
		String(&option, "service", "", ""),
	).Parse([]string{"web"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := operand, "web"; got != want {
		t.Errorf("operand = %q, want %q", got, want)
	}
}

// TestDuplicateValueNameCollides asserts the collision a positional does
// still have: two operands shown by the same value name, which no error
// message could tell apart. An explicit ValueName is enough, since what a
// reader sees is the whole of the ambiguity.
func TestDuplicateValueNameCollides(t *testing.T) {
	var a, b string
	_, err := NewCommand("test", "").Flags(
		String(&a, "src", "", "").Positional().ValueName("PATH"),
		String(&b, "dst", "", "").Positional().ValueName("PATH"),
	).Parse(nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := humanMessage(err), "test: operand declared more than once: PATH"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

// TestUsageFuncIsInheritedAtCompileTime asserts that a command carries the
// renderer it will be printed with, rather than the Usage method going
// looking for one up the tree: a subcommand that sets none compiles with
// its nearest ancestor's, and one that sets its own keeps it.
func TestUsageFuncIsInheritedAtCompileTime(t *testing.T) {
	rootFunc := func(w io.Writer, cmd *ir.Command) error {
		_, err := io.WriteString(w, "root renderer\n")
		return err
	}
	leafFunc := func(w io.Writer, cmd *ir.Command) error {
		_, err := io.WriteString(w, "leaf renderer\n")
		return err
	}
	own := NewCommand("own", "").UsageFunc(leafFunc)
	inherits := NewCommand("inherits", "").Subcommands(
		NewCommand("deep", ""),
	)
	node, err := NewCommand("app", "").UsageFunc(rootFunc).
		Subcommands(own, inherits).Compile()
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		path []string
		want string
	}{
		{nil, "root renderer\n"},
		{[]string{"own"}, "leaf renderer\n"},
		{[]string{"inherits"}, "root renderer\n"},
		{[]string{"inherits", "deep"}, "root renderer\n"}, // two levels up
	} {
		t.Run(strings.Join(append([]string{"app"}, tt.path...), " "), func(t *testing.T) {
			cmd := node
			for _, name := range tt.path {
				for _, sub := range cmd.Subcommands {
					if sub.Name == name {
						cmd = sub
					}
				}
			}
			if cmd.UsageFunc == nil {
				t.Fatal("UsageFunc was not resolved at compile time")
			}
			var buf strings.Builder
			if err := cmd.Usage(&buf); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("Usage() = %q, want %q", got, tt.want)
			}
		})
	}

	// A tree that sets none leaves it nil, so Usage falls back.
	bare, err := NewCommand("bare", "").Compile()
	if err != nil {
		t.Fatal(err)
	}
	if bare.UsageFunc != nil {
		t.Error("UsageFunc = non-nil, want nil so Usage falls back to the default")
	}
}

// TestAncestryIsResolvedAtCompileTime asserts that each compiled command
// carries the commands whose flags are in scope at it, from the root down,
// so nothing reading the tree walks back up to work it out. Siblings must
// not share a backing array: appending to the parent's slice in place
// would let the second subcommand overwrite the first.
func TestAncestryIsResolvedAtCompileTime(t *testing.T) {
	node, err := NewCommand("app", "").Subcommands(
		NewCommand("one", "").Subcommands(NewCommand("deep", "")),
		NewCommand("two", ""),
	).Compile()
	if err != nil {
		t.Fatal(err)
	}

	names := func(c *ir.Command) []string {
		var out []string
		for _, a := range c.Ancestry {
			out = append(out, a.Name)
		}
		return out
	}
	one, two := node.Subcommands[0], node.Subcommands[1]
	deep := one.Subcommands[0]

	for _, tt := range []struct {
		cmd  *ir.Command
		want []string
	}{
		{node, []string{"app"}},
		{one, []string{"app", "one"}},
		{two, []string{"app", "two"}},
		{deep, []string{"app", "one", "deep"}},
	} {
		if got := names(tt.cmd); !slices.Equal(got, tt.want) {
			t.Errorf("%s.Ancestry = %v, want %v", tt.cmd.Name, got, tt.want)
		}
	}

	// The ancestry ends with the command itself, and begins at its root.
	if got, want := deep.Ancestry[len(deep.Ancestry)-1], deep; got != want {
		t.Errorf("last of Ancestry = %v, want the command itself", got)
	}
	if got, want := deep.Ancestry[0], node; got != want {
		t.Errorf("first of Ancestry = %v, want the root", got)
	}
}
