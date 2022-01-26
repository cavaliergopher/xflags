// This example demonstrates a simple "Hello, World!" CLI program.
package xflags

import (
	"fmt"
	"os"
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
	Synopsis(
		"The helloworld utility writes \"Hello, World!\" to the standard\n"+
			" output multiple languages.",
	).
	Flags(
		// Bool flag to turn off newline printing with -n. The flag value is
		// stored in cmd.NoNewLines.
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
			ShortName("l").
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
func helloWorld(args []string) (exitCode int) {
	s, ok := translations[flagLanguage]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unsupported language: %s", flagLanguage)
		return 1
	}
	if len(flagMessage) > 0 {
		s = strings.Join(flagMessage, " ")
	}
	fmt.Print(s)
	if !flagNoNewLines {
		fmt.Print("\n")
	}
	return
}

func Example() {
	fmt.Println("+ helloworld --help")
	RunWithArgs(App, "--help")

	// Most programs will call the following from main:
	//
	//     func main() {
	//         os.Exit(xflags.Run(App))
	//     }
	//
	fmt.Println()
	fmt.Println("+ helloworld --language=es")
	RunWithArgs(App, "--language=es")
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
