package xflags

import (
	"testing"

	"github.com/cavaliergopher/xflags/ir"
)

// compRootTree returns a command with a mix of flag kinds for the
// completion engine to exercise: a bare boolean, a hidden boolean, a
// string with Choices, and a string with neither Choices nor a
// CompleteFunc -- what a value falls back to when nothing can complete it.
// It also declares two subcommands, one hidden, and mounts a further flag
// on the visible one so descent can be asserted.
func compRootTree() *Command {
	add := NewCommand("add", "").Flags(
		String(new(string), "tags", "", "").ShortName("t"),
	)
	hiddenSub := NewCommand("hidden-sub", "").Hidden()
	return NewCommand("app", "").
		Flags(
			Bool(new(bool), "verbose", false, "").ShortName("v"),
			Bool(new(bool), "extra", false, "").ShortName("x").Hidden(),
			String(new(string), "env", "", "").ShortName("e").
				Choices("dev", "staging", "prod"),
			String(new(string), "name", "", "").ShortName("n"),
		).
		Subcommands(add, hiddenSub)
}

// compPosSlotsTree returns a command with two positionals, so completion
// after the first is exhausted resumes on the second: BAZ, bounded to
// exactly two, and QUX, unbounded. Each declares its own Choices so a test
// can tell which slot answered.
func compPosSlotsTree() *Command {
	return NewCommand("app", "").Flags(
		Strings(new([]string), "baz", nil, "").Positional().NArgs(2, 2).
			Choices("b1", "b2"),
		Strings(new([]string), "qux", nil, "").Positional().NArgs(0, 0).
			Choices("q1", "q2"),
	)
}

// compOptionsEndedTree returns a command with one positional whose choices
// include a dash-prefixed value, so a test can tell whether a word after
// "--" was read as a flag prefix or as an ordinary operand.
func compOptionsEndedTree() *Command {
	return NewCommand("app", "").Flags(
		String(new(string), "file", "", "").Positional().
			Choices("-rf", "normal"),
	)
}

func TestComplete(t *testing.T) {
	for _, tt := range []struct {
		name  string
		build func() *Command
		args  []string
		word  string
		want  []string
		dir   ir.CompDirective
	}{
		{
			"FlagFormsHiddenExcludedHelpPresent",
			compRootTree, nil, "-",
			[]string{"--env", "--help", "--name", "--verbose", "-e", "-n", "-v"},
			ir.CompNoFileComp,
		},
		{
			"FlagPrefixFilteringHiddenStillExcluded",
			compRootTree, nil, "--e",
			[]string{"--env"},
			ir.CompNoFileComp,
		},
		{
			"SubcommandCompletionHiddenExcluded",
			compRootTree, nil, "",
			[]string{"add"},
			ir.CompNoFileComp,
		},
		{
			"AncestorFlagsStayOfferedAfterDescent",
			compRootTree, []string{"add"}, "-",
			[]string{
				"--env", "--help", "--name", "--tags", "--verbose",
				"-e", "-n", "-t", "-v",
			},
			ir.CompNoFileComp,
		},
		{
			"ChoicesCompleteADetachedValue",
			compRootTree, []string{"--env"}, "",
			[]string{"dev", "prod", "staging"},
			ir.CompNoFileComp,
		},
		{
			"ChoicesCompleteAnAttachedValue",
			compRootTree, nil, "--env=st",
			[]string{"--env=staging"},
			ir.CompNoFileComp,
		},
		{
			"AwaitingValueWithNoCompleterOffersNothing",
			compRootTree, []string{"--name"}, "",
			nil,
			ir.CompDefault,
		},
		{
			"ABrokenLineStillCompletesAfterIt",
			compRootTree, []string{"--bogus"}, "",
			[]string{"add"},
			ir.CompNoFileComp,
		},
		{
			"OptionsEndedReadsADashAsAnOperand",
			compOptionsEndedTree, []string{"--"}, "-",
			[]string{"-rf"},
			ir.CompNoFileComp,
		},
		{
			"FirstPositionalSlot",
			compPosSlotsTree, nil, "",
			[]string{"b1", "b2"},
			ir.CompNoFileComp,
		},
		{
			"SlotAdvancesOnceNArgsIsExhausted",
			compPosSlotsTree, []string{"a", "b"}, "",
			[]string{"q1", "q2"},
			ir.CompNoFileComp,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cands, dir := tt.build().Complete(tt.args, tt.word)
			assertStrings(t, tt.want, cands)
			if got, want := dir, tt.dir; got != want {
				t.Errorf("directive = %v, want %v", got, want)
			}
		})
	}
}

// TestCompleteEmptyArgs asserts that completing an entirely empty command
// line does not panic and, for a command with no subcommands or
// positionals to offer, reports no candidates and lets the shell fall
// back to filename completion.
func TestCompleteEmptyArgs(t *testing.T) {
	cmd := NewCommand("app", "").Flags(
		Bool(new(bool), "verbose", false, "").ShortName("v"),
	)
	cands, dir := cmd.Complete(nil, "")
	assertStrings(t, nil, cands)
	if got, want := dir, ir.CompDefault; got != want {
		t.Errorf("directive = %v, want %v", got, want)
	}
}

// TestCompleteFuncSeesEarlierFlag asserts that a positional's CompleteFunc
// is given an Invocation on which a flag named earlier on the line
// already holds its value -- the git checkout <ref> case wip/lexer.md
// builds the callback signature around.
func TestCompleteFuncSeesEarlierFlag(t *testing.T) {
	var region string
	var gotInv *Invocation
	var sawRegion string
	fn := func(inv *Invocation, word string) ([]string, ir.CompDirective) {
		gotInv = inv
		sawRegion = region
		return []string{"i-2", "i-1"}, ir.CompNoFileComp
	}
	cmd := NewCommand("app", "").Flags(
		String(&region, "region", "", "").ShortName("r"),
		String(new(string), "instance", "", "").Positional().Complete(fn),
	)

	cands, dir := cmd.Complete([]string{"--region", "us-east"}, "i-")

	if gotInv == nil {
		t.Fatal("CompleteFunc was not called")
	}
	if got, want := gotInv.Cmd.String(), cmd.String(); got != want {
		t.Errorf("inv.Cmd = %v, want %v", got, want)
	}
	if got, want := sawRegion, "us-east"; got != want {
		t.Errorf("region seen by CompleteFunc = %q, want %q", got, want)
	}
	assertStrings(t, []string{"i-1", "i-2"}, cands)
	if got, want := dir, ir.CompNoFileComp; got != want {
		t.Errorf("directive = %v, want %v", got, want)
	}
}
