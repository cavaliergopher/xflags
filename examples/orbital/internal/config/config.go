// Package config implements "orbital config get|set", a small in-memory
// key/value store standing in for a real configuration service. get and
// set each take positional arguments, and set validates its key against
// the ones the store already knows about.
package config

import "go.hotsrc.dev/climux"

// store is orbital's local configuration, seeded with the values a fresh
// checkout would ship with.
var store = map[string]string{
	"region":      "us-west-2",
	"environment": "staging",
}

// Command returns the "config" subcommand, which only groups "get" and
// "set" -- it has no handler of its own, so "orbital config" alone is a
// usage error naming its subcommands.
func Command() *climux.Command {
	return climux.NewCommand("config", "Read and write orbital's local configuration").
		Subcommands(getCommand(), setCommand())
}
