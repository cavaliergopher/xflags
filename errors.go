package xflags

import (
	"errors"
	"fmt"
)

// The exit codes Run terminates with. A handler may name any other code by
// returning an error that implements ExitCoder.
const (
	ExitCodeSuccess = 0 // A handler returned nil, or help was requested.
	ExitCodeFailure = 1 // A handler returned an error.
	ExitCodeUsage   = 2 // Nothing ran: the command line or tree was wrong, or no handler.
)

// errorOrString prefers a String() method over Error(). ConfigError and
// ArgumentError give each a distinct purpose: String() is the plain
// sentence Run prints for a human, and Error() is that sentence tagged
// "xflags: ", for a Go caller that prints or logs the error itself.
func errorOrString(err error) string {
	if s, ok := err.(fmt.Stringer); ok {
		return s.String()
	}
	return err.Error()
}

// ExitCoder is an error that names the exit code a program should terminate
// with. Run looks for one in the chain of errors returned by a handler,
// using errors.As, and exits with 1 if it finds none.
//
// TIP: *exec.Error implements ExitCoder, so a handler that shells out to
// another program can return its error unchanged to exit with the same code
// as the child process.
type ExitCoder interface {
	error
	ExitCode() int
}

// ExitCode unwraps err until it finds an ExitCoder and returns its exit code.
// If none is found, it returns ExitCodeFailure.
func ExitCode(err error) int {
	var exitCoder ExitCoder
	if errors.As(err, &exitCoder) {
		return exitCoder.ExitCode()
	}
	return ExitCodeFailure
}

// Exit returns an error that reports err and asks Run to terminate the
// program with the given exit code.
//
// err may be nil to exit with a code and no explanation, in which case the
// error reads "exit status N", as an *exec.ExitError does.
func Exit(code int, err error) error {
	return &exitError{Err: err, Code: code}
}

// Exitf returns an error that reports the formatted error message and asks Run
// to terminate the program with the given exit code.
//
// Error wrapping is supported using the %w verb like fmt.Errorf.
func Exitf(code int, format string, a ...any) error {
	return Exit(code, fmt.Errorf(format, a...))
}

type exitError struct {
	Err  error
	Code int
}

func (e *exitError) Unwrap() error { return e.Err }

func (e *exitError) ExitCode() int { return e.Code }

func (e *exitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit status %d", e.Code)
	}
	return e.Err.Error()
}

// ConfigError indicates that an error was detected in the configuration of a
// command or flag before arguments could be parsed. This is a developer error
// and should be fixed in the code, not at runtime.
//
// Run reports it as "Program error: ...", which names the culprit rather
// than the fault: the reader at a terminal did not write this configuration
// and can only report it onward. See docs/adr/human-readable-errors.md.
type ConfigError struct {
	Err     error
	Cmd     *Command
	Flag    *Flag
	Message string
}

func newConfigErrorf(err error, cmd *Command, flag *Flag, format string, a ...any) *ConfigError {
	return &ConfigError{
		Err:     err,
		Cmd:     cmd,
		Flag:    flag,
		Message: fmt.Sprintf(format, a...),
	}
}

func (e *ConfigError) ExitCode() int { return ExitCodeUsage }
func (e *ConfigError) Unwrap() error { return e.Err }
func (e *ConfigError) Error() string { return "xflags: " + e.String() }

// String reports which command or flag is misconfigured, if either is
// known, followed by the message describing what's wrong with it.
func (e *ConfigError) String() string {
	switch {
	case e.Cmd != nil:
		return e.Cmd.String() + ": " + e.Message
	case e.Flag != nil:
		return e.Flag.String() + ": " + e.Message
	default:
		return e.Message
	}
}

// ArgumentError indicates that an argument specified on the command line was
// incorrect.
//
// Run reports it as "Argument error: ...", which tells the reader the command
// line is theirs to retype. Both types exit 2, so the prefix is the only thing
// distinguishing them.
type ArgumentError struct {
	Err     error
	Cmd     *Command
	Flag    *Flag
	Arg     string // The command line argument that failed validation.
	Message string
}

func newArgumentErrorf(err error, cmd *Command, flag *Flag, arg string, format string, a ...any) *ArgumentError {
	return &ArgumentError{
		Err:     err,
		Cmd:     cmd,
		Flag:    flag,
		Arg:     arg,
		Message: fmt.Sprintf(format, a...),
	}
}

func (e *ArgumentError) ExitCode() int { return ExitCodeUsage }
func (e *ArgumentError) Unwrap() error { return e.Err }
func (e *ArgumentError) Error() string { return "xflags: " + e.String() }

// String reports the message describing what was wrong with the argument,
// followed by the error it wraps, if any.
func (e *ArgumentError) String() string {
	if e.Err == nil {
		return e.Message
	}
	if e.Message == "" {
		return e.Err.Error()
	}
	return e.Message + ": " + e.Err.Error()
}
