# Expressive flags for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/cavaliergopher/xflags.svg)](https://pkg.go.dev/github.com/cavaliergopher/xflags) [![Build Status](https://app.travis-ci.com/cavaliergopher/xflags.svg?branch=main)](https://app.travis-ci.com/cavaliergopher/xflags)

Package xflags implements command-line flag parsing and is a compatible
alternative to Go's flag package. This package provides higher-order features
such as subcommands, positional arguments, required arguments, validation,
support for environment variables and others.

Package xflags aims to make composing large, full-featured command line tools as
simple and clean as possible. The Builder pattern is employed with method
chaining to configure commands and flags declaratively with error checking.

## Example

The following is a simple [Hello, World!](https://en.wikipedia.org/wiki/%22Hello,_World!%22_program)
program that demonstrates how to use xflags to build a command line interface.

```plaintext
$ helloworld --help
Usage: helloworld [OPTIONS] [MESSAGE...]

Print "Hello, World!"

Positional arguments:
  MESSAGE  Optional message to print

Options:
  -n              Do not print the trailing newline character
  -l, --language  Language (en, es, it or nl)

The helloworld utility writes "Hello, World!" the standard output in multiple
languages.

$ helloworld --language es
Hola, Mundo!
```

```go
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cavaliergopher/xflags"
)

var translations = map[string]string{
	"en": "Hello, World!",
	"es": "Hola, Mundo!",
	"it": "Ciao, Mondo!",
	"nl": "Hallo, Wereld!",
}

var (
	flagLanguage   string
	flagNoNewlines bool
	flagMessage    []string
)

// App is the entry-point command for our program and is invoked in main.
var App = xflags.Command("helloworld", "Print \"Hello, World!\"").
	Synopsis(
		"The helloworld utility writes \"Hello, World!\" the standard output in multiple\n"+
			"languages.",
	).
	Flags(
		// Bool flag to turn off newline printing with -n.
		xflags.BoolVar(
			&flagNoNewlines,
			"n",
			false,
			"Do not print the trailing newline character",
		).Must(),

		// String flag to select a desired language. Can be specified with both -l and --language.
		xflags.StringVar(&flagLanguage, "language", "en", "Language (en, es, it or nl)").
			ShortName("l").
			Must(),

		// StringSlice flag to optionally print multiple positional arguments.
		// Positional arguments are not denoted with "-" or "--".
		xflags.StringSliceVar(&flagMessage, "MESSAGE", nil, "Optional message to print").
			Positional().
			Must(),
	).
	Subcommands(
	// Subcommands can be defined in other Go files and included here.
	).
	Handler(HelloWorld).
	Must()

// HelloWorld is a CommandFunc that handles the App command.
func HelloWorld(args []string) (exitCode int) {
	s, ok := translations[flagLanguage]
	if !ok {
		log.Fatalf("Unsupported language: %s", flagLanguage)
	}
	if len(flagMessage) > 0 {
		s = strings.Join(flagMessage, " ")
	}
	fmt.Print(s)
	if !flagNoNewlines {
		fmt.Print("\n")
	}
	return
}

func main() {
	os.Exit(xflags.Run(App))
}
```
