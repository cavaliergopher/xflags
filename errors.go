package xflags

import (
	"fmt"
	"os"
)

type argError struct {
	Message  string
	ExitCode int
}

func (err *argError) Error() string {
	return err.Message
}

func newArgError(exitCode int, format string, a ...interface{}) *argError {
	return &argError{
		Message:  fmt.Sprintf(format, a...),
		ExitCode: exitCode,
	}
}

func handleErr(err error) int {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	if argErr, ok := err.(*argError); ok {
		return argErr.ExitCode
	}
	return 1
}
