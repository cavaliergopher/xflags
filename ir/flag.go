package ir

import (
	"strings"
	"unicode/utf8"
)

// Flag is the compiled, implementation form of a command line flag or
// positional argument, produced by lowering a configuration tree with
// (*xflags.Command).Compile.
//
// Every field marshals except Value, ValidateFunc and CompleteFunc,
// tagged json:"-": they are behavior a formatter, a completion engine or
// any other marshaler has no use for, so they are excluded by tag rather
// than by staying unexported. See the package doc for the two-type model
// this is one half of, and TestMarshalOmitsBehavior for what enforces the
// tags.
type Flag struct {
	// Name is the flag's canonical name, undecorated: the first of Names
	// that is not empty. It is what an error reports the flag by, and what
	// a positional argument is named for.
	Name string

	// Names is every name the flag answers to, in the order
	// xflags.Flag.Aliases documents: the canonical name, the short name,
	// then any further aliases. A slot may be empty, which is how a flag
	// declares an alias but no short name, so the slice is read by index
	// rather than compacted.
	Names []string

	// Forms is how each of Names is spelled on the command line, parallel
	// to it so that an empty slot stays empty: Forms[0] is the canonical
	// spelling and Forms[1] the short one, both of which a help formatter
	// prints, and anything after is an alias it does not. A positional
	// argument has no forms, being named rather than spelled.
	//
	// Not every form a parser accepts need appear here, so this is what is
	// worth showing rather than an exhaustive account of what matches.
	Forms []string

	Usage       string
	Default     string
	ShowDefault bool
	Positional  bool
	Hidden      bool
	MinCount    int
	MaxCount    int
	EnvVar      string
	Choices     []string

	// TakesValue reports whether giving the flag on the command line
	// consumes a value. A boolean flag reports false: naming it alone
	// stands for true, though it still accepts a value attached with "=" to
	// set it false explicitly. Every other flag, including every
	// positional argument, reports true.
	TakesValue bool

	// HasDefault records that Default was captured from a live Value, so
	// it may be re-applied to restore it. See Resetter for the
	// alternative a Value offers when Set cannot restore its default by
	// re-applying it.
	HasDefault bool

	// Value is the flag's bound value: Set writes to it once for each
	// argument the flag is given on the command line, after ValidateFunc,
	// if any, approves the argument.
	Value Value `json:"-"`

	// ValidateFunc, if set, validates an argument before Set writes it to
	// Value.
	ValidateFunc ValidateFunc `json:"-"`

	// CompleteFunc, if set, completes the flag's value for a shell. See
	// Complete.
	CompleteFunc CompleteFunc `json:"-"`
}

// String returns the flag's canonical spelling on the command line: its
// upper-cased name for a positional argument, and otherwise its canonical
// name spelled as an option.
func (f *Flag) String() string {
	switch {
	case f.Name == "":
		return "unknown"
	case f.Positional:
		return strings.ToUpper(f.Name)
	default:
		return FormOf(f.Name)
	}
}

// FormOf returns how a flag name is spelled on the command line: one
// character takes a single dash and anything longer takes two, which is
// POSIX guideline 3 and the GNU long-option convention between them. The
// shape of the name decides, not the slot it was declared in.
//
// This is the one place a name becomes a command line spelling, which is
// what a second argv dialect would replace; see
// docs/adr/posix-gnu-argv-dialect.md.
func FormOf(name string) string {
	if name == "" {
		return ""
	}
	if utf8.RuneCountInString(name) == 1 {
		return "-" + name
	}
	return "--" + name
}

// FormsOf returns the spelling of each name given, parallel to them so
// that an empty slot stays empty.
func FormsOf(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	forms := make([]string, len(names))
	for i, name := range names {
		forms[i] = FormOf(name)
	}
	return forms
}

// CanonicalName returns the first of names that is not empty, which is the
// name a flag is known by wherever one name has to stand for it.
func CanonicalName(names []string) string {
	for _, name := range names {
		if name != "" {
			return name
		}
	}
	return ""
}

// Set validates s with the flag's ValidateFunc, if it has one, and then
// sets its bound Value to s.
func (f *Flag) Set(s string) error {
	if f.ValidateFunc != nil {
		if err := f.ValidateFunc(s); err != nil {
			return err
		}
	}
	return f.Value.Set(s)
}

// FlagGroup is the compiled, implementation form of a nominal grouping of
// flags, which affects how the flags are shown in help messages. Unlike
// Command and Flag, FlagGroup carries no behavior of its own, so every
// field is exported.
type FlagGroup struct {
	Name string

	// Title is the heading a formatter prints above the group's flags.
	Title string

	// Mounted reports that the command took this group from a shared
	// GroupSet rather than declaring it. The flags belong to the library
	// that registered them, so the same group may appear on more than one
	// command in a tree without those commands conflicting.
	Mounted bool

	Flags []*Flag
}
