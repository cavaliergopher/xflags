package xflags

import (
	"strings"
	"time"
)

const (
	defaultMinNArgs = 0
	defaultMaxNArgs = 1
)

var flagHelp bool

var helpFlag = BoolVar(&flagHelp, "help", false, "Show help info").
	ShortName("h").
	Hidden().
	Must()

// TODO: Groups?
// TODO: mutually exclusive flags?
// TODO: custom validation errors?
// TODO: error handling modes

// FlagInfo describes a command line flag that may be specified on the command
// line.
//
// Programs should not create FlagInfo directly and instead use one of the
// FlagBuilders to build one with proper error checking.
type FlagInfo struct {
	Name        string
	ShortName   string
	Usage       string
	ShowDefault bool
	Positional  bool
	MinCount    int
	MaxCount    int
	Hidden      bool
	EnvVar      string
	Value       Value
}

func (c *FlagInfo) String() string {
	if c.Positional {
		return strings.ToUpper(c.Name)
	}
	return "--" + c.Name
}

// FlagBuilder builds a FlagInfo which defines a command line flag for a CLI
// command.
type FlagBuilder struct {
	info *FlagInfo
	err  error
}

func (c *FlagBuilder) setErr(err error) {
	if c.err != nil {
		return
	}
	c.err = err
}

// ShowDefault specifies that the default vlaue of this flag should be show in
// the help message.
func (c *FlagBuilder) ShowDefault() *FlagBuilder {
	c.info.ShowDefault = true
	return c
}

// ShortName specifies an alternative short name for a command line flag. For
// example, a command named "foo" can be specified on the command line with
// "--foo" but may also use a short name of "f" to be specified by "-f".
func (c *FlagBuilder) ShortName(name string) *FlagBuilder {
	if len(name) > 1 {
		c.setErr(newArgError(1, "shortname must be one character in length: %s", name))
		return c
	}
	c.info.ShortName = name
	return c
}

// Position indicates that this flag is a positional argument, and therefore has
// no "-" or "--" delimeter. You cannot specify both a positional arguments and
// subcommands.
func (c *FlagBuilder) Positional() *FlagBuilder {
	c.info.Positional = true
	return c
}

// NArgs indicates how many times this flag may be specified on the command
// line. Value.Set will be called once for each instance of the flag specified
// in the command arguments.
//
// To disable min or max count checking, set their value to 0.
func (c *FlagBuilder) NArgs(min, max int) *FlagBuilder {
	if min < 0 || max < 0 || (max > 0 && min > max) {
		c.setErr(newArgError(1, "invalid NArgs: %d, %d", min, max))
		return c
	}
	c.info.MinCount = min
	c.info.MaxCount = max
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
	c.info.Hidden = true
	return c
}

// Env allows the value of the flag to be specified with an environment variable
// if it is not specified on the command line.
func (c *FlagBuilder) Env(name string) *FlagBuilder {
	c.info.EnvVar = name
	return c
}

// Build checks for any correctness errors in the specification of the command
// line flag and produces a FlagInfo.
func (c *FlagBuilder) Build() (*FlagInfo, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.info, nil
}

// Must is a helper that calls Build and panics if the error is non-nil. It is
// intended only for use in variable initializations.
func (c *FlagBuilder) Must() *FlagInfo {
	info, err := c.Build()
	if err != nil {
		panic(err)
	}
	return info
}

// Var returns a FlagBuilder that can be used to define a command line
// flag with custom value parsing.
func Var(value Value, name, usage string) *FlagBuilder {
	c := &FlagBuilder{
		info: &FlagInfo{
			Name:     name,
			Usage:    usage,
			MinCount: defaultMinNArgs,
			MaxCount: defaultMaxNArgs,
			Value:    value,
		},
	}
	if value == nil {
		c.setErr(newArgError(1, "value interface cannot be nil"))
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
