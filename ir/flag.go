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

	// ValueName is how the value the flag takes is written where the flag
	// is shown to a reader: the "SERVICE" of "Usage: deploy SERVICE" and
	// of "missing required argument: SERVICE", and the placeholder beside
	// an option that takes one. Where Forms says what a reader types to
	// name the flag, this says what they type after it -- so a positional
	// argument, which is named by nothing and is only its value, is shown
	// by this alone.
	//
	// Like Forms it arrives already written, so a formatter prints it as
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
// flag all refer to it: its canonical spelling on the command line, or,
// for a positional argument that has no spelling, its value name.
//
// It does not spell a name for itself. How a name is spelled is settled
// when the tree is compiled, so that everything showing a flag shows the
// same thing.
func (f *Flag) String() string {
	for _, form := range f.Forms {
		if form != "" {
			return form
		}
	}
	switch {
	case f.ValueName != "":
		return f.ValueName
	case f.Name != "":
		return f.Name
	default:
		return "unknown"
	}
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
