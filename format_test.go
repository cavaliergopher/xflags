package xflags

import (
	"strings"
	"testing"
)

// TestPrintUsage pins the bytes of the usage line: the command's full name
// assembled from its ancestry, then each clause -- [OPTIONS], COMMAND, and
// one form per positional arity.
func TestPrintUsage(t *testing.T) {
	// leaf inherits [OPTIONS] and its full name from its parent.
	leaf := NewCommand("leaf", "")
	NewCommand("root", "").
		Flags(String(new(string), "foo", "", "")).
		Subcommands(leaf)

	tests := []struct {
		name string
		cmd  *Command
		want string
	}{
		{
			name: "Bare",
			cmd:  NewCommand("test", ""),
			want: "Usage: test\n",
		},
		{
			name: "Options",
			cmd: NewCommand("test", "").
				Flags(String(new(string), "foo", "", "")),
			want: "Usage: test [OPTIONS]\n",
		},
		{
			name: "OptionsInherited",
			cmd:  leaf,
			want: "Usage: root leaf [OPTIONS]\n",
		},
		{
			name: "Subcommands",
			cmd:  NewCommand("test", "").Subcommands(NewCommand("sub", "")),
			want: "Usage: test COMMAND\n",
		},
		{
			name: "OptionalPositional",
			cmd: NewCommand("test", "").
				Flags(String(new(string), "arg", "", "").Positional()),
			want: "Usage: test [ARG]\n",
		},
		{
			name: "OptionalRepeatedPositional",
			cmd: NewCommand("test", "").
				Flags(Strings(new([]string), "arg", nil, "").Positional()),
			want: "Usage: test [ARG...]\n",
		},
		{
			name: "RequiredPositional",
			cmd: NewCommand("test", "").
				Flags(String(new(string), "arg", "", "").Positional().Required()),
			want: "Usage: test ARG\n",
		},
		{
			name: "RequiredRepeatedPositional",
			cmd: NewCommand("test", "").
				Flags(Strings(new([]string), "arg", nil, "").Positional().NArgs(1, 0)),
			want: "Usage: test ARG...\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := tt.cmd.Describe()
			if err != nil {
				t.Fatal(err)
			}
			var sb strings.Builder
			if err := printUsage(&sb, node); err != nil {
				t.Fatal(err)
			}
			if got, want := sb.String(), tt.want; got != want {
				t.Errorf("usage line = %q, want %q", got, want)
			}
		})
	}
}
