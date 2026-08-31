// This example demonstrates the packaging convention for a library that
// contributes command line flags to whichever program imports it: a
// settings struct whose FlagGroup method binds flags to the receiver, and
// one package-level instance registered with Register.
package xflags

import (
	"context"
	"fmt"
)

// logSettings is the settings struct a library such as a logging package
// would own. Its FlagGroup method binds flags to the receiver rather than
// to package variables, which is the point of the convention: a test binds
// a fresh, isolated instance, while the package-level instance below is
// merely the one the program mounts.
type logSettings struct {
	Level  string
	Format string
}

// FlagGroup returns a new group of flags bound to s.
func (s *logSettings) FlagGroup() *FlagGroup {
	return NewFlagGroup(
		"logging", "Logging options",
		String(&s.Level, "log-level", "info", "Set log verbosity"),
		String(&s.Format, "log-format", "text", "Log output format"),
	)
}

// logFlags is the instance the program mounts, registered in the var
// declaration that builds it -- Register returns its argument, so no init
// function is needed. A library wanting no handle registers into the blank
// identifier: var _ = Register(...). Either way, a blank import of the
// library is enough to register its flags.
var logFlags = &logSettings{}

var _ = Register(logFlags.FlagGroup())

func Example_sharedFlagGroups() {
	ctx := context.Background()

	// The program mounts every registered group with one line. From
	// outside this package that line reads GroupSets(xflags.CommandLine).
	app := NewCommand("myapp", "Do things, with logging").
		GroupSets(CommandLine).
		HandleFunc(func(ctx context.Context, inv *Invocation) error {
			fmt.Fprintf(inv.Stdout, "level: %s\n", logFlags.Level)
			return nil
		})

	fmt.Println("+ myapp --help")
	RunWithArgs(ctx, app, "--help")

	fmt.Println()
	fmt.Println("+ myapp --log-level=debug")
	RunWithArgs(ctx, app, "--log-level=debug")

	// A test mounts a fresh instance instead, fully isolated from the
	// registered one.
	var isolated logSettings
	cmd := NewCommand("test", "").FlagGroups(isolated.FlagGroup())
	if _, err := Parse(cmd, "--log-level=warn"); err != nil {
		panic(err)
	}
	fmt.Println()
	fmt.Printf("isolated: %s, registered: %s\n", isolated.Level, logFlags.Level)
	// Output:
	// + myapp --help
	// Usage: myapp [OPTIONS]
	//
	// Do things, with logging
	//
	// Logging options:
	//    --log-level   Set log verbosity
	//    --log-format  Log output format
	//
	// + myapp --log-level=debug
	// level: debug
	//
	// isolated: warn, registered: debug
}
