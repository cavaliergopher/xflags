package xflags

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// TODO: Allow packages to declare global flags that are accessible on init.

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
	FormatFunc     FormatFunc
	HandlerFunc    HandlerFunc
	Output         io.Writer

	args []string
}

// Command implements the Commander interface.
func (c *Command) Command() (*Command, error) {
	flagsByName := make(map[string]*Flag)
	hasUnboundedPositional := false
	for _, group := range c.FlagGroups {
		for _, flag := range group.Flags {
			if flag.Positional {
				if len(c.Subcommands) > 0 {
					return nil, errorf(
						"%s: cannot specify both subcommands and"+
							" positional arguments",
						c.Name,
					)
				}
				if hasUnboundedPositional {
					return nil, errorf(
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
					return nil, errorf("%s: flag already declared: %s", c.Name, key)
				}
				flagsByName[key] = flag
			}
			if flag.ShortName != "" {
				key := "-" + flag.ShortName
				if _, ok := flagsByName[key]; ok {
					return nil, errorf("%s: flag already declared: %s", c.Name, key)
				}
				flagsByName[key] = flag
			}
		}
	}
	return c, nil
}

func (c *Command) String() string { return c.Name }

// Args returns any command line arguments specified after the "--" terminator
// if it was enabled. Args is only populated after the command line is
// successfully parsed.
func (c *Command) Args() []string { return c.args }

// Arg returns the i'th argument specified after the "--" terminator if it was enabled. Arg(0) is
// the first remaining argument after flags the terminator. Arg returns an empty string if the
// requested element does not exist.
func (c *Command) Arg(i int) string {
	if i < 0 || i >= len(c.args) {
		return ""
	}
	return c.args[i]
}

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

func (c *Command) output() io.Writer {
	if c.Output != nil {
		return c.Output
	}
	return os.Stdout
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
	target, err := c.Parse(args)
	if err != nil {
		return c.handleErr(err)
	}
	if target.HandlerFunc == nil {
		if err := target.WriteUsage(target.output()); err != nil {
			return target.handleErr(err)
		}
		return 1
	}
	return target.HandlerFunc(target.args)
}

func (c *Command) handleErr(err error) int {
	if err == nil {
		return 0
	}
	w := c.output()
	if err, ok := err.(*HelpError); ok {
		if err := err.Cmd.WriteUsage(w); err != nil {
			return c.handleErr(err)
		}
		return 0
	}
	if err, ok := err.(*ArgumentError); ok {
		fmt.Fprintf(w, "Argument error: %s\n", err.Msg)
		return 1
	}
	fmt.Fprintf(w, "Error: %v\n", err)
	return 1
}

// WriteUsage prints a help message to the given Writer using the configured
// Formatter.
func (c *Command) WriteUsage(w io.Writer) error {
	f := c.FormatFunc
	for p := c; f == nil && p != nil; p = p.Parent {
		f = p.FormatFunc
	}
	if f == nil {
		f = Format
	}
	return f(w, c)
}

// CommandBuilder builds a Command which defines a command and all of its flags.
// Create a command builder with NewCommand.
// All chain methods return a pointer to the same builder.
type CommandBuilder struct {
	cmd         Command
	flagGroups  []*flagGroupBuilder
	subcommands []Commander
	err         error
}

// NewCommand returns a CommandBuilder which can be used to define a command and
// all of its flags.
func NewCommand(name, usage string) *CommandBuilder {
	c := &CommandBuilder{
		cmd: Command{
			Name:  name,
			Usage: usage,
		},
		flagGroups:  make([]*flagGroupBuilder, 1, 8),
		subcommands: make([]Commander, 0, 8),
	}
	c.flagGroups[0] = newFlagGroupBuilder("options", "Options")
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
	c.flagGroups[0].append(flags...)
	return c
}

// FlagGroup adds a group of command line flags to this command and shows them
// under a common heading in help messages.
func (c *CommandBuilder) FlagGroup(
	name, usage string,
	flags ...Flagger,
) *CommandBuilder {
	c.flagGroups = append(c.flagGroups, newFlagGroupBuilder(name, usage, flags...))
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
	c.subcommands = append(c.subcommands, commands...)
	return c
}

// Formatter specifies a custom Formatter for formatting help messages for this
// command.
func (c *CommandBuilder) FormatFunc(fn FormatFunc) *CommandBuilder {
	c.cmd.FormatFunc = fn
	return c
}

// WithTerminator specifies that any command line argument after "--" will be
// passed through to the args parameter of the command's handler without any
// further processing.
func (c *CommandBuilder) WithTerminator() *CommandBuilder {
	c.cmd.WithTerminator = true
	return c
}

// Output sets the destination for usage and error messages. If output is nil, os.Stderr is used.
func (c *CommandBuilder) Output(w io.Writer) *CommandBuilder {
	c.cmd.Output = w
	return c
}

// Command implements the Commander interface and produces a new Command.
func (c *CommandBuilder) Command() (*Command, error) {
	if c.err != nil {
		return nil, c.err
	}
	cmd := c.cmd
	for _, groupBuilder := range c.flagGroups {
		group, err := groupBuilder.FlagGroup()
		if err != nil {
			return nil, err
		}
		cmd.FlagGroups = append(cmd.FlagGroups, group)
	}
	for _, commandBuilder := range c.subcommands {
		sub, err := commandBuilder.Command()
		if err != nil {
			return nil, err
		}
		cmd.Subcommands = append(cmd.Subcommands, sub)
		sub.Parent = &cmd
	}
	return cmd.Command()
}

// Must is a helper that calls Command and panics if the error is non-nil.
func (c *CommandBuilder) Must() *Command {
	cmd, err := c.Command()
	if err != nil {
		panic(err)
	}
	return cmd
}
