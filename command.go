package xflags

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/cavaliergopher/xflags/desc"
)

// TODO: Allow packages to declare global flags that are accessible on init.

// An Invocation is the result of parsing a command line. It records which
// command the arguments named and what was left for its handler.
type Invocation struct {
	// Cmd is the command the arguments named.
	Cmd *Command

	// Path names each command, starting from the one that was parsed --
	// conventionally the program itself, named for os.Args[0] -- and ending
	// with the command that was invoked.
	Path []string

	// Args holds any arguments that followed a "--" terminator.
	Args []string
}

// A HandlerFunc handles the invocation of a command specified by command
// line arguments.
//
// ctx is the context given to Run, so a handler that does anything
// cancellable should honour it. See NotifyContext for a context that is
// cancelled on SIGINT or SIGTERM.
//
// inv describes the invocation: the command that was named, the path it was
// reached by, and any arguments the parser ignored after the "--"
// terminator.
//
// Returning nil exits with code 0 and returning an error exits with code 1,
// unless the error implements ExitCoder, in which case it names its own
// code. See Command.Run for the whole contract.
type HandlerFunc func(ctx context.Context, inv *Invocation) error

// Command configures a command that users may invoke from the command line.
//
// Programs should not create Command directly and instead use NewCommand to
// construct one.
type Command struct {
	parent         *Command
	name           string
	usage          string
	synopsis       string
	hidden         bool
	withTerminator bool
	flagGroups     []*FlagGroup
	subcommands    []*Command
	formatFunc     FormatFunc
	handlerFunc    HandlerFunc
	stdout         io.Writer
	stderr         io.Writer

	// defaultGroup is the implicit "options" flag group appended to by
	// Flags, created lazily on first use so an unused group never appears.
	defaultGroup *FlagGroup
}

// NewCommand returns a new Command with the given name and usage string.
func NewCommand(name, usage string) *Command {
	return &Command{
		name:  name,
		usage: usage,
	}
}

func (c *Command) String() string { return c.name }

// Parse parses the given set of command line arguments and stores the value of
// each argument in each command flag's target. The rules for each flag are
// checked and any errors are returned.
//
// If -h or --help are specified, a HelpError will be returned containing the
// subcommand that was specified.
//
// The returned Invocation names this command, or one of its subcommands if
// the arguments specified one.
func (c *Command) Parse(args []string) (*Invocation, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	return newArgParser(c, args).Parse()
}

// root returns the root of the command tree c belongs to, which is c itself
// if it has no parent.
func (c *Command) root() *Command {
	root := c
	for root.parent != nil {
		root = root.parent
	}
	return root
}

// validate checks the whole command tree for configuration errors and
// returns the first one found.
//
// It always runs from the root, regardless of which command in the tree it
// is called on, so that Parse on a subcommand still validates the whole
// tree.
func (c *Command) validate() error {
	return c.root().validateTree()
}

// validateTree checks c and, recursively, each of its subcommands, returning
// the first error found.
func (c *Command) validateTree() error {
	if err := c.validateSelf(); err != nil {
		return err
	}
	for _, sub := range c.subcommands {
		if err := sub.validateTree(); err != nil {
			return err
		}
	}
	return nil
}

// validateSelf checks c's own flags for configuration errors: flag syntax,
// duplicate names, and positional/subcommand conflicts. It does not descend
// into subcommands.
func (c *Command) validateSelf() error {
	flagsByName := make(map[string]*Flag)
	hasUnboundedPositional := false
	for _, group := range c.flagGroups {
		for _, flag := range group.flags {
			if err := flag.check(); err != nil {
				return err
			}
			if flag.positional {
				if len(c.subcommands) > 0 {
					return errorf(
						"%s: cannot specify both subcommands and"+
							" positional arguments",
						c.name,
					)
				}
				if hasUnboundedPositional {
					return errorf(
						"%s: positional arguments cannot follow unbounded"+
							" positional arguments",
						c.name,
					)
				}
				if flag.maxCount == 0 {
					hasUnboundedPositional = true
				}
			}
			if flag.name != "" {
				key := "--" + flag.name
				if _, ok := flagsByName[key]; ok {
					return errorf("%s: flag already declared: %s", c.name, key)
				}
				flagsByName[key] = flag
			}
			if flag.shortName != "" {
				key := "-" + flag.shortName
				if _, ok := flagsByName[key]; ok {
					return errorf("%s: flag already declared: %s", c.name, key)
				}
				flagsByName[key] = flag
			}
		}
	}
	return nil
}

// Describe describes the whole command tree that c belongs to and returns
// the description of c's position within it. It is not limited to c: the
// tree is found by walking to its root, so the returned node carries
// complete ancestry via desc.Command.Parent as well as its own subcommands,
// and configuration errors anywhere in the tree are reported -- including
// in commands unrelated to c. This is what makes a description correct: a
// subcommand's usage line, inherited flags and environment variables are
// all meaningless without its ancestors.
//
// The errors returned are the same configuration errors Parse returns,
// which likewise validates from the root wherever it is called.
//
// Describe is pure: it does not mutate the command tree or the variables
// flags are bound to, so it is safe to call at any time, including while
// parsed values are live. There is no caching, so every call revalidates
// and describes the tree afresh.
func (c *Command) Describe() (*desc.Command, error) {
	root := c.root()
	if err := root.validateTree(); err != nil {
		return nil, err
	}
	nodeMap := make(map[*Command]*desc.Command)
	root.describe(nil, nodeMap)
	return nodeMap[c], nil
}

// describe recursively describes the command tree rooted at c, setting
// parent as the Parent of the resulting node, and recording every node in
// nodeMap so callers can look up the node for any source *Command after
// describing from the root.
func (c *Command) describe(
	parent *desc.Command,
	nodeMap map[*Command]*desc.Command,
) *desc.Command {
	node := &desc.Command{
		Parent:         parent,
		Name:           c.name,
		Usage:          c.usage,
		Synopsis:       c.synopsis,
		Hidden:         c.hidden,
		WithTerminator: c.withTerminator,
	}
	nodeMap[c] = node
	for _, group := range c.flagGroups {
		node.FlagGroups = append(node.FlagGroups, group.describe())
	}
	for _, sub := range c.subcommands {
		node.Subcommands = append(node.Subcommands, sub.describe(node, nodeMap))
	}
	return node
}

// output returns stdout and stderr, inheriting from parents and defaulting to
// OS defaults.
func (c *Command) output() (stdout, stderr io.Writer) {
	stdout, stderr = c.stdout, c.stderr
	if stdout == nil && stderr == nil {
		if c.parent != nil {
			return c.parent.output()
		}
		return os.Stdout, os.Stderr
	}
	return
}

// Run parses the given set of command line arguments and calls the handler
// for the command or subcommand specified by the arguments. It returns the
// exit code the program should terminate with, which follows a three-value
// contract:
//
//	0  the handler returned nil, or -h or --help was given
//	1  the handler returned an error
//	2  the command line was wrong, or named a command with no handler
//
// A handler may name its own exit code by returning an error that implements
// ExitCoder; see Exit and UsageErrorf.
//
// If -h or --help are specified, usage information is printed to the
// command's stdout. Errors, including a command invoked with no handler, are
// reported on its stderr. See Output.
//
// ctx is passed to the handler unchanged. See NotifyContext for a context
// that is cancelled on SIGINT or SIGTERM.
func (c *Command) Run(ctx context.Context, args []string) int {
	inv, err := c.Parse(args)
	if err != nil {
		return c.handleErr(err)
	}
	if inv.Cmd.handlerFunc == nil {
		_, stderr := inv.Cmd.output()
		if err := inv.Cmd.WriteUsage(stderr); err != nil {
			return reportFatal(err)
		}
		return exitUsage
	}
	return inv.Cmd.handleErr(inv.Cmd.handlerFunc(ctx, inv))
}

// handleErr reports err on the appropriate output for the command that
// produced it and returns the exit code the program should terminate with.
func (c *Command) handleErr(err error) int {
	if err == nil {
		return exitSuccess
	}
	var helpErr *HelpError
	if errors.As(err, &helpErr) {
		stdout, _ := helpErr.Cmd.output()
		if err := helpErr.Cmd.WriteUsage(stdout); err != nil {
			return reportFatal(err)
		}
		return exitSuccess
	}
	var argErr *ArgumentError
	if errors.As(err, &argErr) {
		_, stderr := argErr.Cmd.output()
		fmt.Fprintf(stderr, "Argument error: %s\n", argErr.String())
		return exitUsage
	}
	_, stderr := c.output()
	fmt.Fprintf(stderr, "Error: %s\n", errStr(err))
	return exitCode(err)
}

// reportFatal reports a failure to write to a command's own output, which is
// the one failure that output cannot report itself, and returns the exit
// code to terminate with.
func reportFatal(err error) int {
	fmt.Fprintf(os.Stderr, "xflags: %s\n", errStr(err))
	return exitFailure
}

// WriteUsage prints a help message to the given Writer using the configured
// Formatter.
//
// WriteUsage describes the command (see Describe) and hands the result to
// the resolved FormatFunc, so it returns the same configuration errors
// Parse would if the tree is misconfigured.
func (c *Command) WriteUsage(w io.Writer) error {
	node, err := c.Describe()
	if err != nil {
		return err
	}
	f := c.formatFunc
	for p := c; f == nil && p != nil; p = p.parent {
		f = p.formatFunc
	}
	if f == nil {
		f = Format
	}
	return f(w, node)
}

// Synopsis specifies the detailed help message for this command.
func (c *Command) Synopsis(s string) *Command {
	c.synopsis = s
	return c
}

// HandleFunc registers the handler for the command. If no handler is
// specified and the command is invoked, Run prints usage information to
// stderr and exits with the usage error code.
func (c *Command) HandleFunc(handler HandlerFunc) *Command {
	c.handlerFunc = handler
	return c
}

// Hidden hides the command from all help messages but still allows the command
// to be invoked on the command line.
func (c *Command) Hidden() *Command {
	c.hidden = true
	return c
}

// Flags appends command line flags to the implicit "options" flag group for
// this command, creating the group on its first use.
func (c *Command) Flags(flags ...*Flag) *Command {
	if c.defaultGroup == nil {
		c.defaultGroup = &FlagGroup{name: "options", usage: "Options"}
		c.flagGroups = append(c.flagGroups, c.defaultGroup)
	}
	c.defaultGroup.flags = append(c.defaultGroup.flags, flags...)
	return c
}

// FlagGroup adds a group of command line flags to this command and shows them
// under a common heading in help messages.
func (c *Command) FlagGroup(name, usage string, flags ...*Flag) *Command {
	c.flagGroups = append(c.flagGroups, &FlagGroup{
		name:  name,
		usage: usage,
		flags: flags,
	})
	return c
}

// FlagSet imports flags from a Flagset created using Go's flag package. All
// parsing and error handling is still managed by this package.
//
// To import any globally defined flags, import flag.CommandLine.
func (c *Command) FlagSet(flagSet *flag.FlagSet) *Command {
	flagSet.VisitAll(func(f *flag.Flag) {
		flg := Var(f.Value, f.Name, f.Usage)
		flg.defValue = f.DefValue
		c.Flags(flg)
	})
	return c
}

// Subcommands adds subcommands to this command and sets their parent to this
// command.
func (c *Command) Subcommands(cmds ...*Command) *Command {
	c.subcommands = append(c.subcommands, cmds...)
	for _, cmd := range cmds {
		cmd.parent = c
	}
	return c
}

// FormatFunc specifies a custom FormatFunc for formatting help messages for
// this command.
func (c *Command) FormatFunc(fn FormatFunc) *Command {
	c.formatFunc = fn
	return c
}

// WithTerminator specifies that any command line argument after "--" will be
// passed through to the args parameter of the command's handler without any
// further processing.
func (c *Command) WithTerminator() *Command {
	c.withTerminator = true
	return c
}

// Output sets the destination for usage and error messages.
func (c *Command) Output(stdout, stderr io.Writer) *Command {
	c.stdout, c.stderr = stdout, stderr
	return c
}
