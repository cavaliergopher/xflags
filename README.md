# Expressive flags for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/cavaliergopher/xflags.svg)](https://pkg.go.dev/github.com/cavaliergopher/xflags)
[![CI](https://github.com/cavaliergopher/xflags/actions/workflows/ci.yml/badge.svg)](https://github.com/cavaliergopher/xflags/actions/workflows/ci.yml)

Package xflags implements command-line flag parsing and is a compatible
alternative to Go's flag package. This package provides higher-order features
such as subcommands, positional arguments, required arguments, validation,
support for environment variables and others.

Package xflags aims to make composing large, full-featured command line tools as
simple and clean as possible. Chained setters are employed to configure
commands and flags declaratively. There are no dependencies beyond the standard
library, and no code generation or struct tags.

## Install

```
go get github.com/cavaliergopher/xflags
```

## Usage

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cavaliergopher/xflags"
)

var flagName string

var App = xflags.NewCommand("greet", "Print a greeting").
	Flags(
		xflags.String(&flagName, "name", "World", "Who to greet"),
	).
	HandleFunc(func(ctx context.Context, inv *xflags.Invocation) error {
		fmt.Fprintf(inv.Stdout, "Hello, %s!\n", flagName)
		return nil
	})

func main() {
	ctx, stop := xflags.NotifyContext(context.Background())
	defer stop()
	os.Exit(xflags.Run(ctx, App))
}
```

Flag values are stored in variables you own, so they are read directly with no
lookup by name. Configuration errors — a duplicate flag, a positional argument
declared alongside subcommands — are reported when the command line is parsed.

A handler returns an error and `Run` turns it into an exit code: 0 for success
or `--help`, 1 for a handler that failed, 2 for a command line that was wrong.
An error may name its own code by implementing `ExitCoder`. The context comes
from `NotifyContext`, which cancels it on the first interrupt and restores
default signal handling so a second one kills a wedged process.

The `Invocation` tells the handler how it was called — which command ran, the
path it was reached by, and anything after a `--` terminator. A command is
usually mounted by whoever composes the binary rather than by the team that
wrote it, so its own path is not something it can know until it runs.

## Command line syntax

The dialect is the POSIX Utility Syntax Guidelines plus GNU long options —
what `getopt_long` accepts — rather than Go's `flag` package, where one dash
and two mean the same thing.

```
-f            --flag           a flag taking no value
-f=false      --flag=false     a boolean set false
              --no-flag        a boolean set false, spelled the other way
-fx  -f=x     --flag=x         a value attached to its flag
-f x          --flag x         a value in the next argument
-abc                           -a -b -c, while each takes no value
-abfx                          -a -b -f x, where -f takes one
```

Two arguments are not flags at all. A bare `-` is an ordinary operand, left
to the handler to interpret, and `--` ends option processing by default:
every argument after it is an operand, however many dashes it starts with.
A command that sets `ForwardArgs` hands everything after `--` to the
handler unparsed instead, as `Invocation.Forwarded`.

An argument beginning with `-` is never taken as a detached value, so
`--count -5` is a missing value rather than negative five; write
`--count=-5`. Flags may appear among the operands in any order, and a flag
is legal from the point its own command is named onward.

Every boolean also answers to `--no-flag`, for each of its long names,
which sets it false. Nothing declares it and nothing can switch it off: it
is a second spelling of `--flag=false`, not a feature a flag opts into.
The value negates with the flag, so `--no-flag=false` sets true. Short
names get no negated spelling, and help does not list the negated ones,
since every boolean has one.

Five departures from `getopt_long` are deliberate, and
[the ADR](docs/adr/posix-argument-conventions.md) argues each one:

- **Attached values follow Go, not getopt.** `-n=value` sets `value` rather
  than `=value`, and a boolean accepts an attached value, so `--flag=false`
  and `-f=false` both set false. Without it a boolean could not be turned
  off at all.
- **Negated booleans are generated, not declared.** `getopt_long` leaves
  `--no-flag` to each program to declare; here every boolean has one, so
  a user never has to find out which of a program's booleans got one.
- **Long options may not be abbreviated.** `getopt_long` accepts any unique
  prefix, but a command tree makes "unique" a moving target: adding a flag
  to a subcommand can break a script that never changed.
- **`ForwardArgs` is opt-in.** By default, everything after `--` binds to
  positional flags like any other operand. A command that sets
  `ForwardArgs` instead hands it to the handler unparsed, as
  `Invocation.Forwarded`. POSIX has no subcommands, so it has no case to
  forward arguments to.
- **`-h` and `--help` are reserved** by the parser, which is GNU practice
  rather than POSIX.

## Compiling a command

`Command.Compile` lowers a command tree into the implementation types in the
[ir](https://pkg.go.dev/github.com/cavaliergopher/xflags/ir) package: every
command, flag group and flag, with ancestry resolved and exported fields
holding everything that marshals. The default help formatter walks it, and
so can your own tooling. Most programs never need to call Compile
themselves, or import ir at all -- it is what Parse, Run and the help
output use internally.

See [the docs](https://pkg.go.dev/github.com/cavaliergopher/xflags) for
comprehensive examples.
