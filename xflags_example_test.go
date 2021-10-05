// These examples demonstrate more intricate uses of the xflags package.
package xflags

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	flagSpecies    string = "gopher"
	flagGopherType string = "pocket"
	flagInterval   interval
)

var App = Command("example").
	Usage("An example CLI program").
	Flags(
		// Example 1: A single string flag called "species" with default value
		// "gopher".
		String(&flagSpecies, "species").
			Usage("the species we are studying").
			MustBuild(),

		// Example 2: An alternative short name, so we can have a shorthand.
		String(&flagGopherType, "gopher_type").
			ShortName("g").
			Usage("the variety of gopher").
			MustBuild(),

		// Example 3: A user-defined flag type, a slice of durations.
		// Define a flag to accumulate durations.
		Var(&flagInterval, "deltaT").
			Usage("comma-separated list of intervals to use between events").
			MustBuild(),
	).
	MustBuild()

// interval is a custom flag type that implements the xflags.Value interface.
type interval []time.Duration

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

// Reset is the method to reset the flag value from its default value, part of
// the xflags.Value interface.
func (i *interval) Reset() {
	*i = make([]time.Duration, 0)
}

// String is the method to format the flag's value, part of the xflags.Value interface.
// The String method's output will be used in diagnostics.
func (i *interval) String() string {
	return fmt.Sprint(*i)
}

func Example() {
	// All the interesting pieces are with the variables declared above, but
	// to enable the xflags package to see the cli defined there, one must
	// execute, typically at the start of main (not init!):
	//	xflags.Run(App)
	// We don't run it here because this is not a main function and
	// the testing suite has already parsed the flags.
}