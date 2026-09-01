package climux

import (
	"context"
	"strings"
	"testing"
)

// TestShortOptionGrouping covers POSIX guideline 5: short names are
// consumed while each takes no value, and the first that takes one takes
// the remainder of the argument. Every case here was confirmed against
// getopt(3) with the same declarations.
func TestShortOptionGrouping(t *testing.T) {
	for _, tt := range []struct {
		args []string
		a, b bool
		f    string
		arg  string
		err  bool
	}{
		{args: []string{"-a"}, a: true},
		{args: []string{"-ab"}, a: true, b: true},
		{args: []string{"-ba"}, a: true, b: true},
		{args: []string{"-a", "-b"}, a: true, b: true},

		// The first name that takes a value ends the cluster.
		{args: []string{"-abfx"}, a: true, b: true, f: "x"},
		{args: []string{"-abf", "x"}, a: true, b: true, f: "x"},
		{args: []string{"-abf=x"}, a: true, b: true, f: "x"},
		{args: []string{"-fab"}, f: "ab"},
		{args: []string{"-fx"}, f: "x"},
		{args: []string{"-f", "x"}, f: "x"},

		// A value ends the cluster even when it looks like more names.
		{args: []string{"-f-5"}, f: "-5"},
		{args: []string{"-fa"}, f: "a"},

		{args: []string{"-abf"}, err: true},      // nothing left to take
		{args: []string{"-abz"}, err: true},      // no such flag
		{args: []string{"-af", "-b"}, err: true}, // detached value parses as an option

		// "=" after a boolean is a delimiter, not another name, so a
		// short boolean can be set false like a long one.
		{args: []string{"-a=false"}, a: false},
		{args: []string{"-a=true"}, a: true},
		{args: []string{"-ba=false"}, b: true, a: false},
		{args: []string{"-ab=false"}, a: true, b: false},
		{args: []string{"-a=nonsense"}, err: true},
		{args: []string{"-a="}, err: true}, // the empty string is not a bool
		{args: []string{"-a", "false"}, a: true, arg: "false"},

		// An operand is still an operand.
		{args: []string{"-a", "arg"}, a: true, arg: "arg"},
	} {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var a, b bool
			var f, operand string
			cmd := NewCommand("test", "").Flags(
				Bool(&a, "alpha", false, "").Aliases("a"),
				Bool(&b, "bravo", false, "").Aliases("b"),
				String(&f, "foxtrot", "", "").Aliases("f"),
				String(&operand, "OPERAND", "", "").Positional(),
			)
			_, err := Parse(cmd, tt.args...)
			if tt.err {
				if err == nil {
					t.Fatalf("expected an error, got a=%v b=%v f=%q", a, b, f)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertBool(t, tt.a, a)
			assertBool(t, tt.b, b)
			assertString(t, tt.f, f)
			assertString(t, tt.arg, operand)
		})
	}
}

// TestAttachedValues covers the forms that turn on whether an
// option-argument arrived attached to its option. Guideline 14 makes a
// detached value that begins with "-" a missing value, so the attached
// form is the only way to give one, and a boolean reads a value only when
// it is attached. See docs/adr/posix-argument-conventions.md.
func TestAttachedValues(t *testing.T) {
	for _, tt := range []struct {
		args    []string
		count   int
		verbose bool
		name    string
		err     bool
	}{
		// A negative number is only ever attached: GitHub issue #4.
		{args: []string{"--count=-5"}, count: -5},
		{args: []string{"-c-5"}, count: -5},
		{args: []string{"-c=-5"}, count: -5},
		{args: []string{"--count=5"}, count: 5},
		{args: []string{"--count", "5"}, count: 5},
		{args: []string{"--count", "-5"}, err: true},
		{args: []string{"--count", "--verbose"}, err: true},
		{args: []string{"--count"}, err: true},

		// Any value may begin with a dash, not just a number.
		{args: []string{"--name=--x"}, name: "--x"},
		{args: []string{"--name=-"}, name: "-"},
		{args: []string{"--name="}, name: ""},
		{args: []string{"-n=value"}, name: "value"},
		{args: []string{"-nvalue"}, name: "value"},

		// A boolean takes an attached value and never a detached one.
		{args: []string{"--verbose"}, verbose: true},
		{args: []string{"--verbose=false"}, verbose: false},
		{args: []string{"--verbose=true"}, verbose: true},
		{args: []string{"--verbose=nonsense"}, err: true},
		{args: []string{"--verbose", "false"}, err: true}, // "false" is an operand
	} {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var count int
			var verbose bool
			var name string
			cmd := NewCommand("test", "").Flags(
				Int(&count, "count", 0, "").Aliases("c"),
				Bool(&verbose, "verbose", false, ""),
				String(&name, "name", "", "").Aliases("n"),
			)
			_, err := Parse(cmd, tt.args...)
			if tt.err {
				if err == nil {
					t.Fatalf("expected an error, got count=%d verbose=%v name=%q", count, verbose, name)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertInt64(t, int64(tt.count), int64(count))
			assertBool(t, tt.verbose, verbose)
			assertString(t, tt.name, name)
		})
	}
}

// TestEmptyAttachedValue asserts that an attached "=" with nothing after
// it sets the empty string by either spelling, as getopt_long reads
// "--name=", and that a boolean rejects it identically by either spelling
// -- a flag's two names must agree; see
// docs/adr/posix-argument-conventions.md.
func TestEmptyAttachedValue(t *testing.T) {
	// A non-empty default proves the empty string was set rather than
	// nothing at all.
	parseName := func(arg string) string {
		t.Helper()
		var name string
		cmd := NewCommand("test", "").Flags(
			String(&name, "name", "unset", "").Aliases("n"),
		)
		if _, err := Parse(cmd, arg); err != nil {
			t.Fatal(err)
		}
		return name
	}
	assertString(t, "", parseName("-n="))
	assertString(t, "", parseName("--name="))

	parseVerbose := func(arg string) error {
		cmd := NewCommand("test", "").Flags(
			Bool(new(bool), "verbose", false, "").Aliases("v"),
		)
		_, err := Parse(cmd, arg)
		return err
	}
	errShort, errLong := parseVerbose("-v="), parseVerbose("--verbose=")
	if errShort == nil || errLong == nil {
		t.Fatalf("expected errors, got %v and %v", errShort, errLong)
	}
	assertString(t, errLong.Error(), errShort.Error())
}

// TestUnrecognizedOptionNamesSubtree asserts that an unknown option
// declared somewhere below the current command is reported with the
// subcommand that declares it, since the name is legal only after that
// command is named. The first declarer in depth-first declaration order
// is named, by its own name rather than its path. See
// docs/adr/path-scoped-flag-names.md.
func TestUnrecognizedOptionNamesSubtree(t *testing.T) {
	newApp := func() *Command {
		del := NewCommand("delete", "").Flags(
			Bool(new(bool), "force", false, "").Aliases("f"),
			Bool(new(bool), "dry-run", false, ""),
		)
		add := NewCommand("add", "").Flags(
			Bool(new(bool), "tags", false, "").Aliases("t"),
			Bool(new(bool), "dry-run", false, ""),
		)
		remote := NewCommand("remote", "").Subcommands(add)
		return NewCommand("app", "").Subcommands(del, remote)
	}
	for _, tt := range []struct {
		args []string
		want string
	}{
		// A direct child's declaration, by either spelling.
		{[]string{"--force"}, `unrecognized option: --force (defined by subcommand "delete")`},
		{[]string{"-f"}, `unrecognized option: -f (defined by subcommand "delete")`},

		// A grandchild is named by its own name, not its path.
		{[]string{"--tags"}, `unrecognized option: --tags (defined by subcommand "add")`},
		{[]string{"-t"}, `unrecognized option: -t (defined by subcommand "add")`},

		// The search starts from the current command, not the root.
		{[]string{"remote", "--tags"}, `unrecognized option: --tags (defined by subcommand "add")`},

		// Declaration order decides which declarer is named.
		{[]string{"--dry-run"}, `unrecognized option: --dry-run (defined by subcommand "delete")`},

		// A name nobody declares reads as before.
		{[]string{"--nope"}, "unrecognized option: --nope"},
	} {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			_, err := Parse(newApp(), tt.args...)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got, want := humanMessage(err), tt.want; got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
		})
	}
}

// TestUnrecognizedOptionSkipsHiddenSubtree asserts that a hidden
// subcommand's flags are never named in the hint: a hidden command is
// deliberately unadvertised, so the hint must not advertise it either. The
// flag stays usable once its own command is named; only the hint goes
// quiet, falling back to the plain message. A visible sibling is
// unaffected.
func TestUnrecognizedOptionSkipsHiddenSubtree(t *testing.T) {
	newApp := func() *Command {
		hidden := NewCommand("hidden", "").
			Flags(Bool(new(bool), "force", false, "")).
			Hidden()
		visible := NewCommand("visible", "").
			Flags(Bool(new(bool), "tags", false, ""))
		return NewCommand("app", "").Subcommands(hidden, visible)
	}

	t.Run("HiddenSubcommandNoHint", func(t *testing.T) {
		_, err := Parse(newApp(), "--force")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got, want := humanMessage(err), "unrecognized option: --force"; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})

	t.Run("VisibleSiblingStillHints", func(t *testing.T) {
		_, err := Parse(newApp(), "--tags")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		want := `unrecognized option: --tags (defined by subcommand "visible")`
		if got := humanMessage(err); got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})

	t.Run("HiddenFlagStillUsable", func(t *testing.T) {
		inv, err := Parse(newApp(), "hidden", "--force")
		if err != nil {
			t.Fatal(err)
		}
		if got, want := inv.Cmd.String(), "hidden"; got != want {
			t.Errorf("cmd = %q, want %q", got, want)
		}
	})
}

// TestParseOperandNoSlot asserts the two ways an operand can go unbound.
// A name that matches no subcommand is a lookup miss and stays
// "unrecognized"; an operand arriving when no positional flag or
// subcommand can take it at all is not a lookup miss, so it gets its own
// wording, after coreutils' "extra operand".
func TestParseOperandNoSlot(t *testing.T) {
	t.Run("UnrecognizedSubcommand", func(t *testing.T) {
		cmd := NewCommand("app", "").Subcommands(NewCommand("sub", ""))
		_, err := Parse(cmd, "nope")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got, want := humanMessage(err), "unrecognized subcommand: nope"; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})

	t.Run("ExtraOperand", func(t *testing.T) {
		cmd := NewCommand("app", "")
		_, err := Parse(cmd, "nope")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got, want := humanMessage(err), "extra operand: nope"; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})
}

// TestValidateNArgsSpansThePath asserts that the count rules cover every
// flag that became active along the descended path: an ancestor's
// Required flag is still enforced when a subcommand is invoked, and its
// occurrences accumulate wherever they appear around the subcommand
// token.
func TestValidateNArgsSpansThePath(t *testing.T) {
	newApp := func(name *string) *Command {
		return NewCommand("app", "").
			Flags(String(name, "name", "", "").Required()).
			Subcommands(NewCommand("sub", ""))
	}

	_, err := Parse(newApp(new(string)), "sub")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := humanMessage(err), "missing required argument: --name"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}

	// A parent flag keeps working after the subcommand token, and giving
	// it there satisfies the requirement.
	var name string
	if _, err := Parse(newApp(&name), "sub", "--name=x"); err != nil {
		t.Fatal(err)
	}
	assertString(t, "x", name)

	// Occurrences accumulate across the descent, so the ceiling holds
	// over the whole path too.
	_, err = Parse(newApp(new(string)), "--name=x", "sub", "--name=y")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := humanMessage(err), "argument specified too many times: --name"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

// TestTerminatorEndsOptions asserts the default reading of "--": it ends
// option processing, so every argument after it is an operand however many
// dashes it starts with. This is the escape hatch guideline 14 relies on
// for a detached operand, since an argument that parses as an option is
// one.
func TestTerminatorEndsOptions(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want []string
	}{
		{
			"OperandLooksLikeShortOption",
			[]string{"--", "-rf"},
			[]string{"-rf"},
		},
		{
			"OperandLooksLikeLongOption",
			[]string{"--", "--not-a-flag"},
			[]string{"--not-a-flag"},
		},
		{
			"TerminatorIsNotItselfAnOperand",
			[]string{"--", "a"},
			[]string{"a"},
		},
		{
			"OnlyTheFirstTerminatorIsSpecial",
			[]string{"--", "--", "a"},
			[]string{"--", "a"},
		},
		{
			"HelpAfterTerminatorIsAnOperand",
			[]string{"--", "-h"},
			[]string{"-h"},
		},
		{
			"OptionsStillParseBeforeIt",
			[]string{"--flag", "x", "--", "-rf"},
			[]string{"-rf"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var flag string
			var files []string
			cmd := NewCommand("test", "").Flags(
				String(&flag, "flag", "", ""),
				Strings(&files, "file", nil, "").Positional().NArgs(0, 0),
			)
			inv, err := Parse(cmd, tt.args...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertStrings(t, tt.want, files)
			if inv.Interrupt != nil {
				t.Errorf("Interrupt = %v, want none", inv.Interrupt)
			}
			if len(inv.Forwarded) != 0 {
				t.Errorf("Forwarded = %v, want empty", inv.Forwarded)
			}
		})
	}
}

// TestTerminatorSelectsSubcommand asserts that ending option processing
// does not stop an operand resolving as a subcommand, and that options
// stay ended once the parser has descended.
func TestTerminatorSelectsSubcommand(t *testing.T) {
	var files []string
	sub := NewCommand("sub", "").Flags(
		Strings(&files, "file", nil, "").Positional().NArgs(0, 0),
	)
	cmd := NewCommand("test", "").Subcommands(sub)
	inv, err := Parse(cmd, "--", "sub", "-rf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := inv.Cmd.String(), sub.String(); got != want {
		t.Errorf("Cmd = %v, want %v", got, want)
	}
	assertStrings(t, []string{"-rf"}, files)
}

// TestHelpWinsOverAnEarlierArgumentError asserts that a --help anywhere on
// the command line is honored even when an earlier argument was wrong: help
// no longer depends on position, so "app --bogus --help" prints help
// instead of reporting --bogus. Before the lexer split this reported the
// unrecognized option, since parsing stopped at the first error and never
// reached --help; see wip/lexer.md and wip/batch-2026-08-27.md.
func TestHelpWinsOverAnEarlierArgumentError(t *testing.T) {
	cmd := NewCommand("test", "").
		Flags(String(new(string), "name", "", "")).
		HelpFlag()
	inv, err := Parse(cmd, "--bogus", "--help")
	if err != nil {
		t.Fatalf("Parse() = %v, want no error", err)
	}
	if got, want := inv.Interrupt.String(), "--help"; got != want {
		t.Errorf("Interrupt = %v, want %v", got, want)
	}
}

// TestPositionalIsNotAnOption asserts that a positional flag's name no
// longer enters the option table: "--src=x" for a flag declared
// Positional() is an unrecognized option rather than a way to set it. This
// closes item 33 in wip/TODO.md, found while implementing an earlier
// branch and fixed as a consequence of the lexer split, which never adds a
// positional's name to the table it matches options against.
func TestPositionalIsNotAnOption(t *testing.T) {
	var src string
	cmd := NewCommand("test", "").Flags(
		String(&src, "src", "", "").Positional(),
	)
	_, err := Parse(cmd, "--src=x")
	if got, want := humanMessage(err), "unrecognized option: --src"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	assertString(t, "", src)

	// The operand form still binds it.
	if _, err := Parse(cmd, "x"); err != nil {
		t.Fatal(err)
	}
	assertString(t, "x", src)
}

// FuzzParse asserts the parser's contract over arbitrary command lines:
// Parse never panics, and it returns an Invocation or an error, never
// both and never neither. The tree is rebuilt inside the fuzz body
// because repeat parses of one tree are not idempotent, and no flag
// reads the environment, so iterations cannot bleed into each other.
func FuzzParse(f *testing.F) {
	for _, seed := range [][3]string{
		{"--name=x", "sub", "left.txt"},    // attached long value
		{"--name", "x", "--verbose=false"}, // detached value, bool false
		{"-abc", "", ""},                   // grouped shorts, one unknown
		{"-vc5", "-n=x", "-n="},            // grouped shorts, attached values
		{"-", "--", "-"},                   // a lone "-" is an operand
		{"--", "-rf", "--not-a-flag"},      // terminator ends options
		{"", "", ""},                       // empty arguments
		{"--name=héllo", "-é", "日本語"},      // multi-byte arguments
		{"sub", "--sub-name", "left.txt"},  // subcommand path
	} {
		f.Add(seed[0], seed[1], seed[2])
	}
	f.Fuzz(func(t *testing.T, arg1, arg2, arg3 string) {
		var (
			name    string
			verbose bool
			count   int
			tags    []string
			subName string
			files   []string
		)
		sub := NewCommand("sub", "").Flags(
			String(&subName, "sub-name", "", "").Aliases("s"),
			Strings(&files, "file", nil, "").Positional().NArgs(0, 0),
		)
		cmd := NewCommand("fuzz", "").
			Flags(
				String(&name, "name", "", "").Aliases("n"),
				Bool(&verbose, "verbose", false, "").Aliases("v"),
				Int(&count, "count", 0, "").Aliases("c"),
				Strings(&tags, "tag", nil, "").Aliases("t"),
			).
			Subcommands(sub, NewCommand("other", ""))
		inv, err := Parse(cmd, arg1, arg2, arg3)
		if (inv == nil) == (err == nil) {
			t.Fatalf("Parse returned (%v, %v), want exactly one", inv, err)
		}
	})
}

func TestTerminator(t *testing.T) {
	var foo string
	var bar bool
	cmd := NewCommand("test", "").
		Flags(
			String(&foo, "foo", "", ""),
			Bool(&bar, "bar", false, ""),
		).
		ForwardArgs()
	tailArgs := []string{
		"baz",
		"--baz", "--baz=qux", "--baz", "qux",
		"-q", "-q=quux", "-q", "quux",
		"--", "-", "",
	}
	args := append([]string{"--foo=foo", "--bar", "--"}, tailArgs...)
	inv, err := Parse(cmd, args...)
	if err != nil {
		t.Fatal(err)
	}
	assertString(t, "foo", foo)
	assertBool(t, true, bar)
	assertStrings(t, tailArgs, inv.Forwarded)
}

// TestUnrecognizedOptionNamesMountedFlags asserts that the hint reaches a
// flag a subcommand takes from a mounted GroupSet, not just one it
// declares itself. Compiling flattens a command's own groups and its
// mounted ones into one list, so where the flag came from stops mattering
// to the search -- and the hint is right either way, since a mounted flag
// is just as unusable until its own command is named.
func TestUnrecognizedOptionNamesMountedFlags(t *testing.T) {
	set := new(GroupSet)
	set.FlagGroup(NewFlagGroup("shared", "Shared options",
		Bool(new(bool), "force", false, ""),
	))
	sub := NewCommand("delete", "").GroupSets(set)
	app := NewCommand("app", "").Subcommands(sub)

	_, err := Parse(app, "--force")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := humanMessage(err),
		`unrecognized option: --force (defined by subcommand "delete")`; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

// TestNegatedBool asserts the second spelling every boolean answers to.
// "--verbose=false" already set one false, so "--no-verbose" is a
// spelling rather than a capability, and it is generated for every long
// name a boolean has rather than declared per flag; see
// docs/adr/posix-argument-conventions.md.
func TestNegatedBool(t *testing.T) {
	for _, tt := range []struct {
		args    []string
		verbose bool
		loud    bool
		err     bool
	}{
		// The default is true, so a test that reads false proves the
		// negation ran rather than that nothing happened.
		{args: nil, verbose: true, loud: true},
		{args: []string{"--no-verbose"}, verbose: false, loud: true},

		// The value negates with the flag, so both halves apply and
		// "--no-verbose=false" is a double negative.
		{args: []string{"--no-verbose=false"}, verbose: true, loud: true},
		{args: []string{"--no-verbose=true"}, verbose: false, loud: true},

		// Rejected by the flag's own value, identically to "--verbose".
		{args: []string{"--no-verbose=nonsense"}, err: true},

		// Every long name is negated, aliases included, or two spellings
		// of one flag would disagree about what it can be told.
		{args: []string{"--no-loud"}, verbose: true, loud: false},
		{args: []string{"--no-noisy"}, verbose: true, loud: false},

		// A short name is not: "-v=false" is already the short spelling
		// for false, and "-no-v" is not a thing anyone types.
		{args: []string{"--no-v"}, err: true},

		// Only a boolean has an opposite to spell.
		{args: []string{"--no-name=x"}, err: true},

		// The last spelling wins, the same as any repeated flag, though
		// the count check rejects both being given at once today; see
		// wip/TODO.md item 47.
		{args: []string{"--verbose", "--no-verbose"}, err: true},
	} {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var verbose, loud bool
			var name string
			cmd := NewCommand("test", "").Flags(
				Bool(&verbose, "verbose", true, "").Aliases("v"),
				Bool(&loud, "loud", true, "").Aliases("", "noisy"),
				String(&name, "name", "", ""),
			)
			_, err := Parse(cmd, tt.args...)
			if tt.err {
				if err == nil {
					t.Fatalf("expected an error, got verbose=%v loud=%v", verbose, loud)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertBool(t, tt.verbose, verbose)
			assertBool(t, tt.loud, loud)
		})
	}
}

// TestNegatedBoolIsNotAdvertised asserts the cost of generating the
// spelling rather than declaring it: help says nothing about it. The
// convention is documented once, in the package doc and the README,
// rather than repeated beside every boolean a program declares.
func TestNegatedBoolIsNotAdvertised(t *testing.T) {
	var verbose bool
	cmd := NewCommand("test", "").Flags(
		Bool(&verbose, "verbose", false, "be chatty"),
	)
	node, err := cmd.Compile()
	if err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	if err := node.Usage(&buf); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); strings.Contains(got, "--no-verbose") {
		t.Errorf("help mentions --no-verbose:\n%s", got)
	}
}

// TestInterruptFlagForwardsRemainder asserts that the token naming an
// interrupt ends the parse and everything after it arrives on
// Invocation.Forwarded verbatim, however it is spelled: options, a bare
// "--", operands -- none of it is read.
func TestInterruptFlagForwardsRemainder(t *testing.T) {
	cmd := NewCommand("test", "").
		Flags(Interrupt("where", "", func(ctx context.Context, inv *Invocation) error {
			return nil
		}))
	inv, err := Parse(cmd, "--where", "extra", "--", "-x")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Interrupt == nil {
		t.Fatal("Interrupt = nil, want the flag")
	}
	if got, want := strings.Join(inv.Forwarded, " "), "extra -- -x"; got != want {
		t.Errorf("Forwarded = %q, want %q", got, want)
	}
}

// TestInterruptFlagForwardsNothing asserts that an interrupt with nothing
// after it forwards nothing, rather than an empty slice standing in.
func TestInterruptFlagForwardsNothing(t *testing.T) {
	cmd := NewCommand("test", "").
		Flags(Interrupt("where", "", func(ctx context.Context, inv *Invocation) error {
			return nil
		}))
	inv, err := Parse(cmd, "--where")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Forwarded != nil {
		t.Errorf("Forwarded = %v, want nil", inv.Forwarded)
	}
}

// TestInterruptCommandForwardsRemainder asserts the same rule for the
// command tier: naming an interrupt command ends the parse, and the rest
// of the line -- options a parse would reject included -- arrives on
// Invocation.Forwarded for the handler to interpret.
func TestInterruptCommandForwardsRemainder(t *testing.T) {
	cmd := NewCommand("test", "").
		Flags(String(new(string), "name", "", "").Required()).
		Subcommands(VersionCommand("1.0"))
	inv, err := Parse(cmd, "version", "deploy", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := inv.Cmd.FullName, "test version"; got != want {
		t.Fatalf("Cmd = %q, want %q", got, want)
	}
	if got, want := strings.Join(inv.Forwarded, " "), "deploy --json"; got != want {
		t.Errorf("Forwarded = %q, want %q", got, want)
	}
}
