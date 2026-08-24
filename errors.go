package xflags

import (
	"bytes"
	"errors"
	"fmt"
)

// The exit codes Run terminates with. A handler may name any other code by
// returning an error that implements ExitCoder.
const (
	exitSuccess = 0 // the handler returned nil, or help was requested
	exitFailure = 1 // the handler returned an error
	exitUsage   = 2 // the command line was wrong
)

// ExitCoder is an error that names the exit code a program should terminate
// with. Run looks for one in the chain of errors returned by a handler,
// using errors.As, and exits with 1 if it finds none.
//
// *exec.ExitError implements ExitCoder, so a handler that shells out to
// another program can return its error unchanged to exit with the same code
// as the child process.
type ExitCoder interface {
	error
	ExitCode() int
}

// exitCode returns the exit code named by err, or exitFailure if err names
// none.
func exitCode(err error) int {
	var coder ExitCoder
	if errors.As(err, &coder) {
		return coder.ExitCode()
	}
	return exitFailure
}

// exitError is an error that names its own exit code, as returned by Exit.
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

func (e *exitError) String() string {
	if e.Err == nil {
		return fmt.Sprintf("exit status %d", e.Code)
	}
	return errStr(e.Err)
}

// Exit returns an error that reports err and asks Run to terminate the
// program with the given exit code.
//
// err may be nil to exit with a code and no explanation, in which case the
// error reads "exit status N", as an *exec.ExitError does.
func Exit(err error, code int) error {
	return &exitError{Err: err, Code: code}
}

// UsageErrorf returns an error reporting that a command was invoked
// incorrectly in a way the parser cannot detect, such as two flags that
// contradict each other. Run reports it and exits with the same code it uses
// for a command line it could not parse.
func UsageErrorf(format string, a ...any) error {
	return Exit(fmt.Errorf(format, a...), exitUsage)
}

type xflagsErr struct {
	Text string
	Err  error
}

func (e *xflagsErr) Unwrap() error { return e.Err }

// ExitCode reports a configuration error as a usage error rather than a
// handler failure. A malformed tree is decided before the handler runs,
// which is what code 2 covers.
func (e *xflagsErr) ExitCode() int { return exitUsage }

func (e *xflagsErr) Error() string { return "xflags: " + e.String() }

func (e *xflagsErr) String() string {
	w := new(bytes.Buffer)
	if e.Text != "" {
		fmt.Fprint(w, e.Text)
	}
	if e.Text != "" && e.Err != nil {
		fmt.Fprint(w, ": ")
	}
	if e.Err != nil {
		fmt.Fprint(w, errStr(e.Err))
	}
	return w.String()
}

func errorf(format string, a ...any) error {
	return &xflagsErr{Text: fmt.Sprintf(format, a...)}
}

// HelpError is the error returned if the -h or --help argument is specified
// but no such flag is explicitly defined.
type HelpError struct {
	Cmd *Command // The command that was invoked and produced this error.
}

func (err *HelpError) Error() string {
	return fmt.Sprintf("xflags: help requested: %s", err.Cmd)
}

// ArgumentError indicates that an argument specified on the command line was
// incorrect.
type ArgumentError struct {
	Text string
	Err  error
	Cmd  *Command
	Flag *Flag
	Arg  string
}

func (e *ArgumentError) Unwrap() error { return e.Err }

func (e *ArgumentError) Error() string { return "xflags: " + e.String() }

func (e *ArgumentError) String() string {
	w := new(bytes.Buffer)
	if e.Flag != nil {
		fmt.Fprintf(w, "%s: ", e.Flag)
	}
	if e.Text != "" {
		fmt.Fprint(w, e.Text)
	}
	if e.Text != "" && e.Err != nil {
		fmt.Fprint(w, ": ")
	}
	if e.Err != nil {
		fmt.Fprint(w, errStr(e.Err))
	}
	return w.String()
}

func newArgErr(
	cmd *Command,
	flag *Flag,
	arg string,
	format string,
	a ...any,
) *ArgumentError {
	if cmd == nil {
		panic("developer error: cmd cannot be nil")
	}
	e := wrapArgErr(nil, cmd, flag, arg)
	e.Text = fmt.Sprintf(format, a...)
	return e
}

func wrapArgErr(err error, cmd *Command, flag *Flag, arg string) *ArgumentError {
	return &ArgumentError{
		Err:  err,
		Cmd:  cmd,
		Flag: flag,
		Arg:  arg,
	}
}

func errStr(err error) string {
	if s, ok := err.(fmt.Stringer); ok {
		return s.String()
	}
	return err.Error()
}
