package xflags

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
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
	return cmd.Run(ctx, args)
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

// stringifyDefault returns the string form of a flag's default value, using
// its Value's String method if it implements fmt.Stringer.
func stringifyDefault(v Value) string {
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	return ""
}

// Var returns a Flag that can be used to define a command line flag with
// custom value parsing.
func Var(value Value, name, usage string) *Flag {
	c := &Flag{
		name:     name,
		usage:    usage,
		minCount: defaultMinNArgs,
		maxCount: defaultMaxNArgs,
		value:    value,
	}
	if len(name) == 1 {
		c.shortName = c.name
		c.name = ""
	}
	return c
}

// BitField returns a Flag that can be used to define a uint64 flag
// with specified name, default value, and usage string. The argument p points
// to a uint64 variable in which to toggle each of the bits in the mask
// argument. You can specify multiple BitFieldVars to toggle bits in the same
// underlying uint64.
func BitField(p *uint64, mask uint64, name string, value bool, usage string) *Flag {
	v := newBitFieldValue(value, p, mask)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	return c
}

// Bool returns a Flag that can be used to define a bool flag with
// specified name, default value, and usage string. The argument p points to a
// bool variable in which to store the value of the flag.
func Bool(p *bool, name string, value bool, usage string) *Flag {
	v := newBoolValue(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	return c
}

// Duration returns a Flag that can be used to define a time.Duration
// flag with specified name, default value, and usage string. The argument p
// points to a time.Duration variable in which to store the value of the flag.
// The flag accepts a value acceptable to time.ParseDuration.
func Duration(p *time.Duration, name string, value time.Duration, usage string) *Flag {
	v := newDurationValue(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	return c
}

// Float64 returns a Flag that can be used to define a float64 flag
// with specified name, default value, and usage string. The argument p points
// to a float64 variable in which to store the value of the flag.
func Float64(p *float64, name string, value float64, usage string) *Flag {
	v := newFloat64Value(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	return c
}

// Func returns a Flag that can used to define a flag with the specified name and usage
// string.
// Each time the flag is seen, fn is called with the value of the flag.
// If fn returns a non-nil error, it will be treated as a flag value parsing error.
//
// The flag may be given any number of times, since fn accumulates whatever it
// likes; constrain it with NArgs.
func Func(name, usage string, fn func(s string) error) *Flag {
	return Var(funcValue(fn), name, usage).NArgs(0, 0)
}

// Int returns a Flag that can be used to define an int flag with
// specified name, default value, and usage string. The argument p points to an
// int variable in which to store the value of the flag.
func Int(p *int, name string, value int, usage string) *Flag {
	v := newIntValue(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	return c
}

// Int64 returns a Flag that can be used to define an int64 flag with
// specified name, default value, and usage string. The argument p points to an
// int64 variable in which to store the value of the flag.
func Int64(p *int64, name string, value int64, usage string) *Flag {
	v := newInt64Value(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	return c
}

// String returns a Flag that can be used to define a string flag with
// specified name, default value, and usage string. The argument p points to a
// string variable in which to store the value of the flag.
func String(p *string, name, value, usage string) *Flag {
	v := newStringValue(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	return c
}

// Strings returns a Flag that can be used to define a string slice flag with specified name,
// default value, and usage string. The argument p points to a string slice variable in which each
// flag value will be stored in command line order.
func Strings(p *[]string, name string, value []string, usage string) *Flag {
	v := newStringSliceValue(value, p)
	c := Var(v, name, usage).NArgs(0, 0)
	c.defValue = stringifyDefault(v)
	return c
}

// Uint returns a Flag that can be used to define an uint flag with
// specified name, default value, and usage string. The argument p points to an
// uint variable in which to store the value of the flag.
func Uint(p *uint, name string, value uint, usage string) *Flag {
	v := newUintValue(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	return c
}

// Uint64 returns a Flag that can be used to define an uint64 flag
// with specified name, default value, and usage string. The argument p points
// to an uint64 variable in which to store the value of the flag.
func Uint64(p *uint64, name string, value uint64, usage string) *Flag {
	v := newUint64Value(value, p)
	c := Var(v, name, usage)
	c.defValue = stringifyDefault(v)
	return c
}
