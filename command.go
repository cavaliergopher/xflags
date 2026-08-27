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
// command the arguments named, what was left for its handler, and the
// streams the handler should read and write.
type Invocation struct {
	// Cmd is the command the arguments named.
	Cmd *Command

	// Path names each command, starting from the one that was parsed --
	// conventionally the program itself, named for os.Args[0] -- and ending
	// with the command that was invoked.
	Path []string

	// Forwarded holds the arguments that followed a "--" terminator, which
	// the parser did not interpret. It is empty unless the command opted in
	// with Command.ForwardArgs.
	//
	// This is not the command's operands, which bind to positional flags as
	// usual. These are the arguments the command means to hand on to
	// something else.
	Forwarded []string

	// HelpRequested reports that -h or --help was given, in which case Cmd is
	// the command whose usage was asked for and no handler should run. The
	// rest of the command line is not parsed and the flag rules are not
	// checked, so help works even on an otherwise incomplete command line.
	HelpRequested bool

	// Stdin, Stdout and Stderr are the streams the handler should use in
	// place of the process streams, so that whoever composes the binary decides
	// where its input and output go. Each is resolved independently from the
	// invoked command and its ancestors, defaulting to the matching process
	// stream; see Command.Stdin, Command.Stdout and Command.Stderr.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// A HandlerFunc handles the invocation of a command specified by command
// line arguments.
//
// ctx is the context given to Run, so a handler that does anything
// cancelable should honor it. See NotifyContext for a context that is
// canceled on SIGINT or SIGTERM.
//
// inv describes the invocation: the command that was named, the path it was
// reached by, any arguments forwarded past a "--" terminator, and the
// streams to work with. A handler should read inv.Stdin and write
// inv.Stdout and inv.Stderr rather than the process streams, so that a
// caller that redirects the command captures its output too. Nothing
// enforces it; a handler that reaches for os.Stdout simply escapes the
// redirection.
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
	parent      *Command
	name        string
	summary     string
	description string
	hidden      bool
	forwardArgs bool
	flagGroups  []*FlagGroup
	subcommands []*Command
	formatFunc  FormatFunc
	handlerFunc HandlerFunc
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer

	// defaultGroup is the implicit "options" flag group appended to by
	// Flags, created lazily on first use so an unused group never appears.
	defaultGroup *FlagGroup
}

// NewCommand returns a new Command with the given name and summary.
//
// summary is the one-line description of the command, shown beside its name
// where a parent lists its subcommands, and beneath the usage line in the
// command's own help message. See Command.Description for the longer prose
// that follows it.
func NewCommand(name, summary string) *Command {
	return (&Command{
		name:    name,
		summary: summary,
	}).Flags()
}

func (c *Command) String() string { return c.name }

// Parse parses the given set of command line arguments and stores the value of
// each argument in each command flag's target. The rules for each flag are
// checked and any errors are returned.
//
// The returned Invocation names this command, or one of its subcommands if
// the arguments specified one.
//
// If -h or --help are specified, parsing stops there and the returned
// Invocation has HelpRequested set. That is not an error: it is for the
// caller to report the command's usage. See Command.Run.
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
	return c.root().validateTree(nil)
}

// validateTree checks c and, recursively, each of its subcommands, returning
// the first error found. claimed maps each option spelling declared by c's
// ancestors to the command that declared it: a name may not repeat anywhere
// along an ancestor-descendant chain, and the check runs here because a
// command cannot know its ancestors until the whole tree is in view. See
// docs/adr/path-scoped-flag-names.md.
func (c *Command) validateTree(claimed map[string]*Command) error {
	if err := c.validateSelf(claimed); err != nil {
		return err
	}
	if len(c.subcommands) == 0 {
		return nil
	}
	// Descendants see c's names claimed in a copy, so sibling subtrees may
	// still reuse names freely.
	claims := make(map[string]*Command, len(claimed))
	for key, cmd := range claimed {
		claims[key] = cmd
	}
	for _, group := range c.flagGroups {
		for _, flag := range group.flags {
			if flag.name != "" {
				claims["--"+flag.name] = c
			}
			if flag.shortName != "" {
				claims["-"+flag.shortName] = c
			}
		}
	}
	for _, sub := range c.subcommands {
		if err := sub.validateTree(claims); err != nil {
			return err
		}
	}
	return nil
}

// validateSelf checks c's own flags for configuration errors: flag syntax,
// names already declared -- within c, or by the ancestors whose claims are
// passed in -- and positional/subcommand conflicts. It does not descend
// into subcommands.
func (c *Command) validateSelf(claimed map[string]*Command) error {
	flagsByName := make(map[string]*Flag)
	hasUnboundedPositional := false
	for _, group := range c.flagGroups {
		for _, flag := range group.flags {
			if err := flag.check(); err != nil {
				return err
			}
			if flag.positional {
				if len(c.subcommands) > 0 {
					return newConfigErrorf(nil, c, flag, "cannot specify both subcommands and positional arguments")
				}
				if hasUnboundedPositional {
					return newConfigErrorf(nil, c, flag, "positional arguments cannot follow unbounded positional arguments")
				}
				if flag.maxCount == 0 {
					hasUnboundedPositional = true
				}
			}
			if flag.name != "" {
				key := "--" + flag.name
				if _, ok := flagsByName[key]; ok {
					return newConfigErrorf(nil, c, flag, "flag already declared: %s", key)
				}
				if ancestor, ok := claimed[key]; ok {
					return newConfigErrorf(nil, c, flag,
						"flag already declared by ancestor %q: %s",
						ancestor.name, key)
				}
				flagsByName[key] = flag
			}
			if flag.shortName != "" {
				key := "-" + flag.shortName
				if _, ok := flagsByName[key]; ok {
					return newConfigErrorf(nil, c, flag, "flag already declared: %s", key)
				}
				if ancestor, ok := claimed[key]; ok {
					return newConfigErrorf(nil, c, flag,
						"flag already declared by ancestor %q: %s",
						ancestor.name, key)
				}
				flagsByName[key] = flag
			}
		}
	}
	return nil
}

// findDescendantWithFlag returns the first descendant of c to declare the
// option spelled key -- a "--name" or "-s" -- searching depth first in
// declaration order, or nil when none does. A name declared below the
// current command is legal only once its own command is named, so
// unrecognized-option errors use this to say where the name would work;
// see docs/adr/path-scoped-flag-names.md.
func (c *Command) findDescendantWithFlag(key string) *Command {
	for _, sub := range c.subcommands {
		for _, group := range sub.flagGroups {
			for _, flag := range group.flags {
				if flag.positional {
					continue
				}
				if (flag.name != "" && key == "--"+flag.name) ||
					(flag.shortName != "" && key == "-"+flag.shortName) {
					return sub
				}
			}
		}
		if found := sub.findDescendantWithFlag(key); found != nil {
			return found
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
	if err := root.validateTree(nil); err != nil {
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
		Parent:      parent,
		Name:        c.name,
		Summary:     c.summary,
		Description: c.description,
		Hidden:      c.hidden,
		ForwardArgs: c.forwardArgs,
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

// getStdin returns the reader the command's handler reads, inheriting from
// the nearest ancestor that set one and defaulting to the process stream.
// Each stream resolves on its own, so redirecting one leaves the others
// where they were.
func (c *Command) getStdin() io.Reader {
	for p := c; p != nil; p = p.parent {
		if p.stdin != nil {
			return p.stdin
		}
	}
	return os.Stdin
}

// getStdout returns the writer for the command's help messages and for
// whatever its handler writes to Invocation.Stdout. See getStdin.
func (c *Command) getStdout() io.Writer {
	for p := c; p != nil; p = p.parent {
		if p.stdout != nil {
			return p.stdout
		}
	}
	return os.Stdout
}

// getStderr returns the writer for the command's error messages and for
// whatever its handler writes to Invocation.Stderr. See getStdin.
func (c *Command) getStderr() io.Writer {
	for p := c; p != nil; p = p.parent {
		if p.stderr != nil {
			return p.stderr
		}
	}
	return os.Stderr
}

// Run parses the given set of command line arguments and calls the handler
// for the command or subcommand specified by the arguments. It returns the
// exit code the program should terminate with, which follows a three-value
// contract:
//
//	0  the handler returned nil, or -h or --help was given
//	1  the handler returned an error
//	2  the command line or the command tree was wrong, or there is no handler
//
// A handler may name its own exit code by returning an error that implements
// ExitCoder; see Exit and Exitf.
//
// If -h or --help are specified, usage information is printed to the
// command's stdout. Errors, including a command invoked with no handler, are
// reported on its stderr. The handler is given the same streams on its
// Invocation. See Stdin, Stdout and Stderr.
//
// ctx is passed to the handler unchanged. See NotifyContext for a context
// that is canceled on SIGINT or SIGTERM.
func (c *Command) Run(ctx context.Context, args []string) int {
	inv, err := c.Parse(args)
	if err != nil {
		return c.handleErr(err)
	}
	if inv.HelpRequested {
		if err := inv.Cmd.WriteUsage(inv.Stdout); err != nil {
			return fallbackToStderr(err)
		}
		return ExitCodeSuccess // Help was requested, not an error.
	}
	if inv.Cmd.handlerFunc == nil {
		// The command exists only to group its subcommands, so naming it
		// alone is a usage error rather than a request for help. Its usage
		// goes to stderr, unlike the help path above, and the exit code
		// says nothing ran.
		if err := inv.Cmd.WriteUsage(inv.Stderr); err != nil {
			return fallbackToStderr(err)
		}
		return ExitCodeUsage
	}
	return inv.Cmd.handleErr(inv.Cmd.handlerFunc(ctx, inv))
}

// handleErr reports err on the appropriate output for the command that
// produced it and returns the exit code the program should terminate with.
func (c *Command) handleErr(err error) int {
	if err == nil {
		return ExitCodeSuccess
	}

	errStr := errorOrString(err)

	errTypeName := "Error"

	var argErr *ArgumentError
	var cfgErr *ConfigError
	switch {
	case errors.As(err, &argErr):
		errTypeName = "Argument error"

	case errors.As(err, &cfgErr):
		// The tree is malformed, so the fault is the program's rather than
		// the user's. Both exit 2, so the prefix is all that says which.
		errTypeName = "Program error"
	}

	if _, err := fmt.Fprintf(c.getStderr(), "%s: %s\n", errTypeName, errStr); err != nil {
		return fallbackToStderr(err)
	}
	return ExitCode(err)
}

// fallbackToStderr reports a failure to write to a command's own output, which
// is the one failure that output cannot report itself, on os.Stderr and returns
// the exit code to terminate with.
//
// It names xflags, unlike the messages Run prints on the command's own
// stderr, because a plain write failure says nothing about which program
// produced it. See docs/adr/human-readable-errors.md.
func fallbackToStderr(err error) int {
	fmt.Fprintf(os.Stderr, "xflags: %s\n", err)
	return ExitCodeFailure
}

// WriteUsage prints a help message to the given Writer using the configured
// Formatter.
//
// WriteUsage describes the command (see Describe) and hands the result to
// the resolved FormatFunc, so it returns the same configuration errors
// Parse would if the tree is misconfigured.
func (c *Command) WriteUsage(w io.Writer) error {
	// TODO: Usage formatting is a function of the chosen argv vocabulary
	// (POSIX/GNU, Go, Windows, etc.) so we'll need to break this API.
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

// Description specifies the prose printed at the end of this command's help
// message, after its flags and subcommands. It carries the detail that does
// not fit the one-line summary given to NewCommand.
func (c *Command) Description(s string) *Command {
	c.description = s
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

// ForwardArgs specifies that a "--" on the command line ends option
// processing, and that everything after it reaches the handler unparsed as
// Invocation.Forwarded rather than binding to positional flags.
//
// This is for a command that hands arguments on to something else -- a
// subprocess, or another parser -- which is why they are kept apart from
// the operands it consumes itself. A command that has not opted in gives
// "--" no meaning. See docs/adr/posix-argument-conventions.md.
func (c *Command) ForwardArgs() *Command {
	c.forwardArgs = true
	return c
}

// Stdin sets the source the command's handler reads as Invocation.Stdin.
//
// A nil reader inherits the stream from the command's parent, and is
// os.Stdin at the root.
func (c *Command) Stdin(r io.Reader) *Command {
	c.stdin = r
	return c
}

// Stdout sets the destination for the command's usage messages and for
// whatever its handler writes to Invocation.Stdout.
//
// A nil writer inherits the stream from the command's parent, and is
// os.Stdout at the root.
func (c *Command) Stdout(w io.Writer) *Command {
	c.stdout = w
	return c
}

// Stderr sets the destination for the command's error messages and for
// whatever its handler writes to Invocation.Stderr. Each stream is set on
// its own, so redirecting stdout still leaves errors on stderr.
//
// A nil writer inherits the stream from the command's parent, and is
// os.Stderr at the root.
func (c *Command) Stderr(w io.Writer) *Command {
	c.stderr = w
	return c
}
