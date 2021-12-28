package xflags

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// TODO: move error checking into Command/Flag.

// Commander is an interface that describes any type that produces a Command.
//
// The interface is implemented by both CommandBuilder and Command so they can
// often be used interchangeably.
type Commander interface {
	Command() (*Command, error)
}

// A HandlerFunc is a function that handles the invokation a command specified
// by command line arguments.
//
// Args will receive any arguments ignored by the parser after the "--"
// terminator if it is enabled.
type HandlerFunc func(args []string) int

// Command describes a command that users may invoke from the command line.
//
// Programs should not create Command directly and instead use the Command
// function to build one with proper error checking.
type Command struct {
	Parent         *Command
	Name           string
	Usage          string
	Synopsis       string
	Hidden         bool
	WithTerminator bool
	FlagGroups     []*FlagGroup
	Subcommands    []*Command
	Formatter      Formatter
	HandlerFunc    HandlerFunc

	args []string
}

// Command implements the Commander interface.
func (c *Command) Command() (*Command, error) {
	if err := c.Err(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Command) String() string { return c.Name }

// Args returns any command line arguments specified after the "--" terminator
// if it was enabled. Args is only populated after the command line is
// successfully parsed.
func (c *Command) Args() []string { return c.args }

// Parse parses the given set of command line arguments and stores the value of
// each argument in each command flag's target. The rules for each flag are
// checked and any errors are returned.
//
// If -h or --help are specified, a HelpError will be returned containing the
// subcommand that was specified.
//
// The returned *Command will be this command or one of its subcommands if
// specified by the command line arguments.
func (c *Command) Parse(args []string) (*Command, error) {
	cmd, args, err := newArgParser(c, args).Parse()
	if err != nil {
		return nil, err
	}
	cmd.args = args
	return cmd, nil
}

// Run parses the given set of command line arguments and calls the handler
// for the command or subcommand specified by the arguments.
//
// If -h or --help are specified, usage information will be printed to os.Stdout
// and the return code will be 0.
//
// If a command is invoked that has no handler, usage information will be
// printed to os.Stderr and the return code will be non-zero.
func (c *Command) Run(args []string) int {
	var err error
	c, err = c.Parse(args)
	if err != nil {
		return c.handleErr(err)
	}
	if c.HandlerFunc == nil {
		if err := c.WriteUsage(os.Stderr); err != nil {
			return c.handleErr(err)
		}
		return 1
	}
	return c.HandlerFunc(c.args)
}

func (c *Command) handleErr(err error) int {
	if err == nil {
		return 0
	}
	if err, ok := err.(*HelpError); ok {
		if err := err.Cmd.WriteUsage(os.Stdout); err != nil {
			return c.handleErr(err)
		}
		return 0
	}
	if err, ok := err.(*ArgumentError); ok {
		fmt.Fprintf(os.Stderr, "Argument error: %s\n", err.Msg)
		return 1
	}
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	return 1
}

// WriteUsage prints a help message to the given Writer using the configured
// Formatter.
func (c *Command) WriteUsage(w io.Writer) error {
	f := c.Formatter
	for p := c; f == nil && p != nil; p = p.Parent {
		f = p.Formatter
	}
	if f == nil {
		f = DefaultFormatter
	}
	return f(w, c)
}

// Err checks the command configuration and returns the first error it
// encounters.
func (c *Command) Err() error {
	flagsByName := make(map[string]*Flag)
	hasUnboundedPositional := false
	for _, group := range c.FlagGroups {
		for _, flag := range group.Flags {
			if flag.Positional {
				if len(c.Subcommands) > 0 {
					return errorf(
						"%s: cannot specify both subcommands and"+
							" positional arguments",
						c.Name,
					)
				}
				if hasUnboundedPositional {
					return errorf(
						"%s: positional arguments cannot follow unbounded"+
							" positional arguments",
						c.Name,
					)
				}
				if flag.MaxCount == 0 {
					hasUnboundedPositional = true
				}
			}
			if flag.Name != "" {
				key := "--" + flag.Name
				if _, ok := flagsByName[key]; ok {
					return errorf("%s: flag already declared: %s", c.Name, key)
				}
				flagsByName[key] = flag
			}
			if flag.ShortName != "" {
				key := "-" + flag.ShortName
				if _, ok := flagsByName[key]; ok {
					return errorf("%s: flag already declared: %s", c.Name, key)
				}
				flagsByName[key] = flag
			}
		}
	}
	return nil
}

// CommandBuilder builds a Command which defines a command and all of its
// flags. Create a command builder with NewCommand.
type CommandBuilder struct {
	cmd *Command
	err error
}

// NewCommand returns a CommandBuilder which can be used to define a command and
// all of its flags.
func NewCommand(name, usage string) *CommandBuilder {
	c := &CommandBuilder{
		cmd: &Command{
			Name:        name,
			Usage:       usage,
			FlagGroups:  make([]*FlagGroup, 1),
			Subcommands: make([]*Command, 0),
		},
	}
	c.cmd.FlagGroups[0] = &FlagGroup{Name: "options", Usage: "Options"}
	return c
}

func (c *CommandBuilder) error(err error) *CommandBuilder {
	if c.err != nil {
		return c
	}
	c.err = err
	return c
}

// Synopsis specifies the detailed help message for this command.
func (c *CommandBuilder) Synopsis(s string) *CommandBuilder {
	c.cmd.Synopsis = s
	return c
}

// HandleFunc registers the handler for the command. If no handler is specified
// and the command is invoked, it will print usage information to stderr.
func (c *CommandBuilder) HandleFunc(
	handler func(args []string) int,
) *CommandBuilder {
	if handler == nil {
		return c.error(errorf("%s: nil handler", c.cmd.Name))
	}
	c.cmd.HandlerFunc = handler
	return c
}

// Hidden hides the command from all help messages but still allows the command
// to be invoked on the command line.
func (c *CommandBuilder) Hidden() *CommandBuilder {
	c.cmd.Hidden = true
	return c
}

// Flag adds command line flags to the default FlagGroup for this command.
func (c *CommandBuilder) Flags(flags ...Flagger) *CommandBuilder {
	for _, flagger := range flags {
		flag, err := flagger.Flag()
		if err != nil {
			return c.error(err)
		}
		c.cmd.FlagGroups[0].append(flag)
	}
	return c
}

// FlagGroup adds a group of command line flags to this command and shows them
// under a common heading in help messages.
func (c *CommandBuilder) FlagGroup(
	name, usage string,
	flags ...Flagger,
) *CommandBuilder {
	group := &FlagGroup{
		Name:  name,
		Usage: usage,
	}
	for _, flagger := range flags {
		flag, err := flagger.Flag()
		if err != nil {
			return c.error(err)
		}
		group.append(flag)
	}
	c.cmd.FlagGroups = append(c.cmd.FlagGroups, group)
	return c
}

// FlagSet imports flags from a Flagset created using Go's flag package. All
// parsing and error handling is still managed by this package.
//
// To import any globally defined flags, import flag.CommandLine.
func (c *CommandBuilder) FlagSet(flagSet *flag.FlagSet) *CommandBuilder {
	flagSet.VisitAll(func(f *flag.Flag) {
		flag, err := Var(f.Value, f.Name, f.Usage).Flag()
		if err != nil {
			c.err = err
			return
		}
		c = c.Flags(flag)
	})
	return c
}

// Subcommands adds subcommands to this command.
func (c *CommandBuilder) Subcommands(commands ...Commander) *CommandBuilder {
	if len(commands) == 0 {
		return c
	}
	for _, commander := range commands {
		cmd, err := commander.Command()
		if err != nil {
			return c.error(err)
		}
		cmd.Parent = c.cmd
		c.cmd.Subcommands = append(c.cmd.Subcommands, cmd)
	}
	return c
}

// Formatter specifies a custom Formatter for formatting help messages for this
// command.
func (c *CommandBuilder) Formatter(formatter Formatter) *CommandBuilder {
	c.cmd.Formatter = formatter
	return c
}

// WithTerminator specifies that any command line argument after "--" will be
// passed through to the args parameter of the command's handler without any
// further processing.
func (c *CommandBuilder) WithTerminator() *CommandBuilder {
	c.cmd.WithTerminator = true
	return c
}

// Command checks for any correctness errors in the specification of the command
// and produces a Command.
func (c *CommandBuilder) Command() (*Command, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.cmd.Command()
}

// Must is a helper that calls Command and panics if the error is non-nil.
func (c *CommandBuilder) Must() *Command {
	cmd, err := c.Command()
	if err != nil {
		panic(err)
	}
	return cmd
}
