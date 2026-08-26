package xflags

import (
	"strings"
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
// Programs should not create Flag directly and instead use one of the typed
// constructors such as String, Int or Var to construct one.
type Flag struct {
	name        string
	shortName   string
	usage       string
	defValue    string
	showDefault bool
	positional  bool
	minCount    int
	maxCount    int
	hidden      bool
	envVar      string
	validate    ValidateFunc
	value       Value
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
	if c.validate != nil {
		if err := c.validate(s); err != nil {
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
	c.validate = f
	return c
}

// Choices is a convenience method that calls Validate and sets a ValidateFunc
// that enforces that the flag value must be one of the given choices.
func (c *Flag) Choices(elems ...string) *Flag {
	return c.Validate(
		func(arg string) error {
			for _, elem := range elems {
				if arg == elem {
					return nil
				}
			}
			return newArgumentErrorf(nil, nil, c, arg,
				"expected one of: %s",
				strings.Join(elems, ", "),
			)
		},
	)
}

// check verifies that the flag is configured correctly, independent of the
// command it belongs to.
func (c *Flag) check() error {
	if strings.HasPrefix(c.name, "-") {
		return newConfigErrorf(nil, nil, c, "flag name must not start with '-'")
	}
	if c.value == nil {
		return newConfigErrorf(nil, nil, c, "flag must be bound to a value")
	}
	if c.shortName != "" && !isShortName(c.shortName) {
		return newConfigErrorf(
			nil, nil, c,
			"short name must be one character from [A-Za-z0-9]: %q",
			c.shortName,
		)
	}
	if c.minCount < 0 {
		return newConfigErrorf(nil, nil, c,
			"minimum count must not be negative: %d", c.minCount)
	}
	if c.maxCount < 0 {
		return newConfigErrorf(nil, nil, c,
			"maximum count must not be negative: %d", c.maxCount)
	}
	// A max of 0 is unbounded, so it is never exceeded by the min.
	if c.maxCount > 0 && c.minCount > c.maxCount {
		return newConfigErrorf(nil, nil, c,
			"minimum count %d exceeds maximum count %d", c.minCount, c.maxCount)
	}
	return nil
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
	}
}

// FlagGroup is a nominal grouping of flags which affects how the flags are
// shown in help messages.
type FlagGroup struct {
	name  string
	usage string
	flags []*Flag
}

func (c *FlagGroup) describe() *desc.FlagGroup {
	group := &desc.FlagGroup{
		Name:  c.name,
		Usage: c.usage,
	}
	for _, flag := range c.flags {
		group.Flags = append(group.Flags, flag.describe())
	}
	return group
}
