// This example demonstrates a simple "Hello, World!" CLI program.
package xflags

import (
	"context"
	"fmt"
	"strings"
)

var (
	flagLanguage   string
	flagNoNewLines bool
	flagMessage    []string
)

var translations = map[string]string{
	"en": "Hello, World!",
	"es": "Hola, Mundo!",
	"it": "Ciao, Mondo!",
	"nl": "Hallo, Wereld!",
}

var App = NewCommand("helloworld", "Print \"Hello, World!\"").
	Description(
		"The helloworld utility writes \"Hello, World!\" to the standard\n"+
			" output multiple languages.",
	).
	Flags(
		// Bool flag to turn off newline printing with -n. A
		// one-character name is a short name, so this declares "-n" and
		// not "--n". The flag value is stored in flagNoNewLines.
		Bool(
			&flagNoNewLines,
			"n",
			false,
			"Do not print the trailing newline character",
		),

		// String flag to select a desired language. Can be specified with
		// -l, --language or the HW_LANG environment variable.
		String(
			&flagLanguage,
			"language",
			"en",
			"Language (en, es, it or nl)",
		).
			Aliases("l").
			Env("HW_LANG"),

		// StringSlice flag to optionally print multiple positional
		// arguments. Positional arguments are not denoted with "-" or "--".
		Strings(
			&flagMessage,
			"MESSAGE",
			nil,
			"Optional message to print",
		).Positional(),
	).
	HandleFunc(helloWorld)

// helloWorld is the HandlerFunc for the main App command.
func helloWorld(ctx context.Context, inv *Invocation) error {
	s, ok := translations[flagLanguage]
	if !ok {
		return fmt.Errorf("unsupported language: %s", flagLanguage)
	}
	if len(flagMessage) > 0 {
		s = strings.Join(flagMessage, " ")
	}
	fmt.Fprint(inv.Stdout, s)
	if !flagNoNewLines {
		fmt.Fprint(inv.Stdout, "\n")
	}
	return nil
}

func Example() {
	ctx := context.Background()

	fmt.Println("+ helloworld --help")
	RunWithArgs(ctx, App, "--help")

	// Most programs will call the following from main:
	//
	//     func main() {
	//         ctx, stop := xflags.NotifyContext(context.Background())
	//         defer stop()
	//         os.Exit(xflags.Run(ctx, App))
	//     }
	//
	fmt.Println()
	fmt.Println("+ helloworld --language=es")
	RunWithArgs(ctx, App, "--language=es")
	// Output:
	// + helloworld --help
	// Usage: helloworld [OPTIONS] [MESSAGE...]
	//
	// Print "Hello, World!"
	//
	// Positional arguments:
	//   MESSAGE  Optional message to print
	//
	// Options:
	//   -n              Do not print the trailing newline character
	//   -l, --language  Language (en, es, it or nl)
	//
	// Environment variables:
	//   HW_LANG  Language (en, es, it or nl)
	//
	// The helloworld utility writes "Hello, World!" to the standard
	//  output multiple languages.
	//
	// + helloworld --language=es
	// Hola, Mundo!
}
