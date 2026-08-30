package xflags

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// Run parses the arguments provided by os.Args and executes the handler for the
// command or subcommand specified by the arguments. It returns the exit code
// the program should terminate with; see Command.Run for the contract.
//
//	func main() {
//	    ctx, stop := xflags.NotifyContext(context.Background())
//	    defer stop()
//	    os.Exit(xflags.Run(ctx, cmd))
//	}
func Run(ctx context.Context, cmd *Command) int {
	return RunWithArgs(ctx, cmd, os.Args[1:]...)
}

// RunWithArgs parses the given arguments and executes the handler for the
// command or subcommand specified by the arguments. It returns the exit code
// the program should terminate with; see Command.Run for the contract.
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
