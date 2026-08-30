package argv

import (
	"slices"
	"strings"

	"github.com/cavaliergopher/xflags/ir"
)

// Complete resolves shell completion candidates for a command line that is
// still being typed against the compiled command cmd. args is the command
// line so far, excluding the program name and the word currently under the
// cursor; word is that fragment, possibly empty.
//
// Complete is best-effort: a broken or half-typed command line still
// yields whatever can be offered for the position the cursor is in --
// required flags need not be present, and an unrecognized token earlier on
// the line does not stop completion at the position after it. It calls
// Set on every flag named earlier in args, since a value's CompleteFunc
// may need to see them, so it must run in a throwaway process built for
// completion and never beside a program's live state.
func Complete(cmd *ir.Command, args []string, word string) ([]string, ir.CompDirective) {
	if err := applyDefaults(rootOf(cmd)); err != nil {
		return nil, ir.CompNoFileComp
	}
	res := lex(cmd, args)

	var forwarded []string
	var forwardedArgs bool
	for _, instr := range res.instructions {
		switch instr.kind {
		case instSet:
			// Best-effort: a value a CompleteFunc would reject outright is
			// still worth setting, since completion is not validation.
			_ = instr.flag.Set(instr.value)
		case instForward:
			forwarded = instr.forwarded
			forwardedArgs = true
		}
	}
	if forwardedArgs {
		// Once argv has crossed a ForwardArgs terminator, the rest belongs
		// to whatever the command forwards to, not to this tree -- nothing
		// here can say what completes it.
		return nil, ir.CompDefault
	}

	path := ancestorPath(res.active)
	inv := invocationFor(res.active, path, forwarded, false)

	cands, dir := completeCandidates(res, path, inv, word)
	return finalizeCandidates(cands, word), dir
}

// ancestorPath returns active and every ancestor of it, from the root
// down.
func ancestorPath(active *ir.Command) []*ir.Command {
	var path []*ir.Command
	for c := active; c != nil; c = c.Parent {
		path = append(path, c)
	}
	slices.Reverse(path)
	return path
}

// completeCandidates chooses which of the four candidate rules applies at
// the cursor and returns its result, unfiltered; Complete filters,
// deduplicates and sorts every rule's output the same way, so no rule does
// it for itself.
func completeCandidates(res lexResult, path []*ir.Command, inv *ir.Invocation, word string) ([]string, ir.CompDirective) {
	if res.awaitingValue != nil {
		return completeValue(res.awaitingValue, inv, word)
	}

	if strings.HasPrefix(word, "-") && !res.optionsEnded {
		if key, frag, ok := strings.Cut(word, "="); ok {
			f := optionTable(path)[key]
			if f == nil {
				return nil, ir.CompNoFileComp
			}
			cands, dir := completeValue(f, inv, frag)
			prefixed := make([]string, len(cands))
			for i, cand := range cands {
				prefixed[i] = key + "=" + cand
			}
			return prefixed, dir
		}
		return flagForms(path), ir.CompNoFileComp
	}

	active := res.active
	if len(active.Subcommands) > 0 {
		return subcommandNames(active), ir.CompNoFileComp
	}
	if res.openPositional != nil {
		return completeValue(res.openPositional, inv, word)
	}
	return nil, ir.CompDefault
}

// completeValue resolves f's own value candidates: its Choices if it
// declared any, since an enumerable list always wins, otherwise its
// CompleteFunc if it declared one, otherwise no candidates with the shell
// left free to fall back to filename completion.
func completeValue(f *ir.Flag, inv *ir.Invocation, word string) ([]string, ir.CompDirective) {
	if len(f.Choices) > 0 {
		return f.Choices, ir.CompNoFileComp
	}
	if f.CompleteFunc != nil {
		return f.CompleteFunc(inv, word)
	}
	return nil, ir.CompDefault
}

// optionTable returns every non-positional flag reachable along path,
// keyed by its long and short spellings -- "--name" and "-s" -- the same
// accumulation lexer.enterCommand builds while lexing, so a key resolves
// here exactly as it would resolve on the command line.
func optionTable(path []*ir.Command) map[string]*ir.Flag {
	table := make(map[string]*ir.Flag)
	for _, cmd := range path {
		for _, group := range cmd.FlagGroups {
			for _, f := range group.Flags {
				if f.Positional {
					continue
				}
				for _, form := range f.Forms {
					if form == "" {
						continue
					}
					table[form] = f
				}
			}
		}
	}
	return table
}

// flagForms returns every form -- "--name" and "-s" -- of every flag along
// path that is neither positional nor Hidden, plus "--help", which is
// never declared as an ordinary flag but is always legal.
func flagForms(path []*ir.Command) []string {
	var forms []string
	for _, cmd := range path {
		for _, group := range cmd.FlagGroups {
			for _, f := range group.Flags {
				if f.Positional || f.Hidden {
					continue
				}
				for _, form := range f.Forms {
					if form == "" {
						continue
					}
					forms = append(forms, form)
				}
			}
		}
	}
	return append(forms, helpLongForm)
}

// subcommandNames returns the name of every subcommand of active that is
// not Hidden.
func subcommandNames(active *ir.Command) []string {
	var names []string
	for _, sub := range active.Subcommands {
		if sub.Hidden {
			continue
		}
		names = append(names, sub.Name)
	}
	return names
}

// finalizeCandidates applies the one rule every candidate list shares:
// keep what has word as a prefix, drop duplicates, and sort what remains,
// so a shell always sees a stable, minimal reply regardless of which rule
// produced it.
func finalizeCandidates(cands []string, word string) []string {
	seen := make(map[string]bool, len(cands))
	var out []string
	for _, cand := range cands {
		if !strings.HasPrefix(cand, word) || seen[cand] {
			continue
		}
		seen[cand] = true
		out = append(out, cand)
	}
	slices.Sort(out)
	return out
}
