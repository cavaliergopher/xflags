/*
Package xflags implements command-line flag parsing and is a compatible alternative to Go's flag
package. This package provides higher-order features such as subcommands, positional arguments,
required arguments, validation, support for environment variables and others.

Package xflags aims to make composing large, full-featured command line tools as simple and clean as
possible. Chained setters are employed to configure commands and flags declaratively.

For compatibility, flag.FlagSets may be imported with Command.FlagSet.

# Usage

Every xflags program must define a top-level command using xflags.NewCommand:

	import (
		"context"
		"os"

		"github.com/cavaliergopher/xflags"
	)

	var App = xflags.NewCommand(os.Args[0], "My application")

	func main() {
		ctx, stop := xflags.NotifyContext(context.Background())
		defer stop()
		os.Exit(xflags.Run(ctx, App))
	}

You can import all global flags defined using Go's flag library with Command.FlagSet.

	var App = xflags.NewCommand(os.Args[0], "").FlagSet(flag.CommandLine)

You can bind a flag to a variable using the Var functions.

	var flagvar int

	var App = xflags.NewCommand(os.Args[0], "").
		Flags(
			xflags.Int(
				&flagvar, "flagname", 1234, "help message for flagname",
			),
		)

Or you can create custom flags that satisfy the Value interface (with pointer receivers) and couple
them to a flag parsing by

	xflags.Var(&flagVal, "name", "help message for flagname")

For such flags, the default value is just the initial value of the variable.

A handler may be defined for your command by

	var App = xflags.NewCommand(os.Args[0], "").HandleFunc(MyAppHandler)

	func MyAppHandler(ctx context.Context, inv *xflags.Invocation) error {
		return nil
	}

The handler is given the context passed to Run, which NotifyContext cancels on
SIGINT or SIGTERM, and an Invocation describing how it was called: the command
that was named, the path it was reached by, any arguments after the "--"
terminator, and the streams to work with. A command is often mounted in a tree
its author does not own, so the path is something only the invocation can tell
it.

A handler should read and write the streams on its invocation rather than the
process streams, for the same reason: whoever composes the binary decides where
the input and output of any command in the tree go, with Command.Stdin,
Command.Stdout and Command.Stderr.

	func MyAppHandler(ctx context.Context, inv *xflags.Invocation) error {
		fmt.Fprintln(inv.Stdout, "Hello, World!")
		return nil
	}

Flag parsing will stop after "--" only if a command sets WithTerminator. All arguments following the
terminator are passed to the command handler as Invocation.Args.

You can define subcommands by

	var (
		FooCommand = xflags.NewCommand("foo", "Foo command")
		BarCommand = xflags.NewCommand("bar", "Bar command")

		App = xflags.NewCommand(os.Args[0], "Foo bar program").
			Subcommands(FooCommand, BarCommand)
	)

After all flags are defined, call

	xflags.Run(ctx, App)

to parse the command line into the defined flags and call the handler associated with the command or
any if its subcommands if specified in os.Args.

Flags may then be used directly.

	fmt.Println("ip has value ", ip)
	fmt.Println("flagvar has value ", flagvar)

# Exit codes

Run returns the exit code the program should terminate with:

	0  the handler returned nil, or -h or --help was given
	1  the handler returned an error
	2  the command line or the command tree was wrong, or there is no handler

A handler names its own exit code by returning an error that implements
ExitCoder. Exit and Exitf attach a code to an error — a handler reporting a
misuse the parser cannot detect itself, such as two mutually exclusive
flags, returns Exitf(ExitCodeBadArgument, ...) — and *exec.ExitError already
implements ExitCoder, so the error from a child process can be returned
unchanged to exit with its code.

# Command line flag syntax

In addition to positional arguments, the following forms are permitted:

	-f
	-fx
	-f=x
	-f x // non-boolean flags only
	--flag
	--flag=x
	--flag x // non-boolean flags only

The detached forms are not permitted for boolean flags because the meaning
of the command

	cmd -x *

where * is a Unix shell wildcard, would change if there were a file called
0, false, and so on. A boolean is set false with an attached value instead,
as in --flag=false.

An attached value is taken literally, so it may look like a flag: --flag=-5
is negative five, where --flag -5 is a missing value. See
docs/adr/posix-argument-conventions.md for the dialect in full, and for the
two places it departs from getopt.
*/
package xflags
