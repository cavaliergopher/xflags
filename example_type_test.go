// This example demonstrates how a custom struct type may be used to encapsulate
// the behavior of a single command.
package xflags

import "fmt"

// exampleCommand implements the Commander interface to define a CLI command
// and its handler using a custom type. It collects the value of each flag as
// a struct field.
type exampleCommand struct {
	Species    string
	GopherType string
}

// Command implements Commander and returns the CLI configuration of the example
// command.
func (c *exampleCommand) Command() (*Command, error) {
	return NewCommand("example", "An example CLI program").
		Flags(
			String(&c.Species, "species", "Gopher", "the species we are studying"),
			String(&c.GopherType, "gopher_type", "Pocket", "the variety of gopher"),
		).
		HandleFunc(c.Run).
		Command()
}

// Run handles calls to this command from the command line.
//
// If WithTerminator is specified for the App command, any arguments given after
// the "--" terminator will be passed in as the args parameter without any
// further parsing.
func (c *exampleCommand) Run(args []string) int {
	fmt.Printf("%s is a variety of species %s\n", c.GopherType, c.Species)
	return 0
}

// ExampleCommand is a global instance of the exampleCommand type so that its
// parsed flag values can be accessed from other commands. This is an optional
// alternative to defining flag variables individually in the global scope.
var ExampleCommand = &exampleCommand{}

func Example_customTypes() {
	fmt.Println("+ example --help")
	RunWithArgs(ExampleCommand, "--help")

	// Most programs will call the following from main:
	//
	//     func main() {
	//         os.Exit(xflags.Run(ExampleCommand))
	//     }
	//
	fmt.Println()
	fmt.Println("+ example --gopher_type 'Goldman's pocket gopher'")
	RunWithArgs(ExampleCommand, "--gopher_type", "Goldman's pocket gopher")
	// Output:
	// + example --help
	// Usage: example [OPTIONS]
	//
	// An example CLI program
	//
	// Options:
	//    --species      the species we are studying
	//    --gopher_type  the variety of gopher
	//
	// + example --gopher_type 'Goldman's pocket gopher'
	// Goldman's pocket gopher is a variety of species Gopher
}
