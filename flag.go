package xflags

import (
	"strings"
	"time"
)

const (
	defaultMinNArgs = 0
	defaultMaxNArgs = 1
)

// Flagger is an interface that describes any type that produces a Flag.
//
// The interface is implemented by both FlagBuilder and Flag so they can often
// be used interchangeably.
type Flagger interface {
	Flag() (*Flag, error)
}

// TODO: mutually exclusive flags?
// TODO: error handling modes
// TODO: support aliases
// TODO: support negated bools

// Flag describes a command line flag that may be specified on the command
// line.
//
// Programs should not create Flag directly and instead use one of the
// FlagBuilders to build one with proper error checking.
type Flag struct {
	Name        string
	ShortName   string
	Usage       string
	ShowDefault bool
	Positional  bool
	MinCount    int
	MaxCount    int
	Hidden      bool
	EnvVar      string
	Validate    ValidateFunc
	Value       Value
}

// Flag implements the Flagger interface.
func (c *Flag) Flag() (*Flag, error) {
	if err := c.Err(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Flag) String() string {
	if c.Positional {
		return strings.ToUpper(c.Name)
	}
	if c.Name != "" {
		return "--" + c.Name
	}
	if c.ShortName != "" {
		return "-" + c.ShortName
	}
	return "unknown"
}

// name returns the name or shortname of the flag in that order of precedence.
func (c *Flag) name() string {
	if c.Name != "" {
		return c.Name
	}
	return c.ShortName
}

// Set sets the value of the command-line flag.
func (c *Flag) Set(s string) error {
	if c.Validate != nil {
		if err := c.Validate(s); err != nil {
			return err
		}
	}
	return c.Value.Set(s)
}

// Err checks the flag configuration and returns the first error it
// encounters.
func (c *Flag) Err() error {
	if strings.HasPrefix(c.Name, "-") {
		return errorf("%s: invalid flag name", c.name())
	}
	if c.Value == nil {
		return errorf("%s: value cannot be nil", c.name())
	}
	if len(c.ShortName) > 1 {
		return errorf(
			"short name must be one character in length: %s",
			c.ShortName,
		)
	}
	if c.MinCount < 0 ||
		c.MaxCount < 0 ||
		(c.MaxCount > 0 && c.MinCount > c.MaxCount) {
		return errorf(
			"%s: invalid NArgs: %d, %d",
			c.name(),
			c.MinCount,
			c.MaxCount,
		)
	}
	return nil
}

// FlagGroup is a nominal grouping of flags which affects how the flags are
// shown in help messages.
type FlagGroup struct {
	Name  string
	Usage string
	Flags []*Flag
}

func (c *FlagGroup) append(flags ...*Flag) {
	c.Flags = append(c.Flags, flags...)
}

// FlagBuilder builds a Flag which defines a command line flag for a CLI
// command.
type FlagBuilder struct {
	flag *Flag
}

// ShowDefault specifies that the default vlaue of this flag should be show in
// the help message.
func (c *FlagBuilder) ShowDefault() *FlagBuilder {
	c.flag.ShowDefault = true
	return c
}

// ShortName specifies an alternative short name for a command line flag. For
// example, a command named "foo" can be specified on the command line with
// "--foo" but may also use a short name of "f" to be specified by "-f".
func (c *FlagBuilder) ShortName(name string) *FlagBuilder {
	c.flag.ShortName = name
	return c
}

// Position indicates that this flag is a positional argument, and therefore has
// no "-" or "--" delimeter. You cannot specify both a positional arguments and
// subcommands.
func (c *FlagBuilder) Positional() *FlagBuilder {
	c.flag.Positional = true
	return c
}

// NArgs indicates how many times this flag may be specified on the command
// line. Value.Set will be called once for each instance of the flag specified
// in the command arguments.
//
// To disable min or max count checking, set their value to 0.
func (c *FlagBuilder) NArgs(min, max int) *FlagBuilder {
	c.flag.MinCount = min
	c.flag.MaxCount = max
	return c
}

// Required is shorthand for NArgs(1, 1) and indicates that this flag must be
// specified on the command line once and only once.
func (c *FlagBuilder) Required() *FlagBuilder {
	return c.NArgs(1, 1)
}

// Hidden hides the command line flag from all help messages but still allows
// the flag to be specified on the command line.
func (c *FlagBuilder) Hidden() *FlagBuilder {
	c.flag.Hidden = true
	return c
}

// Env allows the value of the flag to be specified with an environment variable
// if it is not specified on the command line.
func (c *FlagBuilder) Env(name string) *FlagBuilder {
	c.flag.EnvVar = name
	return c
}

// Validate specifies a function to validate an argument for this flag before
// it is parsed. If the function returns an error, parsing will fail with the
// same error.
func (c *FlagBuilder) Validate(f ValidateFunc) *FlagBuilder {
	c.flag.Validate = f
	return c
}

// Flag checks for any correctness errors in the specification of the command
// line flag and produces a Flag.
func (c *FlagBuilder) Flag() (*Flag, error) {
	return c.flag.Flag()
}

// Must is a helper that calls Build and panics if the error is non-nil.
func (c *FlagBuilder) Must() *Flag {
	flag, err := c.Flag()
	if err != nil {
		panic(err)
	}
	return flag
}

// Var returns a FlagBuilder that can be used to define a command line
// flag with custom value parsing.
func Var(value Value, name, usage string) *FlagBuilder {
	c := &FlagBuilder{
		flag: &Flag{
			Name:     name,
			Usage:    usage,
			MinCount: defaultMinNArgs,
			MaxCount: defaultMaxNArgs,
			Value:    value,
		},
	}
	if len(name) == 1 {
		c.flag.ShortName = c.flag.Name
		c.flag.Name = ""
	}
	return c
}

// BitFieldVar returns a FlagBuilder that can be used to define a uint64 flag
// with specified name, default value, and usage string. The argument p points
// to a uint64 variable in which to toggle each of the bits in the mask
// argument. You can specify multiple BitFieldVars to toggle bits in the same
// underlying uint64.
func BitFieldVar(p *uint64, mask uint64, name string, value bool, usage string) *FlagBuilder {
	return Var(newBitFieldValue(value, p, mask), name, usage)
}

// BoolVar returns a FlagBuilder that can be used to define a bool flag with
// specified name, default value, and usage string. The argument p points to a
// bool variable in which to store the value of the flag.
func BoolVar(p *bool, name string, value bool, usage string) *FlagBuilder {
	return Var(newBoolValue(value, p), name, usage)
}

// Duration returns a FlagBuilder that can be used to define a time.Duration
// flag with specified name, default value, and usage string. The argument p
// points to a time.Duration variable in which to store the value of the flag.
// The flag accepts a value acceptable to time.ParseDuration.
func DurationVar(p *time.Duration, name string, value time.Duration, usage string) *FlagBuilder {
	return Var(newDurationValue(value, p), name, usage)
}

// Float64Var returns a FlagBuilder that can be used to define a float64 flag
// with specified name, default value, and usage string. The argument p points
// to a float64 variable in which to store the value of the flag.
func Float64Var(p *float64, name string, value float64, usage string) *FlagBuilder {
	return Var(newFloat64Value(value, p), name, usage)
}

// FuncVar returns a FlagBuilder that can used to define a flag with the specified name and usage
// string.
// Each time the flag is seen, fn is called with the value of the flag.
// If fn returns a non-nil error, it will be treated as a flag value parsing error.
func FuncVar(name, usage string, fn func(s string) error) *FlagBuilder {
	return Var(funcValue(fn), name, usage)
}

// IntVar returns a FlagBuilder that can be used to define an int flag with
// specified name, default value, and usage string. The argument p points to an
// int variable in which to store the value of the flag.
func IntVar(p *int, name string, value int, usage string) *FlagBuilder {
	return Var(newIntValue(value, p), name, usage)
}

// Int64Var returns a FlagBuilder that can be used to define an int64 flag with
// specified name, default value, and usage string. The argument p points to an
// int64 variable in which to store the value of the flag.
func Int64Var(p *int64, name string, value int64, usage string) *FlagBuilder {
	return Var(newInt64Value(value, p), name, usage)
}

// StringVar returns a FlagBuilder that can be used to define a string flag with
// specified name, default value, and usage string. The argument p points to a
// string variable in which to store the value of the flag.
func StringVar(p *string, name, value, usage string) *FlagBuilder {
	return Var(newStringValue(value, p), name, usage)
}

// StringSliceVar returns a FlagBuilder that can be used to define a string
// slice flag with specified name, default value, and usage string. The argument
// p points to a string slice variable in which each flag value will be stored
// in command line order.
func StringSliceVar(p *[]string, name string, value []string, usage string) *FlagBuilder {
	return Var(newStringSliceValue(value, p), name, usage).NArgs(0, 0)
}

// UintVar returns a FlagBuilder that can be used to define an uint flag with
// specified name, default value, and usage string. The argument p points to an
// uint variable in which to store the value of the flag.
func UintVar(p *uint, name string, value uint, usage string) *FlagBuilder {
	return Var(newUintValue(value, p), name, usage)
}

// Uint64Var returns a FlagBuilder that can be used to define an uint64 flag
// with specified name, default value, and usage string. The argument p points
// to an uint64 variable in which to store the value of the flag.
func Uint64Var(p *uint64, name string, value uint64, usage string) *FlagBuilder {
	return Var(newUint64Value(value, p), name, usage)
}
