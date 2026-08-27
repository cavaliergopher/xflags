package xflags

import (
	"errors"
	"strings"
	"testing"
)

// blipWriter fails exactly once, on its nth Write call, and succeeds on
// every call before and after -- standing in for a transient failure, as
// opposed to a persistently broken writer whose eventual Flush error would
// mask a dropped write in between.
type blipWriter struct {
	n   int
	buf strings.Builder
}

func (w *blipWriter) Write(p []byte) (int, error) {
	if w.n == 0 {
		w.n = -1
		return 0, errors.New("write failed")
	}
	if w.n > 0 {
		w.n--
	}
	return w.buf.Write(p)
}

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

// TestDetailPositionalsReportsBlip asserts that a write failing partway
// through a detail helper is reported even though the tabwriter's rows have
// no usage text, and so no tab character to defer flushing on: each such
// row is flushed straight through on its own Write call, immediately, not
// batched until the trailing Flush -- the very write a persistently broken
// writer's Flush error could otherwise stand in for.
func TestDetailPositionalsReportsBlip(t *testing.T) {
	cmd := NewCommand("test", "").Flags(
		String(new(string), "a", "", "").Positional(),
		String(new(string), "b", "", "").Positional(),
	)
	node, err := cmd.Describe()
	if err != nil {
		t.Fatal(err)
	}
	// The header line succeeds; the first flag's row is where the writer
	// blips.
	w := &blipWriter{n: 1}
	if err := detailPositionals(w, node); err == nil {
		t.Fatal("expected error, got nil")
	}
}
