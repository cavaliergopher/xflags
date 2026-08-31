package xflags

import (
	"errors"
	"strings"
	"testing"

	"github.com/cavaliergopher/xflags/ir"
)

// TestRegister asserts that Register returns its argument, so registration
// fits in a var declaration, and that the group lands in CommandLine. The
// group is left registered and holds no flags, so it cannot disturb any
// other test that imports CommandLine.
func TestRegister(t *testing.T) {
	g := NewFlagGroup("inert", "Inert options")
	if got, want := Register(g), g; got != want {
		t.Errorf("Register(g) = %v, want %v", got, want)
	}
	found := false
	for _, got := range CommandLine.groups {
		if got == g {
			found = true
		}
	}
	if !found {
		t.Error("Register(g) did not append g to CommandLine")
	}
}

// TestGroupSetsResolveAtParse asserts that a mounted set is read when the
// tree is parsed rather than when GroupSets is called, so a group
// registered afterwards -- in a later var declaration, or an init function
// -- is still seen.
func TestGroupSetsResolveAtParse(t *testing.T) {
	var s string
	set := new(GroupSet)
	cmd := NewCommand("test", "").GroupSets(set)
	set.FlagGroup(NewFlagGroup("late", "Late options",
		String(&s, "name", "", ""),
	))
	if _, err := Parse(cmd, "--name=value"); err != nil {
		t.Fatal(err)
	}
	assertString(t, "value", s)
}

// TestGroupSetsAreNotWrittenBack asserts that resolving a mounted set
// leaves the command's own groups untouched: a repeated Parse, with a
// Describe between, must not see the mounted flags twice and report them
// as duplicates.
func TestGroupSetsAreNotWrittenBack(t *testing.T) {
	var s string
	set := new(GroupSet)
	set.FlagGroup(NewFlagGroup("lib", "Lib options",
		String(&s, "name", "", ""),
	))
	cmd := NewCommand("test", "").GroupSets(set)
	for i := 1; i <= 2; i++ {
		if _, err := Parse(cmd, "--name=x"); err != nil {
			t.Fatalf("parse %d: %v", i, err)
		}
		if _, err := cmd.Compile(); err != nil {
			t.Fatalf("compile %d: %v", i, err)
		}
	}
}

// assertConfigError asserts that parsing cmd fails with a ConfigError,
// naming the invalid configuration under test in the failure message.
func assertConfigError(t *testing.T, cmd *Command, reason string) bool {
	t.Helper()
	_, err := Parse(cmd)
	if err == nil {
		t.Errorf("expected error for %s, got nil", reason)
		return false
	}
	var cfgErr *ir.ConfigError
	if !errors.As(err, &cfgErr) {
		t.Errorf("expected ConfigError for %s, got %T: %v", reason, err, err)
		return false
	}
	return true
}

// TestGroupSetsDuplicateFlagNames asserts that a name collision involving
// mounted groups is the same configuration error as one between declared
// flags, whether the collision is with the command's own flag or with
// another mounted set.
func TestGroupSetsDuplicateFlagNames(t *testing.T) {
	newSet := func() *GroupSet {
		set := new(GroupSet)
		set.FlagGroup(NewFlagGroup("lib", "Lib options",
			String(new(string), "foo", "", ""),
		))
		return set
	}
	t.Run("WithOwnFlag", func(t *testing.T) {
		assertConfigError(t, NewCommand("test", "").
			Flags(String(new(string), "foo", "", "")).
			GroupSets(newSet()),
			"a mounted flag colliding with a declared flag")
	})
	t.Run("BetweenSets", func(t *testing.T) {
		assertConfigError(t, NewCommand("test", "").
			GroupSets(newSet(), newSet()),
			"the same flag name mounted from two sets")
	})
}

// TestUsageGroupSets asserts that mounted groups appear in help
// output under their own headings, after the command's own groups.
func TestUsageGroupSets(t *testing.T) {
	set := new(GroupSet)
	set.FlagGroup(NewFlagGroup("logging", "Logging options",
		String(new(string), "log-level", "", "Set log verbosity"),
	))
	set.FlagGroup(NewFlagGroup("metrics", "Metrics options",
		String(new(string), "metrics-addr", "", "Metrics listen address"),
	))
	cmd := NewCommand("test", "").
		Flags(Bool(new(bool), "verbose", false, "Print more")).
		GroupSets(set)

	node, err := cmd.Compile()
	if err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	if err := node.Usage(&buf); err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"Usage: test [OPTIONS]",
		"",
		"Options:",
		"   --verbose  Print more",
		"",
		"Logging options:",
		"   --log-level  Set log verbosity",
		"",
		"Metrics options:",
		"   --metrics-addr  Metrics listen address",
		"",
	}, "\n")
	if got := buf.String(); got != want {
		t.Errorf("Usage = %q, want %q", got, want)
	}
}

// TestGroupSetsMountedTwiceInOnePath asserts that a subcommand may mount
// the same set as an ancestor. A command is often mounted somewhere its
// author did not choose, so two teams both reaching CommandLine is
// ordinary rather than a name conflict; only a command's own declarations
// claim a name against its descendants.
func TestGroupSetsMountedTwiceInOnePath(t *testing.T) {
	var level string
	set := new(GroupSet)
	set.FlagGroup(NewFlagGroup("telemetry", "Telemetry options",
		String(&level, "log-level", "info", ""),
	))
	sub := NewCommand("sub", "").GroupSets(set)
	cmd := NewCommand("test", "").GroupSets(set).Subcommands(sub)

	if _, err := Parse(cmd, "sub", "--log-level=debug"); err != nil {
		t.Fatal(err)
	}
	assertString(t, "debug", level)
}
