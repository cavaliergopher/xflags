package xflags

import (
	"bytes"
	"fmt"
)

type xflagsErr struct {
	Text string
	Err  error
}

func (e *xflagsErr) Unwrap() error { return e.Err }

func (e *xflagsErr) Error() string { return "xflags: " + e.String() }

func (e *xflagsErr) String() string {
	w := new(bytes.Buffer)
	if e.Text != "" {
		fmt.Fprintf(w, e.Text)
	}
	if e.Text != "" && e.Err != nil {
		fmt.Fprintf(w, ": ")
	}
	if e.Err != nil {
		fmt.Fprintf(w, ": %s", errStr(e.Err))
	}
	return w.String()
}

func errorf(format string, a ...interface{}) error {
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
		fmt.Fprintf(w, e.Text)
	}
	if e.Text != "" && e.Err != nil {
		fmt.Fprintf(w, ": ")
	}
	if e.Err != nil {
		fmt.Fprintf(w, "%s", errStr(e.Err))
	}
	return w.String()
}

func newArgErr(
	cmd *Command,
	flag *Flag,
	arg string,
	format string,
	a ...interface{},
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
