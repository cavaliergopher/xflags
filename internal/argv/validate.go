package argv

import (
	"fmt"

	"github.com/cavaliergopher/xflags/ir"
)

// Validate checks the compiled tree rooted at root for the configuration
// errors that are facts about a spelling rather than about the model: two
// flags that would answer to one form, and a flag claiming a form
// reserved for help. It reports every error found in one run.
//
// This is the half of validation that has to ask whatever authority the
// lexer asks. A collision is a collision only once names are spelled --
// two flags collide because their forms are equal, not their names -- so
// checking it anywhere else would be a second opinion on a question this
// package settles. (*ir.Command).Validate covers the rest.
func Validate(root *ir.Command) error {
	return validateTree(root, nil)
}

// validateTree checks c and, recursively, each of its subcommands.
// claimed maps each option spelling declared by c's ancestors to the
// command that declared it: a name may not repeat anywhere along an
// ancestor-descendant chain, and the check runs here because a command
// cannot know its ancestors until the whole tree is in view. See
// docs/adr/path-scoped-flag-names.md.
func validateTree(c *ir.Command, claimed map[string]*ir.Command) error {
	var errs []error
	if err := validateSelf(c, claimed); err != nil {
		errs = append(errs, err)
	}
	if len(c.Subcommands) == 0 {
		return ir.JoinErrors(errs)
	}
	// Descendants see c's names claimed in a copy, so sibling subtrees may
	// still reuse names freely.
	claims := make(map[string]*ir.Command, len(claimed))
	for key, cmd := range claimed {
		claims[key] = cmd
	}
	for _, group := range c.FlagGroups {
		if group.Mounted {
			// A mounted group's flags are the registering library's, not
			// this command's, and a command is often mounted somewhere its
			// author did not choose. Claiming them would make a subcommand
			// that mounts the same set as an ancestor -- the ordinary way
			// two teams both reach CommandLine -- a configuration error.
			continue
		}
		for _, flag := range group.Flags {
			for _, form := range claimedForms(flag) {
				claims[form] = c
			}
		}
	}
	for _, sub := range c.Subcommands {
		if err := validateTree(sub, claims); err != nil {
			errs = append(errs, err)
		}
	}
	return ir.JoinErrors(errs)
}

// validateSelf checks c's own flags for names already declared -- within
// c, or by the ancestors whose claims are passed in -- and for names
// reserved for help. It does not descend into subcommands.
func validateSelf(c *ir.Command, claimed map[string]*ir.Command) error {
	var errs []error

	flagsByName := make(map[string]*ir.Flag)
	for _, group := range c.FlagGroups {
		for _, flag := range group.Flags {
			if err := validateFlagForms(flag); err != nil {
				errs = append(errs, err)
			}
			// A collision is reported by the colliding form rather than by
			// the name behind it: it is a fact about a spelling, so the
			// spelling is what the reader needs.
			for _, key := range claimedForms(flag) {
				if _, ok := flagsByName[key]; ok {
					errs = append(errs, ir.NewConfigErrorf(nil, c, flag, "%s",
						alreadyDeclaredMessage(flag, key)))
				}
				if ancestor, ok := claimed[key]; ok {
					errs = append(errs, ir.NewConfigErrorf(nil, c, flag, "%s",
						alreadyDeclaredByAncestorMessage(flag, key, ancestor.Name)))
				}
				flagsByName[key] = flag
			}
		}
	}
	return ir.JoinErrors(errs)
}

// validateFlagForms reports each of f's names that spells a form the lexer
// answers before it ever consults the option table, which a flag
// declaring one would therefore silently never fire for.
func validateFlagForms(f *ir.Flag) error {
	var errs []error
	for _, name := range f.Names {
		if name == "" {
			continue // an empty slot, which xflags.Flag.Aliases documents
		}
		if form := FormOf(name); form == helpShortForm || form == helpLongForm {
			errs = append(errs, ir.NewConfigErrorf(nil, nil, f,
				"flag name is reserved for help: %s", form))
		}
	}
	return ir.JoinErrors(errs)
}

// alreadyDeclaredMessage reports a name key colliding with one already
// declared on the same command. A positional flag names itself, since key
// is a synthetic "--"/"-" spelling it never appears with on the command
// line; an option is named by that spelling.
func alreadyDeclaredMessage(flag *ir.Flag, key string) string {
	if flag.Positional {
		return fmt.Sprintf("operand already declared: %s", flag)
	}
	return fmt.Sprintf("flag already declared: %s", key)
}

// alreadyDeclaredByAncestorMessage is alreadyDeclaredMessage's counterpart
// for a name an ancestor, named by ancestor, already claimed.
func alreadyDeclaredByAncestorMessage(flag *ir.Flag, key, ancestor string) string {
	if flag.Positional {
		return fmt.Sprintf("operand already declared by ancestor %q: %s", ancestor, flag)
	}
	return fmt.Sprintf("flag already declared by ancestor %q: %s", ancestor, key)
}

// claimedForms returns the spellings f claims in the name space that
// validation checks for collisions. An option claims each of its forms; a
// positional argument has none, so it claims its canonical name spelled as
// an option, since a name means one flag along a command path whether or
// not it is spelled with dashes. See
// docs/adr/path-scoped-flag-names.md.
func claimedForms(f *ir.Flag) []string {
	if f.Positional {
		if f.Name == "" {
			return nil
		}
		return []string{FormOf(f.Name)}
	}
	forms := make([]string, 0, len(f.Forms))
	for _, form := range f.Forms {
		if form != "" {
			forms = append(forms, form)
		}
	}
	return forms
}
