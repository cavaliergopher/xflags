// Package telemetry stands in for a platform library that every team's
// binary links in for logging and tracing. It contributes its flags the
// way a shared library should: a settings struct with a FlagGroup method,
// registered once into xflags.CommandLine so any command that mounts the
// set with GroupSets(xflags.CommandLine) picks it up automatically.
package telemetry

import "github.com/cavaliergopher/xflags"

// Settings holds telemetry configuration. A package that wants an
// isolated instance -- a test, say -- constructs its own Settings and
// calls FlagGroup on it, rather than reaching for the package-level Flags
// below; see example_group_test.go in the xflags module root.
type Settings struct {
	LogLevel string
	Trace    bool
}

// FlagGroup returns a new group of flags bound to s.
func (s *Settings) FlagGroup() *xflags.FlagGroup {
	return xflags.NewFlagGroup(
		"telemetry", "Telemetry options",
		xflags.String(&s.LogLevel, "log-level", "info", "Set the log verbosity").
			Choices("debug", "info", "warn", "error").
			ShowDefault(),
		xflags.Bool(&s.Trace, "trace", false, "Emit a timing trace for every command").
			Env("ORBITAL_TRACE"),
	)
}

// Flags is the instance orbital mounts. Register appends its group to
// xflags.CommandLine in this var declaration, so any command that mounts
// the set -- see main.go's GroupSets(xflags.CommandLine) -- picks it up
// with no further wiring.
var Flags = &Settings{}

var _ = xflags.Register(Flags.FlagGroup())
