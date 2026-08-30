package ir

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// validateTree implements (*Command).Validate: it checks c and,
// recursively, each of its subcommands, joining every error found. claimed
// maps each option spelling declared by c's ancestors to the command that
// declared it: a name may not repeat anywhere along an ancestor-descendant
// chain, and the check runs here because a command cannot know its
// ancestors until the whole tree is in view. See
// docs/adr/path-scoped-flag-names.md.
func validateTree(c *Command, claimed map[string]*Command) error {
	var errs []error
	if err := validateSelf(c, claimed); err != nil {
		errs = append(errs, err)
	}
	if len(c.Subcommands) == 0 {
		return JoinErrors(errs)
	}
	// Descendants see c's names claimed in a copy, so sibling subtrees may
	// still reuse names freely.
	claims := make(map[string]*Command, len(claimed))
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
	return JoinErrors(errs)
}

// validateSelf checks c's own flags for configuration errors: flag syntax,
// names already declared -- within c, or by the ancestors whose claims are
// passed in -- and positional/subcommand conflicts. It does not descend
// into subcommands.
func validateSelf(c *Command, claimed map[string]*Command) error {
	var errs []error

	flagsByName := make(map[string]*Flag)
	hasUnboundedPositional := false
	for _, group := range c.FlagGroups {
		for _, flag := range group.Flags {
			if err := validateFlag(flag); err != nil {
				errs = append(errs, err)
			}
			if flag.Positional {
				if len(c.Subcommands) > 0 {
					errs = append(errs, newConfigErrorf(nil, c, flag, "cannot specify both subcommands and positional arguments"))
				}
				if hasUnboundedPositional {
					errs = append(errs, newConfigErrorf(nil, c, flag, "positional arguments cannot follow unbounded positional arguments"))
				}
				if flag.MaxCount == 0 {
					hasUnboundedPositional = true
				}
			}
			// A collision is reported by the colliding form rather than by
			// the name behind it: it is a fact about a spelling, so the
			// spelling is what the reader needs.
			for _, key := range claimedForms(flag) {
				if _, ok := flagsByName[key]; ok {
					errs = append(errs, newConfigErrorf(nil, c, flag, "%s",
						alreadyDeclaredMessage(flag, key)))
				}
				if ancestor, ok := claimed[key]; ok {
					errs = append(errs, newConfigErrorf(nil, c, flag, "%s",
						alreadyDeclaredByAncestorMessage(flag, key, ancestor.Name)))
				}
				flagsByName[key] = flag
			}
		}
	}
	return JoinErrors(errs)
}

// alreadyDeclaredMessage reports a name key colliding with one already
// declared on the same command. A positional flag names itself, since key
// is a synthetic "--"/"-" spelling it never appears with on the command
// line; an option is named by that spelling.
func alreadyDeclaredMessage(flag *Flag, key string) string {
	if flag.Positional {
		return fmt.Sprintf("operand already declared: %s", flag)
	}
	return fmt.Sprintf("flag already declared: %s", key)
}

// alreadyDeclaredByAncestorMessage is alreadyDeclaredMessage's counterpart
// for a name an ancestor, named by ancestor, already claimed.
func alreadyDeclaredByAncestorMessage(flag *Flag, key, ancestor string) string {
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
func claimedForms(f *Flag) []string {
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

// validateFlag implements the flag half of (*Command).Validate: it verifies
// that f is configured correctly, independent of the command it belongs
// to, reporting every rule it breaks.
func validateFlag(f *Flag) error {
	var errs []error
	fail := func(format string, a ...any) {
		errs = append(errs, newConfigErrorf(nil, nil, f, format, a...))
	}
	// A flag with nothing but empty slots can never be named, and has no
	// canonical name for an error to report it by.
	if f.Name == "" {
		fail("flag must declare a name")
	}
	// A positional argument is named rather than spelled, so an alias
	// would be a name nothing could ever match: it never enters the option
	// table. Reported rather than ignored, so the mistake is not silent.
	if f.Positional && len(f.Names) > 1 {
		fail("positional arguments do not support aliases")
	}
	for _, name := range f.Names {
		if name == "" {
			continue // an empty slot, which xflags.Flag.Aliases documents
		}
		if strings.HasPrefix(name, "-") {
			fail("flag name must not start with '-': %q", name)
		}
		// "=" reads as the delimiter of an attached value and whitespace
		// as an argument break, so a name containing either can never be
		// matched.
		if strings.ContainsRune(name, '=') {
			fail("flag name must not contain '=': %q", name)
		}
		if strings.ContainsFunc(name, unicode.IsSpace) {
			fail("flag name must not contain whitespace: %q", name)
		}
		// A one-character name is spelled with a single dash, and POSIX
		// guideline 3 confines that to one alphanumeric character.
		if utf8.RuneCountInString(name) == 1 && !isShortName(name) {
			fail("short name must be one character from [A-Za-z0-9]: %q", name)
		}
		// The lexer matches -h and --help before the option table, so a
		// flag claiming either spelling would silently never fire.
		if form := FormOf(name); form == "-h" || form == "--help" {
			fail("flag name is reserved for help: %s", form)
		}
	}
	if f.Value == nil {
		fail("flag must be bound to a value")
	}
	if f.MinCount < 0 {
		fail("minimum count must not be negative: %d", f.MinCount)
	}
	if f.MaxCount < 0 {
		fail("maximum count must not be negative: %d", f.MaxCount)
	}
	// A max of 0 is unbounded, so it is never exceeded by the min.
	if f.MaxCount > 0 && f.MinCount > f.MaxCount {
		fail("minimum count %d exceeds maximum count %d", f.MinCount, f.MaxCount)
	}
	// Defaults are applied without going through Set, so a default outside
	// the choices is never rejected at parse time: the flag would hold, and
	// help would advertise, a value the same program refuses to accept on
	// the command line.
	//
	// Only a flag that takes one value is checked. A repeatable flag
	// accumulates, so its default renders as the whole collection -- "[]"
	// for an empty one -- which is not a value any single choice could
	// match. An empty default is left alone either way: it is how a flag
	// says it has no default.
	if len(f.Choices) > 0 && f.MaxCount == 1 && f.Default != "" &&
		!slices.Contains(f.Choices, f.Default) {
		fail("default %q is not one of: %s", f.Default, strings.Join(f.Choices, ", "))
	}
	return JoinErrors(errs)
}

// isShortName reports whether s is a legal short name. POSIX guideline 3
// confines one to a single character from the portable character set, and
// the parser leans on that: reading "=" as a delimiter after a boolean
// short flag costs no ambiguity only because "=" can never be a name.
//
// Measured in characters rather than bytes, so a multi-byte rune is
// rejected for falling outside the set rather than for its length.
func isShortName(s string) bool {
	r, size := utf8.DecodeRuneInString(s)
	if size != len(s) {
		return false
	}
	return ('a' <= r && r <= 'z') ||
		('A' <= r && r <= 'Z') ||
		('0' <= r && r <= '9')
}
