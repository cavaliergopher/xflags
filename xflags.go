package xflags

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cavaliergopher/xflags/internal/argv"
	"github.com/cavaliergopher/xflags/ir"
)

// Run parses the arguments provided by os.Args and executes the handler for
// the command or subcommand specified by the arguments. It returns the exit
// code the program should terminate with; see RunWithArgs for the contract.
//
//	func main() {
//	    ctx, stop := xflags.NotifyContext(context.Background())
//	    defer stop()
//	    os.Exit(xflags.Run(ctx, cmd))
//	}
func Run(ctx context.Context, cmd *Command) int {
	return RunWithArgs(ctx, cmd, os.Args[1:]...)
}

// RunWithArgs parses the given set of command line arguments and calls the
// handler for the command or subcommand specified by the arguments. It
// returns the exit code the program should terminate with, which follows a
// three-value contract:
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
// error names, or of cmd when the error names none, and an argument error
// -- a wrong command line, including a command invoked with no handler --
// is followed by that command's usage. The handler is given the same
// streams on its Invocation. See Command.Stdin, Command.Stdout and
// Command.Stderr.
//
// A malformed command tree is the exception: it is reported on os.Stderr
// whatever the tree configured, because the configuration is part of what
// failed to validate. Command.Compile reports the same faults as an
// ordinary error value, which is where a program is best served catching
// them.
//
// RunWithArgs is Dispatch plus this reporting; a program that wants to
// report errors its own way calls Dispatch instead and keeps the raw
// error.
//
// If EnableCompletion was called on cmd, RunWithArgs first checks the shell
// completion environment variable it names and, when it carries a value
// RunWithArgs recognizes, answers it directly instead of parsing args at
// all; see Command.EnableCompletion.
//
// ctx is passed to the handler unchanged. See NotifyContext for a context
// that is canceled on SIGINT or SIGTERM.
//
//	func main() {
//	    os.Exit(xflags.RunWithArgs(context.Background(), cmd, "--foo", "--bar"))
//	}
func RunWithArgs(ctx context.Context, cmd *Command, args ...string) int {
	// The one place an ordinary invocation compiles the tree. Everything
	// below takes the compiled node, so lowering happens once however
	// many steps consult it.
	node, err := cmd.Compile()
	if err != nil {
		return reportConfigError(err)
	}
	if cmd.completionEnabled {
		if code, handled := completionHook(node); handled {
			return code
		}
	}
	return runCompiled(ctx, node, args...)
}

// Dispatch compiles the whole command tree cmd belongs to (see
// Command.Compile), parses the given set of command line arguments against
// it, and calls the handler for the command or subcommand specified by the
// arguments. The handler's error, or the error that stopped the command
// line from being parsed, is returned raw: Dispatch prints no error text,
// so a program that wants to report errors its own way calls Dispatch
// directly. RunWithArgs is Dispatch plus the reporting and the mapping to
// an exit code.
//
// If -h or --help are specified, no handler runs: usage information is
// printed to the command's stdout and Dispatch returns nil, or the error
// that kept the help message from being written. Help output is not error
// reporting, and a caller who wants it formatted differently has
// Command.UsageFunc.
//
// A command invoked with no handler returns an *ir.ArgumentError, as for
// any other wrong command line, and no middleware runs: there is no
// handler for it to wrap. Otherwise the handler runs inside every
// middleware declared on the command and its ancestors, outermost first,
// compiling having wrapped it while lowering. See Command.Middleware.
func Dispatch(ctx context.Context, cmd *Command, args ...string) error {
	node, err := cmd.Compile()
	if err != nil {
		return err
	}
	return dispatch(ctx, node, args)
}

// Parse compiles the whole command tree cmd belongs to (see
// Command.Compile) and parses the given set of command line arguments
// against it, storing the value of each argument in each command flag's
// target. The rules for each flag are checked and any errors are returned.
//
// Parse resets every flag to its default before reading any arguments, so
// parsing the same tree twice yields the same result.
//
// The returned Invocation names cmd, or one of its subcommands if the
// arguments specified one.
//
// If -h or --help are specified, parsing stops there and the returned
// Invocation has HelpRequested set. That is not an error: it is for the
// caller to report the command's usage. See RunWithArgs.
func Parse(cmd *Command, args ...string) (*Invocation, error) {
	node, err := cmd.Compile()
	if err != nil {
		return nil, err
	}
	return argv.Parse(node, args)
}

// Complete resolves shell completion candidates for a command line that is
// still being typed. args is the command line so far, excluding the
// program name and the word currently under the cursor; word is that
// fragment, possibly empty.
//
// Complete compiles cmd (see Command.Compile) and delegates to the compiled
// tree. On a configuration error it returns no candidates and
// ir.CompNoFileComp, the same best-effort contract as a broken command
// line: a misconfigured tree cannot say what would complete it.
func Complete(cmd *Command, args []string, word string) ([]string, ir.CompDirective) {
	node, err := cmd.Compile()
	if err != nil {
		return nil, ir.CompNoFileComp
	}
	return argv.Complete(node, args, word)
}

// dispatch is Dispatch against a tree that is already compiled, which is
// what lets RunWithArgs lower the tree once and hand the same node to
// everything below it. See runCompiled.
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
// code the program should terminate with. It is RunWithArgs's whole body
// past the compile, and the seam a precompiled entry point would be
// exported at if one is ever wanted.
func runCompiled(ctx context.Context, cmd *ir.Command, args ...string) int {
	return report(cmd, dispatch(ctx, cmd, args))
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
				// The error names no command, so the command RunWithArgs was
				// given describes itself instead.
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

// NotifyContext returns a copy of parent that is canceled when the program
// is interrupted, on SIGINT or SIGTERM, and a stop function that releases
// the signal handler.
//
// Unlike signal.NotifyContext, default signal handling is restored once the
// context is canceled, so a second interrupt terminates the program even if
// it is wedged. This is the behavior a user expects of a command line
// program: the first interrupt asks it to stop, and the second insists.
//
// Programs that call os.Exit skip deferred calls, so stop may never run.
// That is harmless -- the process is exiting -- and deferring it keeps the
// context released if the program later grows another path out of main.
func NotifyContext(parent context.Context) (ctx context.Context, stop func()) {
	ctx, cancel := context.WithCancel(parent)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-ch:
		case <-ctx.Done():
		}
		// However this ended -- a signal, stop, or a canceled parent --
		// stop relaying signals, which restores their default disposition
		// so a second one kills a process that did not stop on the first.
		signal.Stop(ch)
		cancel()
	}()
	return ctx, func() {
		signal.Stop(ch)
		cancel()
	}
}
