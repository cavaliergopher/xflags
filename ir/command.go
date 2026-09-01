package ir

import (
	"context"
	"io"

	"github.com/cavaliergopher/xflags/desc"
)

// An Invocation is the result of parsing a command line. It records which
// command the arguments named, what was left for its handler, and the
// streams the handler should read and write.
//
// Where the invoked command sits in the program is Cmd's to answer:
// Cmd.FullName names it from the root down, and Cmd.Ancestry is the
// commands themselves.
type Invocation struct {
	// Cmd is the command the arguments named.
	Cmd *Command

	// Forwarded holds the arguments the parser deliberately left
	// unparsed: everything after a "--" terminator, for a command that
	// opted in with Command.ForwardArgs, or everything after the token
	// that ended the parse, when an interrupt flag or an interrupt
	// command did. It is empty otherwise.
	//
	// This is not the command's operands, which bind to positional flags
	// as usual. These are the arguments left for the handler to
	// interpret or ignore: what a forwarding command hands on to
	// something else, or what follows an interrupt, verbatim.
	Forwarded []string

	// Interrupt is the flag that ended the parse, and is nil both when
	// the whole command line was read and when Cmd is itself an
	// interrupt command; see Command.Interrupt. Its Handler runs in
	// place of Cmd's, which is the command that was active when the flag
	// was given -- the one whose help is printed when the flag is the
	// one asking for it.
	//
	// The rest of the command line is not parsed and the flag rules are
	// not checked, so an interrupt answers even on an otherwise incomplete
	// command line; what followed the flag arrives in Forwarded. See
	// Flag.Handler.
	Interrupt *Flag

	// Stdin, Stdout and Stderr are the streams the handler should use in
	// place of the process streams, so that whoever composes the binary
	// decides where its input and output go. Each is resolved independently
	// from the invoked command and its ancestors, defaulting to the
	// matching process stream; see Command.Stdin, Command.Stdout and
	// Command.Stderr.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// A HandlerFunc handles the invocation of a command specified by command
// line arguments.
//
// ctx is the context given to xflags.Dispatch, so a handler
// that does anything cancelable should honor it.
//
// inv describes the invocation: the command that was named, the path it was
// reached by, any arguments forwarded past a "--" terminator, and the
// streams to work with. A handler should read inv.Stdin and write
// inv.Stdout and inv.Stderr rather than the process streams, so that a
// caller that redirects the command captures its output too. Nothing
// enforces it; a handler that reaches for os.Stdout simply escapes the
// redirection.
//
// Returning nil exits with code 0 and returning an error exits with code 1,
// unless the error implements ExitCoder, in which case it names its own
// code.
type HandlerFunc func(ctx context.Context, inv *Invocation) error

// Command is the compiled, implementation form of a command that users may
// invoke from the command line, produced by lowering a configuration tree
// with (*xflags.Command).Compile.
//
// Every field is exported, including Ancestry and Root, which would make
// an encoded tree self-referential, and Handler, UsageFunc and the three
// streams, which carry behavior: ir is never encoded, so nothing has to
// be hidden from an encoder to keep any of it out of a document. See the
// package doc for the two-type model this is one half of.
type Command struct {
	Name        string
	Summary     string
	Description string
	Hidden      bool
	ForwardArgs bool

	// Interrupt, if set, is what makes the command an interrupt, and
	// runs in place of Handler: invoking the command ends the parse the
	// way an interrupt flag does, and a check that would otherwise fail
	// the command line -- a required flag missing, an argument count
	// unmet, an option nothing recognizes -- does not stop it from
	// answering. No middleware wraps it. Nil means the command is not
	// an interrupt. See xflags.InterruptCommand.
	Interrupt HandlerFunc

	// FullName is the command's name joined with each ancestor's, from the
	// root down, so a deep subcommand reads as "app remote add" rather
	// than the bare "add" that String returns. Compile computes it top
	// down while lowering.
	FullName string

	// ForwardedValueName and ForwardedUsage name and explain the
	// arguments the command forwards to its handler unparsed -- what
	// follows an interrupt command's name, or a ForwardArgs command's
	// "--" terminator. Both arrive already written for a reader, and
	// both are empty when the program named nothing; a command may
	// forward without naming what it forwards.
	ForwardedValueName string
	ForwardedUsage     string

	FlagGroups  []*FlagGroup
	Subcommands []*Command

	// Ancestry is every command from the root of the tree down to and
	// including this one, which is the commands whose flags are in scope
	// here: a flag is usable from the point its own command is named
	// onward, so what this command accepts is the union of theirs. See
	// docs/adr/path-scoped-flag-names.md.
	//
	// Compile builds it top down while lowering, so nothing reading a
	// compiled tree has to walk back up to reconstruct it. It is
	// derivable from the tree's shape; the command that mounted this one
	// is Ancestry's second to last entry.
	Ancestry []*Command

	// Root is the command at the top of the tree this command belongs to,
	// and is the command itself at the root. Whole-tree work -- validation,
	// and restoring defaults before a parse -- starts here, so that calling
	// Parse on a subcommand still governs the tree it belongs to, and is
	// what requires it to be set. Like Ancestry, it is derivable from the
	// tree's shape.
	Root *Command

	// Handler runs the command once its command line parses
	// successfully, and is never nil: it is the whole of what a command
	// does, assembled while lowering. Whatever the command declared
	// arrives here already wrapped in the wrappers its program put around
	// it, and a command that declared no handler of its own gets one
	// reporting a usage error, since such a command exists only to group
	// its subcommands. Calling it is how a command is run -- unless the
	// command is an interrupt, in which case Interrupt runs instead and
	// this is never called.
	Handler HandlerFunc

	// UsageFunc renders this command's help message in place of the
	// default, and is inherited from the nearest ancestor that set one.
	// Compile resolves it while lowering, so a command carries the
	// renderer it will actually be printed with rather than one the Usage
	// method has to go looking for; it is nil only when no command on the
	// path set one, and Usage falls back to the default renderer.
	UsageFunc UsageFunc

	// Stdin, Stdout and Stderr are the streams resolved for this command
	// when the tree was compiled, and are never nil: each defaults to the
	// matching process stream. Because xflags.Parse compiles a
	// fresh tree on every call, nothing observable changes, though a
	// caller holding a compiled tree across a reassignment of os.Stdout
	// keeps the stream it compiled with.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// String returns the command's own name, unqualified by its ancestry. See
// FullName for the full path from the root.
func (c *Command) String() string { return c.Name }

// Validate checks c and, recursively, each of its subcommands for
// configuration errors, reporting every error found in one run -- a
// malformed tree surfaces its errors in a batch, not one per run.
//
// Validation always covers the whole tree, from Root down, wherever in the
// tree it is called. It checks each command and flag on its own terms:
// whether two flags would answer to the same spelling is settled where
// spelling is, so a tree that passes here may still be rejected by
// (*xflags.Command).Compile, which runs both. A Command produced by
// Compile is already validated.
func (c *Command) Validate() error {
	return validateTree(c.Root)
}

// Usage prints a help message for c to w, using c's UsageFunc, which
// Compile resolved from the nearest ancestor that set one, or the default
// renderer, Usage, when no command on the path did.
func (c *Command) Usage(w io.Writer) error {
	return writeUsage(c, w)
}

// Describe returns c's description, and, recursively, every subcommand
// beneath it. Behavior -- Handler, UsageFunc and the three streams --
// carries nothing to describe and is absent from the result.
//
// The document a program publishes should be rooted at c.Root. Describing
// a subtree is legal but understates what it accepts: a flag is in scope
// for a command from the point its own command is named onward, so a
// command's ancestors hold flags it accepts that are not beneath it here.
func (c *Command) Describe() *desc.Command {
	cmd := &desc.Command{
		Name:        c.Name,
		FullName:    c.FullName,
		Summary:     c.Summary,
		Description: c.Description,
		Hidden:      c.Hidden,
		ForwardArgs: c.ForwardArgs,
	}
	if c.ForwardedValueName != "" {
		cmd.Forwarded = &desc.Forwarded{
			ValueName: c.ForwardedValueName,
			Usage:     c.ForwardedUsage,
		}
	}
	for _, group := range c.FlagGroups {
		cmd.FlagGroups = append(cmd.FlagGroups, group.Describe())
	}
	for _, sub := range c.Subcommands {
		cmd.Subcommands = append(cmd.Subcommands, sub.Describe())
	}
	return cmd
}
