/*
Package xflags implements command-line flag parsing and is a compatible alternative to Go's flag
package. This package provides higher-order features such as subcommands, positional arguments,
required arguments, validation, support for environment variables and others.

Package xflags aims to make composing large, full-featured command line tools as simple and clean as
possible. The Builder pattern is employed with method chaining to configure commands and flags
declaratively with error checking.

For compatibility, flag.FlagSets may be imported with CommandBuilder.FlagSet.

Usage

Every xflags program must define a top-level command using xflags.NewCommand:

	import (
		"os"

		"github.com/cavaliergopher/xflags"
	)

	var App = xflags.NewCommand(os.Args[0], "My application")

	func main() {
		os.Exit(xflags.Run(App))
	}

You can import all global flags defined using Go's flag library with CommandBuilder.FlagSet.

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

	func MyAppHandler(args []string) int {
		return 0
	}

Flag parsing will stop after "--" only if a command sets WithTerminator. All arguments following the
terminator will be passed to the command handler.

You can define subcommands by

	var (
		FooCommand = xflags.NewCommand("foo", "Foo command")
		BarCommand = xflags.NewCommand("bar", "Bar command")

		App = xflags.NewCommand(os.Args[0], "Foo bar program").
			Subcommands(FooCommand, BarCommand)
	)

After all flags are defined, call

	xflags.Run(App)

to parse the command line into the defined flags and call the handler associated with the command or
any if its subcommands if specified in os.Args.

Flags may then be used directly.

	fmt.Println("ip has value ", ip)
	fmt.Println("flagvar has value ", flagvar)

Command line flag syntax

In addition to positional arguments, the following forms are permitted:

	-f
	-f=x
	-f x // non-boolean flags only
	--flag
	--flag=x
	--flag x // non-boolean flags only

The noted forms are not permitted for boolean flags because of the meaning of the command

	cmd -x *

where * is a Unix shell wildcard, will change if there is a file called 0, false, etc.
*/
package xflags
