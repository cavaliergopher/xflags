package ir

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// validateTree implements (*Command).Validate: it checks c and,
// recursively, each of its subcommands, joining every error found.
//
// Every rule here reads a command or a flag on its own terms, so the
// recursion carries nothing down it. The rules that read a spelling --
// whether two flags collide on one, and whether one is reserved for help
// -- live in internal/argv, which owns how a name is spelled.
func validateTree(c *Command) error {
	var errs []error
	if err := validateSelf(c); err != nil {
		errs = append(errs, err)
	}
	for _, sub := range c.Subcommands {
		if err := validateTree(sub); err != nil {
			errs = append(errs, err)
		}
	}
	return JoinErrors(errs)
}

// validateSelf checks c's own flags for configuration errors: flag syntax
// and positional/subcommand conflicts. It does not descend into
// subcommands.
func validateSelf(c *Command) error {
	var errs []error

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
		}
	}
	return JoinErrors(errs)
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
