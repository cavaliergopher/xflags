package xflags

import (
	"flag"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cavaliergopher/xflags/internal/argv"
	"github.com/cavaliergopher/xflags/ir"
)

const (
	defaultMinNArgs = 0
	defaultMaxNArgs = 1
)

// TODO: mutually exclusive flags?
// TODO: error handling modes
// TODO: support aliases
// TODO: support negated bools

// Flag describes a command line flag that may be specified on the command
// line.
//
// The same type also describes a positional operand once marked with
// Positional: every constructor and every chained modifier applies to
// both, and Flag is simply named for the more common case.
//
// Programs should not create Flag directly and instead use one of the typed
// constructors such as String, Int or Var to construct one.
type Flag struct {
	// names are every name the flag answers to, in the slot order Names
	// documents. A slot may be empty, which is how a flag declares an
	// alias without a short name, so the slice is read by index rather
	// than compacted.
	names    []string
	usage    string
	defValue string

	// valueName overrides the name shown for the flag's value; empty
	// leaves the command line conventions to name it. See Flag.ValueName.
	valueName string

	// hasDefault records that a typed constructor captured defValue from a
	// live Value, so Parse may re-apply it. It stays false for Var and for
	// flags imported from a flag.FlagSet, whose defValue is display-only.
	hasDefault bool

	showDefault  bool
	positional   bool
	minCount     int
	maxCount     int
	hidden       bool
	envVar       string
	choices      []string
	validateFunc ir.ValidateFunc
	completeFunc ir.CompleteFunc
	value        ir.Value
}

// Var returns a Flag that can be used to define a command line flag with
// custom value parsing.
//
// name becomes the flag's canonical name, and how it is spelled on the
// command line follows from its shape: one character takes a single dash,
// so Var(v, "n", usage) declares "-n", and anything longer takes two.
// Every constructor in this package is built on Var. Add further names
// with Flag.Aliases.
func Var(value ir.Value, name, usage string) *Flag {
	return &Flag{
		names:    []string{name},
		usage:    usage,
		minCount: defaultMinNArgs,
		maxCount: defaultMaxNArgs,
		value:    value,
	}
}

// stringifyDefault returns the string form of a flag's default value, using
// its Value's String method if it implements fmt.Stringer.
func stringifyDefault(v ir.Value) string {
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	return ""
}

// BitField returns a Flag that can be used to define a uint64 flag
// with specified name, default value, and usage string. The argument p points
// to a uint64 variable in which to toggle each of the bits in the mask
// argument. You can specify multiple BitFieldVars to toggle bits in the same
// underlying uint64.
func BitField(p *uint64, mask uint64, name string, value bool, usage string) *Flag {
	v := newBitFieldValue(value, p, mask)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	c.hasDefault = true
	return c
}

// Bool returns a Flag that can be used to define a bool flag with
// specified name, default value, and usage string. The argument p points to a
// bool variable in which to store the value of the flag.
func Bool(p *bool, name string, value bool, usage string) *Flag {
	v := newBoolValue(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	c.hasDefault = true
	return c
}

// Duration returns a Flag that can be used to define a time.Duration
// flag with specified name, default value, and usage string. The argument p
// points to a time.Duration variable in which to store the value of the flag.
// The flag accepts a value acceptable to time.ParseDuration.
func Duration(p *time.Duration, name string, value time.Duration, usage string) *Flag {
	v := newDurationValue(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	c.hasDefault = true
	return c
}

// Float64 returns a Flag that can be used to define a float64 flag
// with specified name, default value, and usage string. The argument p points
// to a float64 variable in which to store the value of the flag.
func Float64(p *float64, name string, value float64, usage string) *Flag {
	v := newFloat64Value(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	c.hasDefault = true
	return c
}

// Func returns a Flag that can used to define a flag with the specified name and usage
// string.
// Each time the flag is seen, fn is called with the value of the flag.
// If fn returns a non-nil error, it will be treated as a flag value parsing error.
//
// The flag may be given any number of times, since fn accumulates whatever it
// likes; constrain it with NArgs.
func Func(name, usage string, fn func(s string) error) *Flag {
	return Var(funcValue(fn), name, usage).NArgs(0, 0)
}

// Int returns a Flag that can be used to define an int flag with
// specified name, default value, and usage string. The argument p points to an
// int variable in which to store the value of the flag.
func Int(p *int, name string, value int, usage string) *Flag {
	v := newIntValue(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	c.hasDefault = true
	return c
}

// Int64 returns a Flag that can be used to define an int64 flag with
// specified name, default value, and usage string. The argument p points to an
// int64 variable in which to store the value of the flag.
func Int64(p *int64, name string, value int64, usage string) *Flag {
	v := newInt64Value(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	c.hasDefault = true
	return c
}

// String returns a Flag that can be used to define a string flag with
// specified name, default value, and usage string. The argument p points to a
// string variable in which to store the value of the flag.
func String(p *string, name, value, usage string) *Flag {
	v := newStringValue(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	c.hasDefault = true
	return c
}

// Strings returns a Flag that can be used to define a string slice flag with specified name,
// default value, and usage string. The argument p points to a string slice variable in which each
// flag value will be stored in command line order.
func Strings(p *[]string, name string, value []string, usage string) *Flag {
	v := newStringSliceValue(value, p)
	c := Var(v, name, usage).NArgs(0, 0)
	c.defValue = stringifyDefault(v)
	c.hasDefault = true
	return c
}

// Uint returns a Flag that can be used to define an uint flag with
// specified name, default value, and usage string. The argument p points to an
// uint variable in which to store the value of the flag.
func Uint(p *uint, name string, value uint, usage string) *Flag {
	v := newUintValue(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	c.hasDefault = true
	return c
}

// Uint64 returns a Flag that can be used to define an uint64 flag
// with specified name, default value, and usage string. The argument p points
// to an uint64 variable in which to store the value of the flag.
func Uint64(p *uint64, name string, value uint64, usage string) *Flag {
	v := newUint64Value(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	c.hasDefault = true
	return c
}

// String returns how the flag is shown to a reader, which is what it
// compiles to: see (*ir.Flag).String.
func (c *Flag) String() string {
	var errs []error // discarded: Compile is what reports them
	return c.lower(&errs).String()
}

// ShowDefault specifies that the default value of this flag should be shown
// in the help message.
func (c *Flag) ShowDefault() *Flag {
	c.showDefault = true
	return c
}

// Aliases specifies further names the flag answers to, after the one its
// constructor gave. Each is matched on the command line and sets the same
// value, so the flag below answers to "--verbose", "-v" and "--loud"
// alike:
//
//	Bool(&v, "verbose", false, usage).Aliases("v", "loud")
//
// The names carry a convention, which nothing enforces. The first alias is
// the short name, and help prints it beside the constructor's name.
// Anything after is matched but left out of help, which is what a
// compatibility spelling wants. A flag needing one of those but no short
// name leaves the first alias empty:
//
//	String(&c, "colour", "", usage).Aliases("", "color")
//
// A short name is one character from [A-Za-z0-9]. Short names that take no
// value group into a single argument, so "-a -b" may also be written
// "-ab". How each name is spelled follows from its shape rather than its
// position, so a name given out of convention still matches; it is only
// printed elsewhere.
func (c *Flag) Aliases(names ...string) *Flag {
	c.names = append(c.names, names...)
	return c
}

// ValueName names the value the flag takes, which stands in for it
// wherever the flag is shown to a reader: the usage line, the help
// message, and any error naming the flag. A positional argument is shown
// by this alone, having no spelling of its own, and an option shows it
// beside its names.
//
// The flag's own name is used when this is not called, so a flag needs it
// only where that name reads poorly for the value. An option whose only
// name is a single character is shown as VALUE instead, since the letter
// says nothing about what it takes; a positional argument keeps its name
// however short:
//
//	Strings(&tags, "tags", nil, usage).Positional().ValueName("tag")
//
// Give the name alone, undecorated. How it is written for a reader
// follows the command line conventions this package speaks, which write
// "tag" as TAG, the same way they decide a name is spelled "--tag". A
// flag that takes no value, such as a boolean, has nothing to name and
// ignores this.
func (c *Flag) ValueName(name string) *Flag {
	c.valueName = name
	return c
}

// Positional indicates that this flag is a positional argument, and therefore
// has no "-" or "--" delimiter. You cannot specify both positional arguments
// and subcommands.
func (c *Flag) Positional() *Flag {
	c.positional = true
	return c
}

// NArgs indicates how many times this flag may be specified on the command
// line. Value.Set will be called once for each instance of the flag specified
// in the command arguments.
//
// A count of 0 removes the bound, and so means something different at each
// end: a min of 0 is no floor, making the flag optional, while a max of 0 is
// no ceiling, letting it repeat without limit.
//
//	NArgs(0, 1)  optional, at most once -- the default
//	NArgs(1, 1)  exactly once; see Required
//	NArgs(0, 0)  optional, unbounded -- what Strings and Func set
//	NArgs(1, 0)  required, unbounded
func (c *Flag) NArgs(min, max int) *Flag {
	c.minCount = min
	c.maxCount = max
	return c
}

// Required is shorthand for NArgs(1, 1) and indicates that this flag must be
// specified on the command line once and only once.
func (c *Flag) Required() *Flag {
	return c.NArgs(1, 1)
}

// Hidden hides the command line flag from all help messages but still allows
// the flag to be specified on the command line.
func (c *Flag) Hidden() *Flag {
	c.hidden = true
	return c
}

// Env allows the value of the flag to be specified with an environment
// variable if it is not specified on the command line.
func (c *Flag) Env(name string) *Flag {
	c.envVar = name
	return c
}

// Validate specifies a function to validate an argument for this flag before
// it is parsed. If the function returns an error, parsing will fail with the
// same error.
func (c *Flag) Validate(f ir.ValidateFunc) *Flag {
	c.validateFunc = f
	return c
}

// Choices restricts the flag's value to one of elems: any other value
// fails to parse, naming the legal choices.
func (c *Flag) Choices(elems ...string) *Flag {
	c.choices = elems
	return c.Validate(
		func(arg string) error {
			for _, elem := range c.choices {
				if arg == elem {
					return nil
				}
			}
			return fmt.Errorf("expected one of: %s", strings.Join(c.choices, ", "))
		},
	)
}

// Complete registers fn to complete this flag's value for a shell, whether
// the flag is an option or a positional argument. It is consulted only
// when Choices is not declared; Choices, being the enumerable case, always
// wins.
func (c *Flag) Complete(fn ir.CompleteFunc) *Flag {
	c.completeFunc = fn
	return c
}

// lower returns the compiled ir.Flag for c: its data fields copied
// across, its names decorated the way the command line writes them, with
// TakesValue derived from whether its Value is a BoolValue, and its
// behavior -- the Value, the ValidateFunc and the CompleteFunc -- copied
// into the fields only Compile has any business setting.
//
// The rules a name itself must keep are checked here, into errs, because
// this is the last point at which the names are still undecorated. The
// rest of flag configuration is not checked here; see ir.Flag's own
// validation, which Compile runs over the whole lowered tree.
func (c *Flag) lower(errs *[]error) *ir.Flag {
	takesValue := c.positional || !isBoolValue(c.value)
	// How a flag is written down is the command line's question rather
	// than this package's, in both halves of it, so what it declared goes
	// over and the answers come back whole: which options it has and
	// which it only answers to, and what its value is called, including
	// when the answer is none. See ir.Flag.
	namedOptions, claimedOptions := argv.OptionsFor(c.names, c.positional, takesValue)
	valueName := argv.ValueNameFor(canonicalName(c.names), c.valueName, c.positional, takesValue)
	flag := &ir.Flag{
		NamedOptions:   namedOptions,
		ClaimedOptions: claimedOptions,
		ValueName:      valueName,
		Usage:          c.usage,
		Default:        c.defValue,
		ShowDefault:    c.showDefault,
		Positional:     c.positional,
		Hidden:         c.hidden,
		MinCount:       c.minCount,
		MaxCount:       c.maxCount,
		EnvVar:         c.envVar,
		Choices:        slices.Clone(c.choices),
		TakesValue:     takesValue,
		HasDefault:     c.hasDefault,
		Value:          c.value,
		ValidateFunc:   c.validateFunc,
		CompleteFunc:   c.completeFunc,
	}
	c.validateNames(flag, errs)
	return flag
}

// validateNames checks the names c was declared with, undecorated, and
// records what it finds against the lowered flag so an error can name the
// flag the way everything else does.
func (c *Flag) validateNames(flag *ir.Flag, errs *[]error) {
	// A flag with nothing but empty slots can never be named, and has
	// nothing for an error to report it by.
	if canonicalName(c.names) == "" {
		*errs = append(*errs, ir.NewConfigErrorf(nil, nil, flag,
			"flag must declare a name"))
	}
	// What a name may be, and whether a flag of this shape may have more
	// than one, are both the command line's questions; see
	// argv.ValidateNames.
	for _, err := range argv.ValidateNames(c.names, c.positional) {
		*errs = append(*errs, ir.NewConfigErrorf(nil, nil, flag, "%s", err))
	}
}

// canonicalName returns the first of names that is not empty, which is
// the name a flag is known by wherever one name has to stand for it: a
// flag that declares only a short name still has something for its value
// name to be taken from.
func canonicalName(names []string) string {
	for _, name := range names {
		if name != "" {
			return name
		}
	}
	return ""
}

// FlagGroup is a nominal grouping of flags which affects how the flags are
// shown in help messages.
type FlagGroup struct {
	name  string
	title string
	flags []*Flag
}

// NewFlagGroup returns a new FlagGroup with the given name that shows its
// flags under the given title in help messages.
//
// A group built standalone is how a library contributes flags bound to
// variables it owns: mount it on a command with Command.FlagGroups, or
// register it with Register so every command that mounts CommandLine
// picks it up.
func NewFlagGroup(name, title string, flags ...*Flag) *FlagGroup {
	return &FlagGroup{
		name:  name,
		title: title,
		flags: flags,
	}
}

// Flags appends command line flags to the group.
func (c *FlagGroup) Flags(flags ...*Flag) *FlagGroup {
	c.flags = append(c.flags, flags...)
	return c
}

// FromFlagSet returns a FlagGroup holding the flags declared on fs, a flag
// set created with Go's flag package, so a program composed with xflags can
// carry flags from stdlib-flavored libraries. Mount the group with
// Command.FlagGroups, or register it with Register. To import the flags
// declared on the flag package itself, pass flag.CommandLine. Parsing and
// error handling are taken over by this package; boolean flags keep their
// no-argument arity via the flag.Value IsBoolFlag convention.
//
// The flag set is snapshotted when FromFlagSet is called: each flag's
// Value is bound directly, and its name, usage text and DefValue are
// captured for help messages. A flag declared on fs afterwards is not
// seen.
func FromFlagSet(name, title string, fs *flag.FlagSet) *FlagGroup {
	group := NewFlagGroup(name, title)
	fs.VisitAll(func(f *flag.Flag) {
		flg := Var(f.Value, f.Name, f.Usage)
		flg.defValue = f.DefValue
		group.Flags(flg)
	})
	return group
}

// lower returns the compiled ir.FlagGroup for c.
func (c *FlagGroup) lower(mounted bool, errs *[]error) *ir.FlagGroup {
	group := &ir.FlagGroup{
		Name:    c.name,
		Title:   c.title,
		Mounted: mounted,
	}
	for _, flag := range c.flags {
		group.Flags = append(group.Flags, flag.lower(errs))
	}
	return group
}
