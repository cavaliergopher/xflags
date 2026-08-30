package xflags

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/cavaliergopher/xflags/internal/argv"
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

// A Middleware wraps a command's handler in another handler, so that the
// wrapper runs first and decides whether, and with what, to call the one
// it wrapped:
//
//	func timing(next xflags.HandlerFunc) xflags.HandlerFunc {
//	    return func(ctx context.Context, inv *xflags.Invocation) error {
//	        start := time.Now()
//	        err := next(ctx, inv)
//	        fmt.Fprintf(inv.Stderr, "%s took %s\n", inv.Cmd.FullName, time.Since(start))
//	        return err
//	    }
//	}
//
// A Middleware must be a pure function of the handler it is given, doing
// its work in the handler it returns rather than in the wrapper itself.
// Compile applies it while lowering, and may lower a tree more than once
// in a run, so a wrapper that registers a metric or opens a connection
// before returning does so more often than its author expects.
//
// See Command.Middleware for how one is declared and what it wraps.
type Middleware func(HandlerFunc) HandlerFunc

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
	usageFunc   ir.UsageFunc
	handlerFunc HandlerFunc
	middleware  []Middleware
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer

	// completionEnabled records that EnableCompletion was called on this
	// command, so Run consults the shell completion environment variable
	// before doing anything else. See EnableCompletion.
	completionEnabled bool

	// defaultGroup is the implicit "options" flag group that Flags appends
	// to. NewCommand creates it eagerly, so every command carries one from
	// construction; it stays out of help output regardless, since Usage
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
// if it has no parent, and reports a configuration error if the parent
// links form a cycle instead of reaching a root.
//
// Nothing else walks the source tree: a cycle is representable until
// Compile has rejected it, so every other reader takes its ancestry from
// the compiled tree. See Compile.
func (c *Command) root() (*Command, error) {
	seen := map[*Command]struct{}{c: {}}
	root := c
	for root.parent != nil {
		root = root.parent
		if _, ok := seen[root]; ok {
			return nil, ir.NewConfigErrorf(nil, nil, nil,
				"%q is its own ancestor", root.name)
		}
		seen[root] = struct{}{}
	}
	return root, nil
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
	return argv.Parse(node, args)
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
// ir.Command.Ancestry as well as its own subcommands, and configuration
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
	root, err := c.root()
	if err != nil {
		// The tree has no root to lower from, so this error stands alone
		// rather than joining the batch the rest of the run collects.
		return nil, err
	}
	nodeMap := make(map[*Command]*ir.Command)
	var errs []error
	rootNode := root.lower(nil, nil, nodeMap, &errs)
	if err := rootNode.Validate(); err != nil {
		errs = append(errs, err)
	}
	// Whether two flags collide is a question about the options they are
	// written as rather than about the model, so it is argv's to answer;
	// see internal/argv.Validate. What a name may be in the first place
	// is argv's too, and was settled during lowering, while the names
	// were still undecorated.
	if err := argv.Validate(rootNode); err != nil {
		errs = append(errs, err)
	}
	if err := ir.JoinErrors(errs); err != nil {
		return nil, err
	}
	return nodeMap[c], nil
}

// lower builds the compiled ir.Command for c and, recursively, each of its
// subcommands, copying across every
// field the ir type keeps, including those inherited from an ancestor so
// nothing about the compiled tree has to walk itself again: FullName,
// computed from parent's own FullName plus c's name, and the three
// streams and the usage renderer, each c's own where it named one and
// the parent's otherwise, and Ancestry, the parent's with c appended.
// Inheritance reads the node being built rather than the source tree's
// parent links, which are not to be trusted until Compile has checked
// them.
//
// Handler is assembled rather than copied: c's own handler is wrapped in
// the middleware c declared and in everything it inherited, which arrives
// as inherited because it is scaffolding for building Handler rather than
// anything a compiled command carries. A command that declared no handler
// gets missingSubcommand's instead, so Handler is never nil and nothing
// downstream needs to know either mechanism exists.
//
// It records the node for c in nodeMap so Compile can look up the node
// for any source *Command after lowering from the root.
//
// It also collects into errs the three configuration checks that cannot
// move onto the compiled tree, because they depend on the source tree's
// own bookkeeping rather than on anything a lowered node carries:
//
// Whether any middleware c declared is nil, which cannot be applied to a
// handler and which chainMiddleware therefore skips. Composing leaves
// nothing on the node to check afterwards, so it is checked here, where
// it is still a list of what one command declared.
//
// Whether a subcommand's parent actually names the command about to
// claim it as a child. See Subcommands for why -- a shared command such
// as xflags.CommandLine may be mounted under more than one parent, and
// only the source *Command remembers which one Subcommands actually
// accepted.
//
// Whether a subcommand has already been lowered, which means the
// subcommand links lead back into the tree above. nodeMap is the record
// of what has been visited, so the descent stops there rather than
// building nodes forever. Note that a cycle here need not be one in the
// parent links Compile checked: Subcommands leaves an owned command's
// parent alone, so a command can be mounted below its own ancestor
// without any parent link changing.
func (c *Command) lower(parent *ir.Command, inherited Middleware, nodeMap map[*Command]*ir.Command, errs *[]error) *ir.Command {
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
		Handler:     c.handlerFunc,
		UsageFunc:   c.usageFunc,
		Stdin:       c.stdin,
		Stdout:      c.stdout,
		Stderr:      c.stderr,
	}
	node.Root = node
	node.Ancestry = []*ir.Command{node}
	if parent != nil {
		node.Root = parent.Root
		// Cloned rather than appended in place, or sibling subcommands
		// would share a backing array and overwrite each other.
		node.Ancestry = append(slices.Clone(parent.Ancestry), node)
	}

	// What c named for itself is above; what it left unnamed is inherited
	// here. Each field resolves on its own, so naming one leaves the
	// others inherited, and a parent's are already resolved, so
	// inheriting ends the search.
	if parent != nil {
		if node.UsageFunc == nil {
			node.UsageFunc = parent.UsageFunc
		}
		if node.Stdin == nil {
			node.Stdin = parent.Stdin
		}
		if node.Stdout == nil {
			node.Stdout = parent.Stdout
		}
		if node.Stderr == nil {
			node.Stderr = parent.Stderr
		}
	}
	// Middleware composes where the fields above fall back: a command's
	// own wrappers run inside every one its ancestors declared, so what
	// this command adds is wrapped by what it inherited rather than
	// replacing it.
	for _, mw := range c.middleware {
		if mw == nil {
			*errs = append(*errs, ir.NewConfigErrorf(nil, node, nil,
				"middleware must not be nil"))
		}
	}
	middleware := chainMiddleware(inherited, c.middleware)

	// The handler is assembled here rather than at dispatch, so the
	// compiled command carries the whole of what it does and nothing
	// downstream has to know that middleware exists. The fallback is not
	// wrapped: there is no handler for a wrapper to wrap, and a command
	// that only groups subcommands must not run its ancestors' wrappers.
	if node.Handler == nil {
		node.Handler = missingSubcommand(node)
	} else if middleware != nil {
		node.Handler = middleware(node.Handler)
	}

	// The root has no parent to inherit from, so a stream nobody named is
	// the process's. UsageFunc has no default to fall back to here: nil
	// means the help renderer chooses one when it prints.
	if node.Stdin == nil {
		node.Stdin = os.Stdin
	}
	if node.Stdout == nil {
		node.Stdout = os.Stdout
	}
	if node.Stderr == nil {
		node.Stderr = os.Stderr
	}
	nodeMap[c] = node

	// Own groups first, then every mounted set, matching the order
	// effectiveGroups reports and marking which is which -- validation
	// needs to tell a declared flag from a mounted one.
	for _, group := range c.flagGroups {
		node.FlagGroups = append(node.FlagGroups, group.lower(false, errs))
	}
	for _, set := range c.groupSets {
		for _, group := range set.groups {
			node.FlagGroups = append(node.FlagGroups, group.lower(true, errs))
		}
	}
	for _, sub := range c.subcommands {
		if prev, ok := nodeMap[sub]; ok {
			*errs = append(*errs, ir.NewConfigErrorf(nil, node, nil,
				"%q is already mounted at %q", sub.name, prev.FullName))
			continue
		}
		// Subcommands leaves an already-parented command's parent alone
		// rather than stealing it, so the mismatch is still visible here
		// to report.
		if sub.parent != c {
			*errs = append(*errs, ir.NewConfigErrorf(nil, node, nil,
				"%q is already a subcommand of %q", sub.name, sub.parent.name))
		}
		node.Subcommands = append(node.Subcommands, sub.lower(node, middleware, nodeMap, errs))
	}
	return node
}

// chainMiddleware composes outer, the middleware a command inherits from
// its parent, with own, the middleware that command declared, into the
// single Middleware the compiled command carries.
//
// Applying the result wraps a handler so that outer runs first, then
// own's entries in the order they were declared, then the handler itself,
// each resuming in reverse as the call returns. The outermost wrapper is
// therefore the one declared highest in the tree, and earliest in its
// command's Middleware call.
//
// A command that adds nothing inherits outer unchanged, so a path that
// declared no middleware at all composes to nil and costs no call.
//
// A nil entry in own is skipped rather than applied. Lowering reports one
// as a configuration error and then has to finish, so that the rest of
// the tree's errors are collected in the same run; the tree it composes
// meanwhile is never dispatched.
func chainMiddleware(outer Middleware, own []Middleware) Middleware {
	if len(own) == 0 {
		return outer
	}
	return func(next HandlerFunc) HandlerFunc {
		// Built inside out, which reads backwards from the order above:
		// the wrapper applied last ends up outermost, and so runs first.
		for _, mw := range slices.Backward(own) {
			if mw == nil {
				continue
			}
			next = mw(next)
		}
		if outer != nil {
			next = outer(next)
		}
		return next
	}
}

// missingSubcommand returns the handler lowering gives a command that
// declared none of its own. Such a command exists only to group its
// subcommands, so naming it alone is a usage error rather than a request
// for help, and reporting that is the whole of what it does.
//
// It names cmd rather than the invocation's command so that the error
// names the command that lacked the handler, whoever ran it.
func missingSubcommand(cmd *ir.Command) HandlerFunc {
	return func(ctx context.Context, inv *Invocation) error {
		return ir.NewArgumentErrorf(nil, cmd, nil, "", "missing subcommand")
	}
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
// A malformed command tree is the exception: it is reported on os.Stderr
// whatever the tree configured, because the configuration is part of what
// failed to validate. Compile reports the same faults as an ordinary
// error value, which is where a program is best served catching them.
//
// Run is Dispatch plus this reporting; a program that wants to report
// errors its own way calls Dispatch instead and keeps the raw error.
//
// If EnableCompletion was called, Run first checks the shell completion
// environment variable it names and, when it carries a value Run
// recognizes, answers it directly instead of parsing args at all; see
// EnableCompletion.
//
// ctx is passed to the handler unchanged. See NotifyContext for a context
// that is canceled on SIGINT or SIGTERM.
func (c *Command) Run(ctx context.Context, args []string) int {
	return RunWithArgs(ctx, c, args...)
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
// UsageFunc.
//
// A command invoked with no handler returns an *ir.ArgumentError, as for
// any other wrong command line, and no middleware runs: there is no
// handler for it to wrap. Otherwise the handler runs inside every
// middleware declared on the command and its ancestors, outermost first,
// Compile having wrapped it while lowering. See Command.Middleware.
func (c *Command) Dispatch(ctx context.Context, args []string) error {
	node, err := c.Compile()
	if err != nil {
		return err
	}
	return dispatch(ctx, node, args)
}

// dispatch is Command.Dispatch against a tree that is already compiled,
// which is what lets RunWithArgs lower the tree once and hand the same
// node to everything below it. See runCompiled.
func dispatch(ctx context.Context, cmd *ir.Command, args []string) error {
	inv, err := argv.Parse(cmd, args)
	if err != nil {
		return err
	}
	if inv.HelpRequested {
		return inv.Cmd.Usage(inv.Stdout)
	}
	return inv.Cmd.Handler(ctx, inv)
}

// runCompiled runs an already-compiled command tree and returns the exit
// code the program should terminate with. It is Run's whole body past the
// compile, and the seam a precompiled entry point would be exported at if
// one is ever wanted; see RunWithArgs.
func runCompiled(ctx context.Context, cmd *ir.Command, args ...string) int {
	return report(cmd, dispatch(ctx, cmd, args))
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
	return argv.Complete(node, args, word)
}

// report reports err against cmd, an already-compiled tree, and returns
// the exit code the program should terminate with. A joined error -- a
// wrong command line can report more than one fault -- prints one
// prefixed line per error.
//
// A line goes to the stderr of the command the error names, or to cmd's
// when it names none. An argument error is followed by that command's
// usage, so the reader who mistyped sees what to type instead; see
// docs/adr/argument-errors-print-usage.md.
//
// A configuration error is the exception, and reaches here only from a
// tree that compiled and then failed anyway -- restoring a default
// through Set is the one way that happens. The fault is the program's
// rather than the user's, so it goes to os.Stderr whatever the tree
// configured and prints no usage. A tree that failed to compile at all
// never reaches here; see reportConfigError.
func report(cmd *ir.Command, err error) int {
	if err == nil {
		return ExitCodeSuccess
	}

	for _, e := range ir.FlattenErrors(err) {
		errTypeName := "Error"
		stderr := cmd.Stderr

		var argErr *ir.ArgumentError
		var cfgErr *ir.ConfigError
		switch {
		case errors.As(e, &argErr):
			errTypeName = "Argument error"
			if argErr.Cmd != nil {
				stderr = argErr.Cmd.Stderr
			}

		case errors.As(e, &cfgErr):
			// Both exit 2, so the prefix is all that says whether the
			// fault was the user's or the program's.
			errTypeName = "Program error"
			stderr = os.Stderr
		}

		// A config error is already on os.Stderr, which is where the
		// fallback would write, so a failure there has nowhere left to go
		// and must not cost the exit code that says the program is
		// malformed.
		_, werr := fmt.Fprintf(stderr, "%s: %s\n", errTypeName, humanMessage(e))
		if werr != nil && cfgErr == nil {
			return fallbackToStderr(werr)
		}
		if argErr != nil {
			usageCmd := argErr.Cmd
			if usageCmd == nil {
				// The error names no command, so the command that was run
				// describes itself instead.
				usageCmd = cmd
			}
			if werr := usageCmd.Usage(stderr); werr != nil {
				return fallbackToStderr(werr)
			}
		}
	}
	return ExitCode(err)
}

// reportConfigError reports a command tree that failed to compile and
// returns the exit code the program should terminate with.
//
// It is the one report that does not consult the tree, because the tree
// is what failed: a command's stream overrides are inherited along the
// parent links Compile checks, so nothing it says about where its output
// goes can be trusted, and the message goes to os.Stderr whatever it
// configured. No usage follows, because a malformed tree cannot describe
// itself. Compile reports the same faults as an ordinary error value,
// which is where a program is best served catching them.
func reportConfigError(err error) int {
	for _, e := range ir.FlattenErrors(err) {
		// A failure to write here has nowhere left to go: os.Stderr is
		// already where the fallback would write, and it must not cost
		// the exit code that says the program is malformed.
		fmt.Fprintf(os.Stderr, "Program error: %s\n", humanMessage(e))
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
//
// The handler runs inside whatever middleware the command and its
// ancestors declared, Compile having wrapped it while lowering the tree;
// see Command.Middleware.
func (c *Command) HandleFunc(handler HandlerFunc) *Command {
	c.handlerFunc = handler
	return c
}

// Middleware wraps the handler of this command, and of every command
// beneath it, in each of the given wrappers. It is where whatever a whole
// subtree has to do belongs -- an authorization check, a timing trace,
// opening a resource and closing it again -- written once instead of at
// the top of every handler.
//
//	var App = xflags.NewCommand("myapp", "Do things").
//	    Middleware(authorize, trace).
//	    Subcommands(GetCommand, DeleteCommand)
//
// Middleware is inherited down the command path, and the outermost
// wrapper is the one declared highest in the tree: a middleware on the
// root runs before one a subcommand declared, and within one command they
// run in the order given here. Repeated calls append.
//
// A wrapper runs only around a handler, and only once the command line
// has parsed, so flag values are set by the time it is called and it may
// read them. Neither -h nor --help, an unparsable command line, nor
// naming a command that only groups subcommands reaches one, because none
// of them runs a handler.
//
// A wrapper decides whether to call the handler it wrapped, so returning
// an error without calling it is how one refuses an invocation, and Run
// maps that error to an exit code as it would the handler's own.
//
// Each wrapper must be a pure function of the handler it is given, doing
// its work in the handler it returns: Compile applies them while lowering
// the tree, and lowers a tree more than once in a run. See Middleware for
// a worked wrapper, and Command.HandleFunc for the handler itself.
func (c *Command) Middleware(mw ...Middleware) *Command {
	c.middleware = append(c.middleware, mw...)
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

// UsageFunc specifies a custom renderer for this command's help messages,
// in place of the default.
func (c *Command) UsageFunc(fn ir.UsageFunc) *Command {
	c.usageFunc = fn
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
