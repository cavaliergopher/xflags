package xflags

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/cavaliergopher/xflags/desc"
)

// TODO: Allow packages to declare global flags that are accessible on init.

// A HandlerFunc is a function that handles the invokation a command specified
// by command line arguments.
//
// Args will receive any arguments ignored by the parser after the "--"
// terminator if it is enabled.
type HandlerFunc func(args []string) int

// Command describes a command that users may invoke from the command line.
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

	args []string
}

// NewCommand returns a new Command with the given name and usage string.
func NewCommand(name, usage string) *Command {
	return &Command{
		name:  name,
		usage: usage,
	}
}

func (c *Command) String() string { return c.name }

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
	if err := c.validate(); err != nil {
		return nil, err
	}
	cmd, args, err := newArgParser(c, args).Parse()
	if err != nil {
		return nil, err
	}
	cmd.args = args
	return cmd, nil
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
	nodes := make(map[*Command]*desc.Command)
	root.describe(nil, nodes)
	return nodes[c], nil
}

// describe recursively describes the command tree rooted at c, setting
// parent as the Parent of the resulting node, and recording every node in
// nodes so callers can look up the node for any source *Command after
// describing from the root.
func (c *Command) describe(
	parent *desc.Command,
	nodes map[*Command]*desc.Command,
) *desc.Command {
	node := &desc.Command{
		Parent:         parent,
		Name:           c.name,
		Usage:          c.usage,
		Synopsis:       c.synopsis,
		Hidden:         c.hidden,
		WithTerminator: c.withTerminator,
	}
	nodes[c] = node
	for _, group := range c.flagGroups {
		node.FlagGroups = append(node.FlagGroups, group.describe())
	}
	for _, sub := range c.subcommands {
		node.Subcommands = append(node.Subcommands, sub.describe(node, nodes))
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
	if target.handlerFunc == nil {
		_, stderr := target.output()
		if err := target.WriteUsage(stderr); err != nil {
			panic(err)
		}
		return 1
	}
	return target.handlerFunc(target.args)
}

func (c *Command) handleErr(err error) int {
	if err == nil {
		return 0
	}
	var helpErr *HelpError
	if errors.As(err, &helpErr) {
		stdout, _ := helpErr.Cmd.output()
		if stdout != os.Stdout {
			if f, ok := stdout.(*os.File); ok {
				panic(f.Name())
			}
			panic(stdout)
		}
		if err := helpErr.Cmd.WriteUsage(stdout); err != nil {
			panic(err)
		}
		return 0
	}
	var argErr *ArgumentError
	if errors.As(err, &argErr) {
		_, stderr := argErr.Cmd.output()
		fmt.Fprintf(stderr, "Argument error: %s\n", argErr.String())
		return 1
	}
	_, stderr := c.output()
	fmt.Fprintf(stderr, "Error: %v\n", errStr(err))
	return 1
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
// specified and the command is invoked, it will print usage information to
// stderr.
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
