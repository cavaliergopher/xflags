package xflags

import (
	"fmt"
)

type xflagsErr string

func (err xflagsErr) Error() string { return "xflags: " + string(err) }

func errorf(format string, a ...interface{}) xflagsErr {
	return xflagsErr(fmt.Sprintf(format, a...))
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
	Msg  string
	Cmd  *Command
	Flag *Flag
	Arg  string
}

func (err *ArgumentError) Error() string { return "xflags: " + err.Msg }

func newArgErr(
	cmd *Command,
	flagInfo *Flag,
	arg string,
	format string,
	a ...interface{},
) *ArgumentError {
	return &ArgumentError{
		Msg:  fmt.Sprintf(format, a...),
		Cmd:  cmd,
		Flag: flagInfo,
		Arg:  arg,
	}
}
