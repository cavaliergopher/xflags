// These examples demonstrate more intricate uses of the xflags package.
package xflags

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	flagSpecies    string
	flagGopherType string
	flagInterval   interval
)

var App = Command("example").
	Usage("An example CLI program").
	Flags(
		// Example 1: A single string flag called "species" with default value
		// "gopher".
		StringVar(&flagSpecies, "species", "Gopher", "the species we are studying").
			Must(),

		// Example 2: An alternative short name, so we can have a shorthand.
		StringVar(&flagGopherType, "gopher_type", "Pocket", "the variety of gopher").
			ShortName("g").
			Must(),

		// Example 3: A user-defined flag type, a slice of durations.
		// Define a flag to accumulate durations.
		Var(&flagInterval, "deltaT", "comma-separated list of intervals to use between events").
			Must(),
	).
	Handler(HandleExample).
	Must()

// interval is a custom flag type that implements the xflags.Value interface.
type interval []time.Duration

// G is the method to get the flag value, part of the xflags.Value interface.
func (i *interval) Get() interface{} { return []time.Duration(*i) }

// Set is the method to set the flag value, part of the xflags.Value interface.
// Set's argument is a string to be parsed to set the flag.
// It's a comma-separated list, so we split it.
func (i *interval) Set(value string) error {
	// If we wanted to allow the flag to be set multiple times,
	// accumulating values, we would delete this if statement.
	// That would permit usages such as
	//	-deltaT 10s -deltaT 15s
	// and other combinations.
	if len(*i) > 0 {
		return errors.New("interval flag already set")
	}
	for _, dt := range strings.Split(value, ",") {
		duration, err := time.ParseDuration(dt)
		if err != nil {
			return err
		}
		*i = append(*i, duration)
	}
	return nil
}

// String is the method to format the flag's value, part of the xflags.Value interface.
// The String method's output will be used in diagnostics.
func (i *interval) String() string {
	return fmt.Sprint(*i)
}

// HandleExample is the CommandFunc for the main App command.
func HandleExample(args []string) int {
	fmt.Printf("%s is a variety of species %s\n", flagGopherType, flagSpecies)
	return 0
}

func Example() {
	// Most programs will call the following from main:
	//
	//     func main() {
	//         os.Exit(xflags.Run(App))
	//     }
	//
	args := []string{"--gopher_type", "Goldman's pocket gopher"}
	App.Run(args)
	// output: Goldman's pocket gopher is a variety of species Gopher
}
