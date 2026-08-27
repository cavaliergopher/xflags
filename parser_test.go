package xflags

import (
	"strings"
	"testing"
)

func TestSplitLongOption(t *testing.T) {
	for _, tt := range []struct {
		arg      string
		name     string
		value    string
		attached bool
	}{
		{"--x", "--x", "", false},
		{"--xVar", "--xVar", "", false}, // long options have no remainder form
		{"--x=Var", "--x", "Var", true},
		{"--x=", "--x", "", true},
		{"--foo=bar=baz", "--foo", "bar=baz", true}, // splits at the first "="
		{"--foo=-5", "--foo", "-5", true},
		{"--=foo", "--=foo", "", false}, // names nothing, so it is not a split
	} {
		t.Run(tt.arg, func(t *testing.T) {
			name, value, attached := splitLongOption(tt.arg)
			if name != tt.name || value != tt.value || attached != tt.attached {
				t.Errorf(
					"expected (%q, %q, %v), got (%q, %q, %v)",
					tt.name, tt.value, tt.attached, name, value, attached,
				)
			}
		})
	}
}

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
				Bool(&a, "alpha", false, "").ShortName("a"),
				Bool(&b, "bravo", false, "").ShortName("b"),
				String(&f, "foxtrot", "", "").ShortName("f"),
				String(&operand, "OPERAND", "", "").Positional(),
			)
			_, err := cmd.Parse(tt.args)
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
				Int(&count, "count", 0, "").ShortName("c"),
				Bool(&verbose, "verbose", false, ""),
				String(&name, "name", "", "").ShortName("n"),
			)
			_, err := cmd.Parse(tt.args)
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
			String(&name, "name", "unset", "").ShortName("n"),
		)
		if _, err := cmd.Parse([]string{arg}); err != nil {
			t.Fatal(err)
		}
		return name
	}
	assertString(t, "", parseName("-n="))
	assertString(t, "", parseName("--name="))

	parseVerbose := func(arg string) error {
		cmd := NewCommand("test", "").Flags(
			Bool(new(bool), "verbose", false, "").ShortName("v"),
		)
		_, err := cmd.Parse([]string{arg})
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
			Bool(new(bool), "force", false, "").ShortName("f"),
			Bool(new(bool), "dry-run", false, ""),
		)
		add := NewCommand("add", "").Flags(
			Bool(new(bool), "tags", false, "").ShortName("t"),
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
			_, err := newApp().Parse(tt.args)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got, want := humanMessage(err), tt.want; got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
		})
	}
}

// TestCheckNArgsSpansThePath asserts that the count rules cover every
// flag that became active along the descended path: an ancestor's
// Required flag is still enforced when a subcommand is invoked, and its
// occurrences accumulate wherever they appear around the subcommand
// token.
func TestCheckNArgsSpansThePath(t *testing.T) {
	newApp := func(name *string) *Command {
		return NewCommand("app", "").
			Flags(String(name, "name", "", "").Required()).
			Subcommands(NewCommand("sub", ""))
	}

	_, err := newApp(new(string)).Parse([]string{"sub"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := humanMessage(err), "missing required argument: --name"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}

	// A parent flag keeps working after the subcommand token, and giving
	// it there satisfies the requirement.
	var name string
	if _, err := newApp(&name).Parse([]string{"sub", "--name=x"}); err != nil {
		t.Fatal(err)
	}
	assertString(t, "x", name)

	// Occurrences accumulate across the descent, so the ceiling holds
	// over the whole path too.
	_, err = newApp(new(string)).Parse([]string{"--name=x", "sub", "--name=y"})
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
			inv, err := cmd.Parse(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertStrings(t, tt.want, files)
			if inv.HelpRequested {
				t.Error("HelpRequested = true, want false")
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
	inv, err := cmd.Parse([]string{"--", "sub", "-rf"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := inv.Cmd, sub; got != want {
		t.Errorf("Cmd = %v, want %v", got, want)
	}
	assertStrings(t, []string{"-rf"}, files)
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
			String(&subName, "sub-name", "", "").ShortName("s"),
			Strings(&files, "file", nil, "").Positional().NArgs(0, 0),
		)
		cmd := NewCommand("fuzz", "").
			Flags(
				String(&name, "name", "", "").ShortName("n"),
				Bool(&verbose, "verbose", false, "").ShortName("v"),
				Int(&count, "count", 0, "").ShortName("c"),
				Strings(&tags, "tag", nil, "").ShortName("t"),
			).
			Subcommands(sub, NewCommand("other", ""))
		inv, err := cmd.Parse([]string{arg1, arg2, arg3})
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
	inv, err := cmd.Parse(args)
	if err != nil {
		t.Fatal(err)
	}
	assertString(t, "foo", foo)
	assertBool(t, true, bar)
	assertStrings(t, tailArgs, inv.Forwarded)
}
