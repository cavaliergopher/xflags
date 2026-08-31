/*
Package xflags implements command-line flag parsing and is a compatible alternative to Go's flag
package. This package provides higher-order features such as subcommands, positional arguments,
required arguments, validation, support for environment variables and others.

Package xflags aims to make composing large, full-featured command line tools as simple and clean as
possible. Chained setters are employed to configure commands and flags declaratively.

For compatibility, a flag.FlagSet may be imported with FromFlagSet.

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

You can import all global flags defined using Go's flag library with FromFlagSet.

	var App = xflags.NewCommand(os.Args[0], "").
		FlagGroups(xflags.FromFlagSet("go", "Options", flag.CommandLine))

You can bind a flag to a variable using the Var functions.

	var flagvar int

	var App = xflags.NewCommand(os.Args[0], "").
		Flags(
			xflags.Int(
				&flagvar, "flagname", 1234, "help message for flagname",
			),
		)

Or you can create custom flags that satisfy the ir.Value interface (with pointer receivers) and
couple them to a flag parsing by

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

Option parsing stops at "--". By default every argument after it is an operand, so a command can be
given an operand that looks like an option. A command that sets ForwardArgs takes the other reading:
everything after "--" is handed to the handler unparsed as Invocation.Forwarded.

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

# Help and version

Command.HelpFlag adds the flag that prints a command's help message,
answering to --help and -h. A program that reports a version adds one or
both spellings of it from the one string a build stamps into a constant:

	var App = xflags.NewCommand("orbital", "Operate the fleet").
		HelpFlag().              // --help, -h
		VersionFlag(version).    // --version
		VersionCommand(version)  // orbital version

Add the flags to the root and every command below answers to them too,
each printing its own help. Nothing else on the command line is read or
checked, so they answer a half-typed command line as well.

Declaring them first is a convention rather than a rule: it puts them at
the head of the options a command lists, which is where argparse puts
them and roughly where every other convention does. A program that wants
them elsewhere -- last, hidden, or under a heading of its own -- builds
them with HelpFlag, VersionFlag and VersionCommand and puts them where it
likes.

All three flags are interrupts, which is the whole of what makes --help
special: a flag that ends the parse and runs in place of the command that
was named. Declare one of your own with Interrupt.

# Middleware

Command.Middleware wraps a command's handler, and every handler beneath
it, in one function of your own:

	var App = xflags.NewCommand(os.Args[0], "My application").
		Middleware(Authorize, Trace).
		Subcommands(GetCommand, DeleteCommand)

	func Authorize(next xflags.HandlerFunc) xflags.HandlerFunc {
		return func(ctx context.Context, inv *xflags.Invocation) error {
			if !allowed(inv.Cmd.FullName) {
				return xflags.Exitf(xflags.ExitCodeUsage, "not authorized")
			}
			return next(ctx, inv)
		}
	}

This is for the work every command in a subtree has to do -- an
authorization check, a timing trace, opening a resource and closing it
again -- written once rather than at the top of every handler. Middleware
is inherited down the command path, so one declared on the root wraps
every command in the program, and the outermost wrapper is the one
declared highest in the tree.

A wrapper decides whether to call the handler it wrapped, so returning an
error without calling it refuses the invocation, and Run maps that error
to an exit code as it would the handler's own. It runs only around a
handler, and only after the command line has parsed: neither an
interrupt such as --help, an unparsable command line, nor a command that
exists only to group subcommands reaches one.

Middleware cannot change what a handler is given, since both sides of it
are a HandlerFunc.

A wrapper must be a pure function of the handler it is given, doing its
work in the handler it returns rather than in the wrapper itself. The
wrapping happens when the command tree is compiled, which happens more
than once in a run, so a wrapper that registers a metric or opens a
connection before returning does so more often than its author expects.

# Exit codes

Run returns the exit code the program should terminate with:

	0  the handler returned nil, or an interrupt such as --help ran
	1  the handler returned an error
	2  the command line or the command tree was wrong, or there is no handler

A handler names its own exit code by returning an error that implements
ExitCoder. Exit and Exitf attach a code to an error — a handler reporting a
misuse the parser cannot detect itself, such as two mutually exclusive
flags, returns Exitf(ExitCodeUsage, ...) — and *exec.ExitError already
implements ExitCoder, so the error from a child process can be returned
unchanged to exit with its code.

# Command line flag syntax

In addition to positional arguments, the following forms are permitted:

	-f
	-fx
	-f=x
	-f x // non-boolean flags only
	-abc // equivalent to -a -b -c, for boolean -a and -b
	--flag
	--flag=x
	--flag x   // non-boolean flags only
	--no-flag  // boolean flags only

Short flags group into one argument while each takes no value. The first
that takes one takes the rest of the argument as its value, so -abfx is
-a -b -f x when -a and -b are boolean. An "=" is always a delimiter rather
than a flag name, so a boolean is set false as -f=false or --flag=false.

Every boolean also answers to --no-flag, which sets it false, for each of
its long names. This needs no declaring and cannot be switched off: it is
a second spelling of --flag=false rather than a feature a flag opts into.
The value negates with the flag, so --no-flag=false sets true. Short names
have no negated spelling, since -f=false is already the short way to say
it, and help does not list the negated spellings, since every boolean has
one. An interrupt, such as --help, binds no value and so has none of
these forms: it is given by name and nothing else.

The detached forms are not permitted for boolean flags because the meaning
of the command

	cmd -x *

where * is a Unix shell wildcard, would change if there were a file called
0, false, and so on.

An attached value is taken literally, so it may look like a flag: --flag=-5
is negative five, where --flag -5 is a missing value. See
docs/adr/posix-argument-conventions.md for the dialect in full, and for the
two places it departs from getopt.

# Shell completion

Command.EnableCompletion opts a command into shell completion:

	var App = xflags.NewCommand(os.Args[0], "My application").
		EnableCompletion()

Once enabled, Run and RunWithArgs check one environment variable before
doing anything else -- the command's name, uppercased, with every
non-alphanumeric rune mapped to "_", and "_COMPLETE" appended, so "myapp"
answers to MYAPP_COMPLETE. A recognized value there makes Run print a
completion script or a completion reply and return, without invoking any
handler; any other value, including the variable being unset, leaves Run's
behavior exactly as if EnableCompletion had not been called.

A user enables completion in their shell with a one-liner naming that
variable:

	source <(MYAPP_COMPLETE=bash_source myapp 2>/dev/null)
	source <(MYAPP_COMPLETE=zsh_source myapp 2>/dev/null)

That prints a small script, generated for the shell asked for, which
re-invokes the binary as the user types to ask what completes the word
under the cursor. Flags declare what completes their value with Choices,
for a fixed list, or Flag.Complete, for a callback computing candidates
from the Invocation parsed so far -- what completes one flag's value often
depends on another already given, as the ref argument to `git checkout`
depends on which repository is checked out.

Complete is the engine behind the reply, and answers the same
question programmatically: given the command line so far and the word
being completed, which candidates apply. It is exported so it can be
tested and driven directly, without a shell in the loop.
*/
package xflags
