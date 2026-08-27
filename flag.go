package xflags

import (
	"flag"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/cavaliergopher/xflags/desc"
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
	name      string
	shortName string
	usage     string
	defValue  string

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
	validateFunc ValidateFunc
	value        Value
}

// Var returns a Flag that can be used to define a command line flag with
// custom value parsing.
//
// A one-character name becomes a short name rather than a long one, so
// Var(v, "n", usage) declares "-n" and not "--n". Every constructor in this
// package is built on Var and shares the rule. Declare both spellings by
// giving the long one here and the short one to Flag.ShortName.
func Var(value Value, name, usage string) *Flag {
	c := &Flag{
		name:     name,
		usage:    usage,
		minCount: defaultMinNArgs,
		maxCount: defaultMaxNArgs,
		value:    value,
	}
	if len(name) == 1 {
		// A single character is a short name: "-n", never "--n".
		c.shortName = c.name
		c.name = ""
	}
	return c
}

// stringifyDefault returns the string form of a flag's default value, using
// its Value's String method if it implements fmt.Stringer.
func stringifyDefault(v Value) string {
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

func (c *Flag) String() string {
	if c.positional {
		return strings.ToUpper(c.name)
	}
	if c.name != "" {
		return "--" + c.name
	}
	if c.shortName != "" {
		return "-" + c.shortName
	}
	return "unknown"
}

// keyName returns the name or short name of the flag in that order of
// precedence.
func (c *Flag) keyName() string {
	if c.name != "" {
		return c.name
	}
	return c.shortName
}

// Set sets the value of the command-line flag.
func (c *Flag) Set(s string) error {
	if c.validateFunc != nil {
		if err := c.validateFunc(s); err != nil {
			return err
		}
	}
	return c.value.Set(s)
}

// ShowDefault specifies that the default value of this flag should be shown
// in the help message.
func (c *Flag) ShowDefault() *Flag {
	c.showDefault = true
	return c
}

// ShortName specifies an alternative short name for a command line flag. For
// example, a command named "foo" can be specified on the command line with
// "--foo" but may also use a short name of "f" to be specified by "-f".
//
// A short name is one character from [A-Za-z0-9]. Short names that take no
// value group into a single argument, so "-a -b" may also be written "-ab".
func (c *Flag) ShortName(name string) *Flag {
	c.shortName = name
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
func (c *Flag) Validate(f ValidateFunc) *Flag {
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
			return newArgumentErrorf(nil, nil, c, arg,
				"expected one of: %s",
				strings.Join(c.choices, ", "),
			)
		},
	)
}

// validate verifies that the flag is configured correctly, independent of
// the command it belongs to, reporting every rule it breaks.
func (c *Flag) validate() error {
	var errs []error
	fail := func(format string, a ...any) {
		errs = append(errs, newConfigErrorf(nil, nil, c, format, a...))
	}
	if strings.HasPrefix(c.name, "-") {
		fail("flag name must not start with '-'")
	}
	// "=" reads as the delimiter of an attached value and whitespace as an
	// argument break, so a name containing either can never be matched.
	if strings.ContainsRune(c.name, '=') {
		fail("flag name must not contain '='")
	}
	if strings.ContainsFunc(c.name, unicode.IsSpace) {
		fail("flag name must not contain whitespace")
	}
	// The parser matches -h and --help before the flag table, so a flag
	// claiming either spelling would silently never fire.
	if c.name == "help" {
		fail("flag name is reserved for help: --help")
	}
	if c.value == nil {
		fail("flag must be bound to a value")
	}
	if c.shortName != "" && !isShortName(c.shortName) {
		fail("short name must be one character from [A-Za-z0-9]: %q", c.shortName)
	}
	if c.shortName == "h" {
		fail("short name is reserved for help: -h")
	}
	if c.minCount < 0 {
		fail("minimum count must not be negative: %d", c.minCount)
	}
	if c.maxCount < 0 {
		fail("maximum count must not be negative: %d", c.maxCount)
	}
	// A max of 0 is unbounded, so it is never exceeded by the min.
	if c.maxCount > 0 && c.minCount > c.maxCount {
		fail("minimum count %d exceeds maximum count %d", c.minCount, c.maxCount)
	}
	return joinErrors(errs)
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

func (c *Flag) describe() *desc.Flag {
	return &desc.Flag{
		Name:        c.name,
		ShortName:   c.shortName,
		Usage:       c.usage,
		Default:     c.defValue,
		ShowDefault: c.showDefault,
		Positional:  c.positional,
		Hidden:      c.hidden,
		MinCount:    c.minCount,
		MaxCount:    c.maxCount,
		EnvVar:      c.envVar,
		Choices:     slices.Clone(c.choices),
	}
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

func (c *FlagGroup) describe() *desc.FlagGroup {
	group := &desc.FlagGroup{
		Name:  c.name,
		Title: c.title,
	}
	for _, flag := range c.flags {
		group.Flags = append(group.Flags, flag.describe())
	}
	return group
}
