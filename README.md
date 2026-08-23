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
	"fmt"
	"os"

	"github.com/cavaliergopher/xflags"
)

var flagName string

var App = xflags.NewCommand("greet", "Print a greeting").
	Flags(
		xflags.String(&flagName, "name", "World", "Who to greet"),
	).
	HandleFunc(func(args []string) int {
		fmt.Printf("Hello, %s!\n", flagName)
		return 0
	})

func main() {
	os.Exit(xflags.Run(App))
}
```

Flag values are stored in variables you own, so they are read directly with no
lookup by name. Configuration errors — a duplicate flag, a positional argument
declared alongside subcommands — are reported when the command line is parsed.

## Describing a command

`Command.Describe` compiles a command tree into the plain data types in the
[desc](https://pkg.go.dev/github.com/cavaliergopher/xflags/desc) package: every
command, flag group and flag, with behavior dropped and ancestry resolved. The
default help formatter walks it, and so can your own tooling.

See [the docs](https://pkg.go.dev/github.com/cavaliergopher/xflags) for
comprehensive examples.
