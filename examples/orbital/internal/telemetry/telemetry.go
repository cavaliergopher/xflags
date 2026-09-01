// Package telemetry stands in for a platform library that every team's
// binary links in for logging and tracing. It contributes its flags the
// way a shared library should: a settings struct with a FlagGroup method,
// registered once into climux.CommandLine so any command that mounts the
// set with GroupSets(climux.CommandLine) picks it up automatically.
package telemetry

import "go.hotsrc.dev/climux"

// Settings holds telemetry configuration. A package that wants an
// isolated instance -- a test, say -- constructs its own Settings and
// calls FlagGroup on it, rather than reaching for the package-level Flags
// below; see example_group_test.go in the climux module root.
type Settings struct {
	LogLevel string
	Trace    bool
}

// FlagGroup returns a new group of flags bound to s.
func (s *Settings) FlagGroup() *climux.FlagGroup {
	return climux.NewFlagGroup(
		"telemetry", "Telemetry options",
		climux.String(&s.LogLevel, "log-level", "info", "Set the log verbosity").
			Choices("debug", "info", "warn", "error").
			ShowDefault(),
		climux.Bool(&s.Trace, "trace", false, "Emit a timing trace for every command").
			Env("ORBITAL_TRACE"),
	)
}

// Flags is the instance orbital mounts. Register appends its group to
// climux.CommandLine in this var declaration, so any command that mounts
// the set -- see main.go's GroupSets(climux.CommandLine) -- picks it up
// with no further wiring.
var Flags = &Settings{}

var _ = climux.Register(Flags.FlagGroup())
