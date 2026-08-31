package ir

import "testing"

// TestCommandDescribeRecurses asserts that Describe covers a command's
// flag groups and subcommands whole, not just the node it was called on.
func TestCommandDescribeRecurses(t *testing.T) {
	root := &Command{
		Name:     "root",
		FullName: "root",
		FlagGroups: []*FlagGroup{
			{Title: "Options", Flags: []*Flag{{Name: "verbose"}}},
		},
		Subcommands: []*Command{
			{Name: "sub", FullName: "root sub"},
		},
	}

	got := root.Describe()
	if got, want := got.FullName, "root"; got != want {
		t.Errorf("FullName = %q, want %q", got, want)
	}
	if got, want := len(got.FlagGroups), 1; got != want {
		t.Fatalf("len(FlagGroups) = %d, want %d", got, want)
	}
	if got, want := len(got.FlagGroups[0].Flags), 1; got != want {
		t.Fatalf("len(FlagGroups[0].Flags) = %d, want %d", got, want)
	}
	if got, want := got.FlagGroups[0].Flags[0].Name, "verbose"; got != want {
		t.Errorf("FlagGroups[0].Flags[0].Name = %q, want %q", got, want)
	}
	if got, want := len(got.Subcommands), 1; got != want {
		t.Fatalf("len(Subcommands) = %d, want %d", got, want)
	}
	if got, want := got.Subcommands[0].FullName, "root sub"; got != want {
		t.Errorf("Subcommands[0].FullName = %q, want %q", got, want)
	}
}
