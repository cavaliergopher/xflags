package xflags

import (
	"fmt"
	"os"
)

func handleErr(err error) int {
	if err == nil {
		return 0
	}
	var msg string
	if xErr, ok := err.(xflagsErr); ok {
		msg = xErr.String()
	} else {
		msg = err.Error()
	}
	fmt.Fprintf(os.Stderr, "Error: %v\n", msg)
	return 1
}

type xflagsErr string

func (err xflagsErr) Error() string { return "xflags: " + err.String() }

func (err xflagsErr) String() string { return string(err) }

func errorf(format string, a ...interface{}) xflagsErr {
	return xflagsErr(fmt.Sprintf(format, a...))
}
