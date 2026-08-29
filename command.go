package xflags

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/cavaliergopher/xflags/ir"
)

// An Invocation is the result of parsing a command line. It records which
// command the arguments named, what was left for its handler, and the
// streams the handler should read and write.
type Invocation = ir.Invocation

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
type HandlerFunc = ir.HandlerFunc

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
	groupSets   []*GroupSet
	subcommands []*Command
	formatFunc  ir.FormatFunc
	handlerFunc HandlerFunc
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer

	// completionEnabled records that EnableCompletion was called on this
	// command, so Run consults the shell completion environment variable
	// before doing anything else. See EnableCompletion.
	completionEnabled bool

	// defaultGroup is the implicit "options" flag group that Flags appends
	// to. NewCommand creates it eagerly, so every command carries one from
	// construction; it stays out of help output regardless, since Format
	// skips a group with no flags in it.
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

// root returns the root of the command tree c belongs to, which is c itself
// if it has no parent.
func (c *Command) root() *Command {
	root := c
	for root.parent != nil {
		root = root.parent
	}
	return root
}

// Parse compiles the whole command tree c belongs to (see Compile) and
// parses the given set of command line arguments against it, storing the
// value of each argument in each command flag's target. The rules for each
// flag are checked and any errors are returned.
//
// Parse resets every flag to its default before reading any arguments, so
// parsing the same tree twice yields the same result.
//
// The returned Invocation names this command, or one of its subcommands if
// the arguments specified one.
//
// If -h or --help are specified, parsing stops there and the returned
// Invocation has HelpRequested set. That is not an error: it is for the
// caller to report the command's usage. See Command.Run.
func (c *Command) Parse(args []string) (*Invocation, error) {
	node, err := c.Compile()
	if err != nil {
		return nil, err
	}
	return node.Parse(args)
}

// validate compiles the whole command tree c belongs to and reports only
// whether it is valid, discarding the compiled tree. See Compile.
func (c *Command) validate() error {
	_, err := c.Compile()
	return err
}

// effectiveGroups returns the flag groups the command presents: its own,
// followed by every group of every mounted GroupSet, in registration
// order. Validation, parsing and Compile all read flags through this
// helper, so a mounted flag behaves exactly like a declared one. The
// result is assembled afresh on each call, never written back into
// flagGroups: that is what keeps Compile pure and a repeated Parse from
// mounting the same groups twice.
func (c *Command) effectiveGroups() []*FlagGroup {
	if len(c.groupSets) == 0 {
		return c.flagGroups
	}
	groups := make([]*FlagGroup, len(c.flagGroups), len(c.flagGroups)+len(c.groupSets))
	copy(groups, c.flagGroups)
	for _, set := range c.groupSets {
		groups = append(groups, set.groups...)
	}
	return groups
}

// Compile lowers the whole command tree that c belongs to into its
// compiled ir.Command form and returns the node corresponding to c's
// position within it. It is not limited to c: the tree is found by walking
// to its root, so the returned node carries complete ancestry via
// ir.Command.Parent as well as its own subcommands, and configuration
// errors anywhere in the tree are reported -- including in commands
// unrelated to c. This is what makes a compiled command correct: a
// subcommand's usage line, inherited flags and environment variables are
// all meaningless without its ancestors.
//
// The errors returned are the same configuration errors Parse returns,
// which likewise compiles from the root wherever it is called.
//
// Compile is pure: it does not mutate the command tree or the variables
// flags are bound to, so it is safe to call at any time, including while
// parsed values are live. There is no caching, so every call lowers,
// validates and returns the tree afresh.
func (c *Command) Compile() (*ir.Command, error) {
	root := c.root()
	nodeMap := make(map[*Command]*ir.Command)
	var errs []error
	rootNode := root.lower(nil, nodeMap, &errs)
	if err := rootNode.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := ir.JoinErrors(errs); err != nil {
		return nil, err
	}
	return nodeMap[c], nil
}

// lower builds the compiled ir.Command for c and, recursively, each of its
// subcommands, setting parent as its Parent and copying across every
// field the ir type keeps, including the three resolved here so nothing
// about the compiled tree has to walk itself again: FullName, computed
// from parent's own FullName plus c's name, and the three streams, read
// once through getStdin/getStdout/getStderr. It records the node for c in
// nodeMap so Compile can look up the node for any source *Command after
// lowering from the root.
//
// It also collects into errs the one configuration check that cannot move
// onto the compiled tree, because it depends on the source tree's own
// bookkeeping rather than anything a lowered node carries: whether a
// subcommand's parent actually names the command about to claim it as a
// child. See Subcommands for why -- a shared command such as
// xflags.CommandLine may be mounted under more than one parent, and only
// the source *Command remembers which one Subcommands actually accepted.
func (c *Command) lower(parent *ir.Command, nodeMap map[*Command]*ir.Command, errs *[]error) *ir.Command {
	fullName := c.name
	if parent != nil {
		fullName = parent.FullName + " " + c.name
	}
	node := &ir.Command{
		Name:        c.name,
		Summary:     c.summary,
		Description: c.description,
		Hidden:      c.hidden,
		ForwardArgs: c.forwardArgs,
		FullName:    fullName,
		Parent:      parent,
		Handler:     c.handlerFunc,
		FormatFunc:  c.formatFunc,
		Stdin:       c.getStdin(),
		Stdout:      c.getStdout(),
		Stderr:      c.getStderr(),
	}
	node.Root = node
	if parent != nil {
		node.Root = parent.Root
	}
	nodeMap[c] = node

	// Own groups first, then every mounted set, matching the order
	// effectiveGroups reports and marking which is which -- validation
	// needs to tell a declared flag from a mounted one.
	for _, group := range c.flagGroups {
		node.FlagGroups = append(node.FlagGroups, group.lower(false))
	}
	for _, set := range c.groupSets {
		for _, group := range set.groups {
			node.FlagGroups = append(node.FlagGroups, group.lower(true))
		}
	}
	for _, sub := range c.subcommands {
		// Subcommands leaves an already-parented command's parent alone
		// rather than stealing it, so the mismatch is still visible here
		// to report.
		if sub.parent != c {
			*errs = append(*errs, ir.NewConfigErrorf(nil, node, nil,
				"%q is already a subcommand of %q", sub.name, sub.parent.name))
		}
		node.Subcommands = append(node.Subcommands, sub.lower(node, nodeMap, errs))
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
// command's stdout. Errors are reported on the stderr of the command the
// error names, or of the command Run was called on when the error names
// none, and an argument error -- a wrong command line, including a command
// invoked with no handler -- is followed by that command's usage. The
// handler is given the same streams on its Invocation. See Stdin, Stdout
// and Stderr.
//
// Run is Dispatch plus this reporting; a program that wants to report
// errors its own way calls Dispatch instead and keeps the raw error.
//
// If EnableCompletion was called, Run first checks the shell completion
// environment variable it names and, when it carries a value Run
// recognizes, answers it directly instead of parsing args at all; see
// EnableCompletion. RunWithArgs shares this behavior, being built on Run.
//
// ctx is passed to the handler unchanged. See NotifyContext for a context
// that is canceled on SIGINT or SIGTERM.
func (c *Command) Run(ctx context.Context, args []string) int {
	if c.completionEnabled {
		if code, handled := completionHook(c); handled {
			return code
		}
	}
	return c.handleErr(c.Dispatch(ctx, args))
}

// Dispatch compiles the whole command tree c belongs to (see Compile),
// parses the given set of command line arguments against it, and calls the
// handler for the command or subcommand specified by the arguments. The
// handler's error, or the error that stopped the command line from being
// parsed, is returned raw: Dispatch prints no error text, so a program
// that wants to report errors its own way calls Dispatch directly. Run is
// Dispatch plus the reporting and the mapping to an exit code.
//
// If -h or --help are specified, no handler runs: usage information is
// printed to the command's stdout and Dispatch returns nil, or the error
// that kept the help message from being written. Help output is not error
// reporting, and a caller who wants it formatted differently has
// FormatFunc.
//
// A command invoked with no handler returns an *ir.ArgumentError, as for
// any other wrong command line.
func (c *Command) Dispatch(ctx context.Context, args []string) error {
	node, err := c.Compile()
	if err != nil {
		return err
	}
	return node.Dispatch(ctx, args)
}

// WriteUsage prints a help message to the given Writer using the configured
// Formatter.
//
// WriteUsage compiles the command (see Compile) and hands the result to
// the resolved FormatFunc, so it returns the same configuration errors
// Parse would if the tree is misconfigured.
func (c *Command) WriteUsage(w io.Writer) error {
	node, err := c.Compile()
	if err != nil {
		return err
	}
	return node.WriteUsage(w)
}

// Complete resolves shell completion candidates for a command line that is
// still being typed. args is the command line so far, excluding the
// program name and the word currently under the cursor; word is that
// fragment, possibly empty.
//
// Complete compiles the command (see Compile) and delegates to the
// compiled tree. On a configuration error it returns no candidates and
// ir.CompNoFileComp, the same best-effort contract as a broken command
// line: a misconfigured tree cannot say what would complete it.
func (c *Command) Complete(args []string, word string) ([]string, ir.CompDirective) {
	node, err := c.Compile()
	if err != nil {
		return nil, ir.CompNoFileComp
	}
	return node.Complete(args, word)
}

// handleErr reports err on the stderr of the command that produced it --
// named by the error's Cmd field when it carries one, the receiver
// otherwise -- and returns the exit code the program should terminate
// with. A joined error -- validation reports every configuration error in
// one run -- prints one prefixed line per error, each on its own command's
// stderr. An argument error is followed by that command's usage, so the
// reader who mistyped sees what to type instead; a config error is not,
// because a malformed tree cannot describe itself. See
// docs/adr/argument-errors-print-usage.md.
func (c *Command) handleErr(err error) int {
	if err == nil {
		return ExitCodeSuccess
	}

	for _, e := range ir.FlattenErrors(err) {
		errTypeName := "Error"
		stderr := c.getStderr()
		writeUsage := c.WriteUsage

		var argErr *ir.ArgumentError
		var cfgErr *ir.ConfigError
		switch {
		case errors.As(e, &argErr):
			errTypeName = "Argument error"
			if argErr.Cmd != nil {
				stderr = argErr.Cmd.Stderr
				writeUsage = argErr.Cmd.WriteUsage
			}

		case errors.As(e, &cfgErr):
			// The tree is malformed, so the fault is the program's rather than
			// the user's. Both exit 2, so the prefix is all that says which.
			errTypeName = "Program error"
			if cfgErr.Cmd != nil {
				stderr = cfgErr.Cmd.Stderr
			}
		}

		if _, werr := fmt.Fprintf(stderr, "%s: %s\n", errTypeName, humanMessage(e)); werr != nil {
			return fallbackToStderr(werr)
		}
		if argErr != nil {
			if werr := writeUsage(stderr); werr != nil {
				return fallbackToStderr(werr)
			}
		}
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

// Description specifies the prose printed at the end of this command's help
// message, after its flags and subcommands. It carries the detail that does
// not fit the one-line summary given to NewCommand.
func (c *Command) Description(s string) *Command {
	c.description = s
	return c
}

// HandleFunc registers the handler for the command. If no handler is
// specified and the command is invoked, Run reports an argument error
// followed by the command's usage and exits with the usage error code.
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

// Flags appends command line flags to the implicit "options" flag group
// every command carries from construction.
func (c *Command) Flags(flags ...*Flag) *Command {
	if c.defaultGroup == nil {
		c.defaultGroup = &FlagGroup{name: "options", title: "Options"}
		c.flagGroups = append(c.flagGroups, c.defaultGroup)
	}
	c.defaultGroup.flags = append(c.defaultGroup.flags, flags...)
	return c
}

// FlagGroups adds groups of command line flags, built with NewFlagGroup or
// FromFlagSet, to this command, showing each under its own heading in help
// messages.
func (c *Command) FlagGroups(groups ...*FlagGroup) *Command {
	c.flagGroups = append(c.flagGroups, groups...)
	return c
}

// GroupSets mounts every flag group registered in each of the given sets
// on this command, in the order given, after the command's own groups.
// Mount CommandLine to pick up everything the program's libraries
// registered:
//
//	var App = xflags.NewCommand("myapp", "").GroupSets(xflags.CommandLine)
//
// A set is read when the tree is parsed or compiled, not when GroupSets
// is called, so a group registered afterwards is still seen. Mounted
// flags validate, parse and print exactly like the command's own, each
// group under its own heading in help messages.
func (c *Command) GroupSets(sets ...*GroupSet) *Command {
	c.groupSets = append(c.groupSets, sets...)
	return c
}

// Subcommands adds subcommands to this command and sets their parent to
// this command, unless a command given here already has one -- typically a
// command already mounted elsewhere, such as xflags.CommandLine -- in
// which case its existing parent is left alone and validation reports the
// mismatch; see lower.
func (c *Command) Subcommands(cmds ...*Command) *Command {
	c.subcommands = append(c.subcommands, cmds...)
	for _, cmd := range cmds {
		if cmd.parent == nil {
			cmd.parent = c
		}
	}
	return c
}

// FormatFunc specifies a custom FormatFunc for formatting help messages for
// this command.
func (c *Command) FormatFunc(fn ir.FormatFunc) *Command {
	c.formatFunc = fn
	return c
}

// EnableCompletion opts this command into shell completion. Call it on the
// command Run or RunWithArgs will be called on -- typically the root --
// since that is where the environment variable it enables is named from.
//
// Without EnableCompletion, Run's behavior is unchanged. With it, Run
// checks one environment variable before doing anything else: the
// command's own name, uppercased, with every rune that is not a letter or
// digit replaced by "_", followed by "_COMPLETE" -- "myapp" becomes
// MYAPP_COMPLETE, "my-app" becomes MY_APP_COMPLETE. A recognized value
// answers a shell's request directly: printing a completion script, or a
// completion reply, without invoking any handler or examining args. Any
// other value, including the variable being unset, leaves Run's behavior
// exactly as if EnableCompletion had not been called.
func (c *Command) EnableCompletion() *Command {
	c.completionEnabled = true
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
