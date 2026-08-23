// Package desc is the compiled, behavior-free description of a command
// tree, produced by (*xflags.Command).Describe.
//
// It is a projection of the source tree, not a mirror of it: it drops
// behavior — no Value, no ValidateFunc, no HandlerFunc, no io.Writers, no
// FormatFunc — and adds resolution — ancestry via Parent, and each flag's
// default rendered as a string.
//
// The types here are plain structs with no methods and no behavior of their
// own. They are meant to be walked, never authored: help formatters are the
// first consumer, with completion, doc generation and machine-readable
// output expected to follow.
package desc

// Command is the compiled description of an xflags.Command.
type Command struct {
	Parent         *Command `json:"-"`
	Name           string
	Usage          string
	Synopsis       string
	Hidden         bool
	WithTerminator bool
	FlagGroups     []*FlagGroup
	Subcommands    []*Command
}

// FlagGroup is the compiled description of an xflags.FlagGroup.
type FlagGroup struct {
	Name  string
	Usage string
	Flags []*Flag
}

// Flag is the compiled description of an xflags.Flag.
type Flag struct {
	Name        string
	ShortName   string
	Usage       string
	Default     string
	ShowDefault bool
	Positional  bool
	Hidden      bool
	MinCount    int
	MaxCount    int
	EnvVar      string
}
