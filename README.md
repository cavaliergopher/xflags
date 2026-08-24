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

## Describing a command

`Command.Describe` compiles a command tree into the plain data types in the
[desc](https://pkg.go.dev/github.com/cavaliergopher/xflags/desc) package: every
command, flag group and flag, with behavior dropped and ancestry resolved. The
default help formatter walks it, and so can your own tooling.

See [the docs](https://pkg.go.dev/github.com/cavaliergopher/xflags) for
comprehensive examples.
