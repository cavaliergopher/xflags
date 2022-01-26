package xflags

import (
	"strings"
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
	if strings.HasPrefix(c.Name, "-") {
		return nil, errorf("%s: invalid flag name", c.name())
	}
	if c.Value == nil {
		return nil, errorf("%s: value cannot be nil", c.name())
	}
	if len(c.ShortName) > 1 {
		return nil, errorf(
			"short name must be one character in length: %s",
			c.ShortName,
		)
	}
	if c.MinCount < 0 ||
		c.MaxCount < 0 ||
		(c.MaxCount > 0 && c.MinCount > c.MaxCount) {
		return nil, errorf(
			"%s: invalid NArgs: %d, %d",
			c.name(),
			c.MinCount,
			c.MaxCount,
		)
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

// FlagGroup is a nominal grouping of flags which affects how the flags are
// shown in help messages.
type FlagGroup struct {
	Name  string
	Usage string
	Flags []*Flag
}

type flagGroupBuilder struct {
	group FlagGroup
	flags []Flagger
}

func newFlagGroupBuilder(name, usage string, flags ...Flagger) *flagGroupBuilder {
	c := &flagGroupBuilder{
		group: FlagGroup{
			Name:  name,
			Usage: usage,
		},
	}
	c.append(flags...)
	return c
}

func (c *flagGroupBuilder) append(flags ...Flagger) {
	c.flags = append(c.flags, flags...)
}

func (c *flagGroupBuilder) FlagGroup() (*FlagGroup, error) {
	group := c.group
	for _, flagger := range c.flags {
		flag, err := flagger.Flag()
		if err != nil {
			return nil, err
		}
		group.Flags = append(group.Flags, flag)
	}
	return &group, nil
}

// FlagBuilder builds a Flag which defines a command line flag for a CLI command.
// All chain methods return a pointer to the same builder.
type FlagBuilder struct {
	flag Flag
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

// Choices is a convenience method that calls Validate and sets ValidateFunc
// that enforces that the flag value must be one of the given choices.
func (c *FlagBuilder) Choices(elems ...string) *FlagBuilder {
	return c.Validate(
		func(arg string) error {
			for _, elem := range elems {
				if arg == elem {
					return nil
				}
			}
			return errorf("please specify one of [ %s ]", strings.Join(elems, " "))
		},
	)
}

// Flag implements the Flagger interface and produces a new Flag.
func (c *FlagBuilder) Flag() (*Flag, error) {
	flag := c.flag
	return flag.Flag()
}

// Must is a helper that calls Build and panics if the error is non-nil.
func (c *FlagBuilder) Must() *Flag {
	flag, err := c.Flag()
	if err != nil {
		panic(err)
	}
	return flag
}
