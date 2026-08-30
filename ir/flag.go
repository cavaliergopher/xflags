package ir

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
	// NamedOptions is the option each name the program declared is shown
	// as: "--verbose" and "-v" rather than "verbose" and "v". It runs
	// parallel to those names, in the order
	// xflags.Flag.Aliases documents -- the canonical name, the short
	// name, then any further aliases -- so a slot a flag left empty stays
	// empty here rather than closing up, and a help formatter can print
	// the first two knowing what they are.
	//
	// A positional argument has none. It is an operand rather than an
	// option, so nothing names it on the command line and ValueName is
	// how it is shown.
	//
	// Not every option a parser accepts appears here, so this is what is
	// worth showing rather than an exhaustive account of what matches;
	// see ClaimedOptions.
	NamedOptions []string

	// ClaimedOptions is every option the flag answers to on the command
	// line, each mapped to the option it came from: a declared one maps
	// to itself, and one the convention generated maps to the declared
	// option it was generated from. Where NamedOptions is what a reader
	// is shown, this is what a reader may type, and the two differ once
	// an option is generated: a boolean answers to --no-verbose, mapped
	// to --verbose, without that belonging beside the flag's synonyms in
	// help.
	//
	// What a generated option does to the value it binds is the
	// convention's own business and is not recorded here, so a convention
	// with no generated options leaves nothing meaningless behind. Where
	// an option came from is not: every convention either generates
	// options or does not, which is the same thing this field's existence
	// already says.
	//
	// It is the enumerable half of what matches, not the whole of it. A
	// convention may also match by rule -- every casing of an option, or
	// every unambiguous prefix of one -- and no map can hold those, so
	// completion offers from this while the command line still accepts
	// more. Every entry of NamedOptions appears here; Validate checks it,
	// since a convention showing an option it will not accept is a bug in
	// the convention rather than in the program that declared the flag.
	//
	// A positional argument claims none, answering to no option at all.
	ClaimedOptions map[string]string

	// ValueName is how the value the flag takes is written where the flag
	// is shown to a reader: the "SERVICE" of "Usage: deploy SERVICE" and
	// of "missing required argument: SERVICE", and the placeholder beside
	// an option that takes one. Where NamedOptions says what a reader
	// types to name the flag, this says what they type after it -- so a
	// positional argument, which names no option and is only its value,
	// is shown by this alone.
	//
	// Like NamedOptions it arrives already written, so a formatter prints it as
	// given. Both the name it is taken from and the convention it is
	// written by belong to whatever reads the command line, which is what
	// lets one convention print SERVICE where another prints <service>. A
	// flag that takes no value has none.
	ValueName string

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
	// (*xflags.Command).Complete.
	CompleteFunc CompleteFunc `json:"-"`
}

// String returns how the flag is shown wherever one string stands for it,
// which is how the usage line, the help message and every error naming a
// flag all refer to it: the first option it is shown by, or, for a
// positional argument that is shown by no option, its value name. A flag
// that declared no name at all has neither, and is shown as "unknown" so
// that the error saying so still reads as a sentence.
//
// It writes nothing for itself. How an option is written is settled when
// the tree is compiled, so that everything showing a flag shows the same
// thing.
func (f *Flag) String() string {
	for _, option := range f.NamedOptions {
		if option != "" {
			return option
		}
	}
	if f.ValueName != "" {
		return f.ValueName
	}
	return "unknown"
}

// Claims reports whether naming the option name on the command line
// reaches this flag. It answers only for the options that can be listed:
// a convention matching by rule, such as by prefix, accepts more than
// this reports. See ClaimedOptions.
func (f *Flag) Claims(name string) bool {
	_, ok := f.ClaimedOptions[name]
	return ok
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
