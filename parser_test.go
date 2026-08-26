package xflags

import (
	"strings"
	"testing"
)

func TestSplitOption(t *testing.T) {
	for _, tt := range []struct {
		arg      string
		name     string
		value    string
		attached bool
	}{
		{"-x", "-x", "", false},
		{"-xVar", "-x", "Var", true},
		{"-x=Var", "-x", "Var", true}, // the "=" is a delimiter, not data
		{"-x=", "-x", "", true},       // ... so this is the empty value
		{"-x-5", "-x", "-5", true},    // an attached value may look like an option
		{"--x", "--x", "", false},
		{"--xVar", "--xVar", "", false}, // long options have no remainder form
		{"--x=Var", "--x", "Var", true},
		{"--x=", "--x", "", true},
		{"--foo=bar=baz", "--foo", "bar=baz", true}, // splits at the first "="
		{"--foo=-5", "--foo", "-5", true},
		{"--=foo", "--=foo", "", false}, // names nothing, so it is not a split
	} {
		t.Run(tt.arg, func(t *testing.T) {
			name, value, attached := splitOption(tt.arg)
			if name != tt.name || value != tt.value || attached != tt.attached {
				t.Errorf(
					"expected (%q, %q, %v), got (%q, %q, %v)",
					tt.name, tt.value, tt.attached, name, value, attached,
				)
			}
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

func TestTerminator(t *testing.T) {
	var foo string
	var bar bool
	cmd := NewCommand("test", "").
		Flags(
			String(&foo, "foo", "", ""),
			Bool(&bar, "bar", false, ""),
		).
		WithTerminator()
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
	assertStrings(t, tailArgs, inv.Args)
}
