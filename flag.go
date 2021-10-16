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
// TODO: show default value in help message?

// FlagInfo describes a command line flag that may be specified on the command
// line.
//
// Programs should not create FlagInfo directly and instead use one of the
// FlagBuilders to build one with proper error checking.
type FlagInfo struct {
	Name       string
	ShortName  string
	Usage      string
	Positional bool
	MinCount   int
	MaxCount   int
	Hidden     bool
	EnvVar     string
	Value      Value
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

// ShortName specifies an alternative short name for a command line flag. For
// example, a command named "foo" can be specified on the command line with
// "--foo" but may also use a short name of "f" to be specified by "-f".
func (c *FlagBuilder) ShortName(name string) *FlagBuilder {
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

// Must calls Build and panics if any error is encountered. This should only be
// used in a global variables or init function.
func (c *FlagBuilder) Must() *FlagInfo {
	info, err := c.Build()
	if err != nil {
		panic(err)
	}
	return info
}

// Var returns a FlagBuilder which can be used to define a command line
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

// BitFieldVar returns a FlagBuilder which can be used to define a command line
// flag that sets a bit in a uint64 value.
//
// A bit field flag does not require a value to be specified on the command line
// and instead stores "true" if the flag appears in the command line arguments.
func BitFieldVar(p *uint64, n int, name string, value bool, usage string) *FlagBuilder {
	fb := Var(newBitFieldValue(value, p, n), name, usage)
	if n > 64 {
		fb.setErr(newArgError(1, "bit field index must be 64 or less, got: %v", n))
	}
	return fb
}

// BoolVar returns a FlagBuilder which can be used to define a command line
// flag with a bool value.
//
// A bool flag does not require a value to be specified on the command line and
// instead stores "true" if the flag appears in the command line arguments.
func BoolVar(p *bool, name string, value bool, usage string) *FlagBuilder {
	return Var(newBoolValue(value, p), name, usage)
}

// Duration returns a FlagBuilder which can be used to define a command line
// flag with a string value.
func DurationVar(p *time.Duration, name string, value time.Duration, usage string) *FlagBuilder {
	return Var(newDurationValue(value, p), name, usage)
}

// Float64 returns a FlagBuilder which can be used to define a command line
// flag with a Float64 value.
func Float64Var(p *float64, name string, value float64, usage string) *FlagBuilder {
	return Var(newFloat64Value(value, p), name, usage)
}

// Int64 returns a FlagBuilder which can be used to define a command line
// flag with an int64 value.
func Int64Var(p *int64, name string, value int64, usage string) *FlagBuilder {
	return Var(newInt64Value(value, p), name, usage)
}

// String returns a FlagBuilder which can be used to define a command line
// flag with a string value.
func StringVar(p *string, name, value, usage string) *FlagBuilder {
	return Var(newStringValue(value, p), name, usage)
}

// String returns a FlagBuilder which can be used to define a command line
// flag with a string slice value.
func StringSliceVar(p *[]string, name string, value []string, usage string) *FlagBuilder {
	return Var(newStringSliceValue(value, p), name, usage).NArgs(0, 0)
}
