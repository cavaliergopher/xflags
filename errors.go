package xflags

import (
	"fmt"

	"github.com/cavaliergopher/xflags/ir"
)

// The exit codes Run terminates with. A handler may name any other code by
// returning an error that implements ExitCoder.
const (
	ExitCodeSuccess = ir.ExitCodeSuccess // A handler returned nil, help included.
	ExitCodeFailure = ir.ExitCodeFailure // A handler returned an error.
	ExitCodeUsage   = ir.ExitCodeUsage   // Nothing ran: the command line or tree was wrong, or no handler.
)

// ExitCoder is an error that names the exit code a program should terminate
// with. Run looks for one in the chain of errors returned by a handler,
// using errors.As, and exits with 1 if it finds none.
//
// *exec.ExitError implements ExitCoder, so a handler that shells out can
// return its error unchanged and exit with the child's code.
type ExitCoder = ir.ExitCoder

// ExitCode unwraps err until it finds an ExitCoder and returns its exit code.
// If none is found, it returns ExitCodeFailure.
func ExitCode(err error) int {
	return ir.ExitCode(err)
}

// Exit returns an error that reports err and asks Run to terminate the
// program with the given exit code.
//
// err may be nil to exit with a code and no explanation, in which case the
// error reads "exit status N", as an *exec.ExitError does.
func Exit(code int, err error) error {
	return ir.Exit(code, err)
}

// Exitf returns an error that reports the formatted error message and asks
// Run to terminate the program with the given exit code.
//
// Error wrapping is supported using the %w verb like fmt.Errorf.
func Exitf(code int, format string, a ...any) error {
	return ir.Exitf(code, format, a...)
}

// humanMessage prefers a String() method over Error(). The two differ by
// audience, not representation: on a ConfigError or ArgumentError from the
// ir package, String() is the plain sentence Run prints for a human, and
// Error() is that sentence tagged "xflags: ", for a Go caller that prints
// or logs the error itself.
func humanMessage(err error) string {
	if s, ok := err.(fmt.Stringer); ok {
		return s.String()
	}
	return err.Error()
}
