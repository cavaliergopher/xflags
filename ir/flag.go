package ir

import "strings"

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
	Name        string
	ShortName   string
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
// upper-cased name for a positional argument, otherwise its long option
// spelling if it has one and its short option spelling otherwise.
func (f *Flag) String() string {
	if f.Positional {
		return strings.ToUpper(f.Name)
	}
	if f.Name != "" {
		return "--" + f.Name
	}
	if f.ShortName != "" {
		return "-" + f.ShortName
	}
	return "unknown"
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
