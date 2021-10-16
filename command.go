package xflags

import (
	"flag"
	"io"
	"os"
)

// CommandFunc is a function implements the execution of a command specified
// in command line arguments.
//
// Args will always be empty and is a placeholder for future releases that will
// pass unhandled arguments to the handler.
type CommandFunc func(args []string) int

// CommandInfo describes a command that users may invoke from the command line.
//
// Programs should not create CommandInfo directly and instead use the Command
// function to build one with proper error checking.
type CommandInfo struct {
	Parent         *CommandInfo
	Name           string
	Usage          string
	Synopsis       string
	Hidden         bool
	WithTerminator bool
	Flags          []*FlagInfo
	Subcommands    []*CommandInfo
	Formatter      Formatter
	Handler        CommandFunc

	args []string
}

func (c *CommandInfo) String() string { return c.Name }

// Args returns any command line arguments specified after the "--" terminator
// if it was enabled.
func (c *CommandInfo) Args() []string { return c.args }

// Parse parses the given set of command line arguments and stores the value of
// each argument in each command flag's target. The rules for each flag are
// checked and any errors are returned.
//
// The returned *CommandInfo is the one for the same command or subcommand
// specified by the arguments.
func (c *CommandInfo) Parse(args []string) (*CommandInfo, error) {
	cmd, args, err := newArgParser(c, args).Parse()
	if err != nil {
		return nil, err
	}
	cmd.args = args
	return cmd, nil
}

// Run parses the given set of command line arguments and calls the handler for
// the command or subcommand specified by the arguments.
func (c *CommandInfo) Run(args []string) int {
	var err error
	c, err = c.Parse(args)
	if err != nil {
		return handleErr(err)
	}
	if flagHelp {
		return c.usage(0)
	}
	if c.Handler == nil {
		return c.usage(1)
	}
	return c.Handler(c.args)
}

// WriteUsage prints a help message to the given Writer using the configured
// Formatter.
func (c *CommandInfo) WriteUsage(w io.Writer) error {
	f := c.Formatter
	for p := c; f == nil && p != nil; p = p.Parent {
		f = p.Formatter
	}
	if f == nil {
		f = DefaultFormatter
	}
	return f(w, c)
}

func (c *CommandInfo) usage(exitCode int) int {
	w := os.Stdout
	if exitCode != 0 {
		w = os.Stderr
	}
	if err := c.WriteUsage(w); err != nil {
		return handleErr(err)
	}
	return exitCode
}

// CommandBuilder builds a CommandInfo which defines a command and all of its
// flags.
type CommandBuilder struct {
	info *CommandInfo
	err  error
}

// Command returns a CommandBuilder which can be used to define a command and
// all of its flags.
func Command(name string) *CommandBuilder {
	c := &CommandBuilder{
		info: &CommandInfo{
			Name:        name,
			Flags:       make([]*FlagInfo, 0),
			Subcommands: make([]*CommandInfo, 0),
		},
	}
	return c.Flags(helpFlag)
}

func (c *CommandBuilder) setErr(err error) {
	if c.err != nil {
		return
	}
	c.err = err
}

// Handler specifies the function to call when this command is specified on the
// the command line.
func (c *CommandBuilder) Handler(handler CommandFunc) *CommandBuilder {
	c.info.Handler = handler
	return c
}

// Usage sets a short description of the command to show in help messages.
func (c *CommandBuilder) Usage(usage string) *CommandBuilder {
	c.info.Usage = usage
	return c
}

// Hidden hides the command from all help messages but still allows the command
// to be invoked on the command line.
func (c *CommandBuilder) Hidden() *CommandBuilder {
	c.info.Hidden = true
	return c
}

func (c *CommandBuilder) flag(flag *FlagInfo) *CommandBuilder {
	if flag.Positional {
		// cannot mix positionals with subcommands
		if len(c.info.Subcommands) > 0 {
			c.setErr(newArgError(1, "cannot specify both subcommands and positional argument: %v", flag))
			return c
		}

		// positionals cannot follow variable length positionals
		for _, other := range c.info.Flags {
			if !other.Positional {
				continue
			}
			if other.MaxCount > 0 && other.MaxCount == other.MinCount {
				continue
			}
			err := newArgError(
				1,
				"positional arguments cannot follow variable-length positional arguments: %v",
				flag,
			)
			c.setErr(err)
			return c
		}
	}
	c.info.Flags = append(c.info.Flags, flag)
	return c
}

// Flag adds command line flags for this command.
func (c *CommandBuilder) Flags(flags ...*FlagInfo) *CommandBuilder {
	for _, flag := range flags {
		c = c.flag(flag)
	}
	return c
}

// FlagSet imports flags from a Flagset created using Go's flag package. All
// parsing and error handling is still managed by this package.
//
// To import any globally defined flags, import flag.CommandLine.
func (c *CommandBuilder) FlagSet(flagSet *flag.FlagSet) *CommandBuilder {
	flagSet.VisitAll(func(f *flag.Flag) {
		flagInfo, err := Var(f.Value, f.Name, f.Usage).Build()
		if err != nil {
			c.setErr(err)
			return
		}
		c = c.Flags(flagInfo)
	})
	return c
}

func (c *CommandBuilder) subcommand(cmd *CommandInfo) *CommandBuilder {
	for _, flag := range c.info.Flags {
		if flag.Positional {
			c.setErr(newArgError(1, "cannot specify both subcommands and positional argument: %v", cmd))
			return c
		}
	}
	cmd.Parent = c.info
	c.info.Subcommands = append(c.info.Subcommands, cmd)
	return c
}

// Subcommands adds subcommands to this command.
func (c *CommandBuilder) Subcommands(commands ...*CommandInfo) *CommandBuilder {
	for _, cmd := range commands {
		c = c.subcommand(cmd)
	}
	return c
}

// Formatter specifies a custom Formatter for formatting help messages for this
// command.
func (c *CommandBuilder) Formatter(formatter Formatter) *CommandBuilder {
	c.info.Formatter = formatter
	return c
}

// WithTerminator specifies that any command line argument after "--" will be
// passed through to the args parameter of the command's handler without any
// further processing.
func (c *CommandBuilder) WithTerminator() *CommandBuilder {
	c.info.WithTerminator = true
	return c
}

// Build checks for any correctness errors in the specification of the command
// and produces a CommandInfo.
func (c *CommandBuilder) Build() (*CommandInfo, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.info, nil
}

// Must is a helper that calls Build and panics if the error is non-nil. It is
// intended only for use in variable initializations.
func (c *CommandBuilder) Must() *CommandInfo {
	info, err := c.Build()
	if err != nil {
		panic(err)
	}
	return info
}

// Run parses the arguments provided by os.Args and executes the handler for the
// command or subcommand specified by the arguments.
//
//     func main() {
//         os.Exit(xflags.Run(cmd))
//     }
//
func Run(cmd *CommandInfo) int {
	return cmd.Run(os.Args[1:])
}
