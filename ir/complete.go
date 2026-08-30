package ir

import (
	"slices"
	"strings"
)

// CompDirective tells a shell how to treat the candidates a CompleteFunc
// returns.
type CompDirective int

const (
	// CompDefault lets the shell fall back to its own filename completion
	// when the candidates given do not satisfy it, such as when there are
	// none.
	CompDefault CompDirective = iota

	// CompNoFileComp tells the shell not to fall back to filename
	// completion: the candidates given, even if there are none, are the
	// whole answer.
	CompNoFileComp
)

// CompleteFunc completes a flag's value for a shell. inv is the invocation
// parsed so far, and word is the fragment under the cursor, which may be
// empty.
//
// inv is given because what completes a value often depends on flags
// already given -- git checkout completing a ref depends on which
// repository -r named, for instance -- and not on the word alone. Flags
// named earlier on the command line are set on inv's command by the time
// CompleteFunc is called; flags named later are not, since completion has
// not read that far.
type CompleteFunc func(inv *Invocation, word string) ([]string, CompDirective)

// complete implements (*Command).Complete.
func complete(c *Command, args []string, word string) ([]string, CompDirective) {
	if err := applyDefaults(c.rootOrSelf()); err != nil {
		return nil, CompNoFileComp
	}
	res := lex(c, args)

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
		return nil, CompDefault
	}

	path := ancestorPath(res.active)
	inv := invocationFor(res.active, path, forwarded, false)

	cands, dir := completeCandidates(res, path, inv, word)
	return finalizeCandidates(cands, word), dir
}

// ancestorPath returns active and every ancestor of it, from the root
// down.
func ancestorPath(active *Command) []*Command {
	var path []*Command
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
func completeCandidates(res lexResult, path []*Command, inv *Invocation, word string) ([]string, CompDirective) {
	if res.awaitingValue != nil {
		return completeValue(res.awaitingValue, inv, word)
	}

	if strings.HasPrefix(word, "-") && !res.optionsEnded {
		if key, frag, ok := strings.Cut(word, "="); ok {
			f := optionTable(path)[key]
			if f == nil {
				return nil, CompNoFileComp
			}
			cands, dir := completeValue(f, inv, frag)
			prefixed := make([]string, len(cands))
			for i, cand := range cands {
				prefixed[i] = key + "=" + cand
			}
			return prefixed, dir
		}
		return flagForms(path), CompNoFileComp
	}

	active := res.active
	if len(active.Subcommands) > 0 {
		return subcommandNames(active), CompNoFileComp
	}
	if res.openPositional != nil {
		return completeValue(res.openPositional, inv, word)
	}
	return nil, CompDefault
}

// completeValue resolves f's own value candidates: its Choices if it
// declared any, since an enumerable list always wins, otherwise its
// CompleteFunc if it declared one, otherwise no candidates with the shell
// left free to fall back to filename completion.
func completeValue(f *Flag, inv *Invocation, word string) ([]string, CompDirective) {
	if len(f.Choices) > 0 {
		return f.Choices, CompNoFileComp
	}
	if f.CompleteFunc != nil {
		return f.CompleteFunc(inv, word)
	}
	return nil, CompDefault
}

// optionTable returns every non-positional flag reachable along path,
// keyed by its long and short spellings -- "--name" and "-s" -- the same
// accumulation lexer.enterCommand builds while lexing, so a key resolves
// here exactly as it would resolve on the command line.
func optionTable(path []*Command) map[string]*Flag {
	table := make(map[string]*Flag)
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
func flagForms(path []*Command) []string {
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
	return append(forms, "--help")
}

// subcommandNames returns the name of every subcommand of active that is
// not Hidden.
func subcommandNames(active *Command) []string {
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
