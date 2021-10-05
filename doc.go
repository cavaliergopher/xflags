/*
Package xflags provides an alternative to Go's flag package for defining and
parsing command line arguments where subcommands are a first class citizen and
common CLI features such as required arguments, position arguments or shorthand
flag names are simple and clear to express.

	// A simple "Hello World" program
	package main

	import (
		"fmt"
		"os"
		"strings"

		"github.com/cavaliergopher/xflags"
	)

	var flagMessages = []string{"Hello,", "World!"}

	var App = xflags.Command("helloworld").
		Usage("An example CLI program").
		Flags(
			xflags.StringSlice(&flagMessages, "messages").
				Positional().
				MustBuild(),
		).
		Handler(helloWorld).
		MustBuild()

	func helloWorld() int {
		fmt.Println(strings.Join(flagMessages, " "))
		return 0
	}

	func main() {
		os.Exit(xflags.Run(App))
	}
*/
package xflags
