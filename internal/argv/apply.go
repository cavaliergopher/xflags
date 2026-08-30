package argv

import (
	"os"

	"github.com/cavaliergopher/xflags/ir"
)

// Parse reads args against the compiled command cmd and stores the value
// of each argument in each flag's target. The rules for each flag are
// checked and any errors are returned.
//
// Parse resets every flag reachable from cmd's tree to its default before
// reading any arguments, so parsing the same tree twice yields the same
// result. It does not validate the tree: a tree produced by
// (*xflags.Command).Compile is already validated.
//
// The returned Invocation names cmd, or one of its subcommands if the
// arguments specified one.
//
// If -h or --help are specified, parsing stops there and the returned
// Invocation has HelpRequested set. That is not an error: it is for the
// caller to report the command's usage.
func Parse(cmd *ir.Command, args []string) (*ir.Invocation, error) {
	if err := applyDefaults(rootOf(cmd)); err != nil {
		return nil, err
	}
	return apply(cmd, lex(cmd, args))
}

// rootOf returns the root of the tree cmd belongs to, tolerating a Root
// that was never set: (*xflags.Command).Compile always sets it, but a tree
// built by hand may not, and whole-tree work should not panic on one.
func rootOf(cmd *ir.Command) *ir.Command {
	if cmd.Root != nil {
		return cmd.Root
	}
	return cmd
}

// applyDefaults restores every flag reachable from c, and from every
// descendant of c, to its default value. Parse calls it on the root, so a
// parse governs the whole tree however deep it was called. Parse calls
// this before lexing, which is what keeps repeat parses idempotent:
// Compile lowers a tree fresh on every call but never mutates it, so
// restoring defaults is the only step that does, and it runs anew each
// time Parse does.
//
// Values are set directly, bypassing Flag.Set, so a ValidateFunc never
// runs against a default.
func applyDefaults(c *ir.Command) error {
	for _, group := range c.FlagGroups {
		for _, f := range group.Flags {
			if r, ok := f.Value.(ir.Resetter); ok {
				r.Reset()
				continue
			}
			if !f.HasDefault {
				continue
			}
			if err := f.Value.Set(f.Default); err != nil {
				return ir.NewConfigErrorf(err, nil, f, "cannot restore default value: %v", err)
			}
		}
	}
	for _, sub := range c.Subcommands {
		if err := applyDefaults(sub); err != nil {
			return err
		}
	}
	return nil
}

// apply walks res's instructions in order against the commands and flags
// they name, and returns the resulting Invocation. It is what is left of
// parsing once lex has resolved argv: Set, environment variables and
// NArgs validation. Everything with an effect happens here, and nothing
// here decides what argv means -- lex has already done that, before apply
// ever runs.
//
// A help instruction anywhere in res wins over every recorded lex error,
// discarding them: this is what lets "cmd --bogus --help" print help
// instead of failing, since a user who is asking for help does not need
// their typo reported too. apply walks instructions up to the first help
// instruction it finds -- earlier Set and Dispatch instructions still run,
// so flags given before --help take effect -- and stops there: env vars are
// not read and NArgs is not checked.
//
// Otherwise, any recorded lex error stops apply before it starts: nothing
// is mutated unless every argument in argv resolved. An error from Set or
// a ValidateFunc can still stop apply partway through, since undoing a
// caller-owned variable it already wrote is not this package's to do.
func apply(root *ir.Command, res lexResult) (*ir.Invocation, error) {
	helpAt := -1
	for i, instr := range res.instructions {
		if instr.kind == instHelp {
			helpAt = i
			break
		}
	}
	if helpAt == -1 && len(res.errs) > 0 {
		return nil, res.errs[0]
	}

	limit := len(res.instructions)
	if helpAt != -1 {
		limit = helpAt
	}

	active := root
	path := []*ir.Command{root}
	counts := make(map[*ir.Flag]int)
	var forwarded []string
	for _, instr := range res.instructions[:limit] {
		switch instr.kind {
		case instSet:
			if err := setFlag(active, instr.flag, instr.value); err != nil {
				return nil, err
			}
			counts[instr.flag]++
		case instDispatch:
			active = instr.cmd
			path = append(path, active)
		case instForward:
			forwarded = instr.forwarded
		}
	}

	if helpAt != -1 {
		return invocationFor(active, path, forwarded, true), nil
	}
	if err := applyEnvVars(path, counts); err != nil {
		return nil, err
	}
	if err := validateNArgs(active, path, counts); err != nil {
		return nil, err
	}
	return invocationFor(active, path, forwarded, false), nil
}

// setFlag sets f's value to token, wrapping a failure the same way it
// always has: the flag alone, or the flag followed by the error it wraps.
// active is the command reported on the error, the one in scope when the
// instruction naming f was lexed.
func setFlag(active *ir.Command, f *ir.Flag, token string) error {
	if err := f.Set(token); err != nil {
		return ir.NewArgumentErrorf(err, active, f, token, "%s", f)
	}
	return nil
}

// applyEnvVars fills every flag along path from its environment variable,
// for whatever counts has no occurrence of yet, then counts it as seen so
// validateNArgs sees it satisfied.
//
// path order, then group order, then declaration order within each command:
// this is deterministic.
func applyEnvVars(path []*ir.Command, counts map[*ir.Flag]int) error {
	for _, cmd := range path {
		for _, group := range cmd.FlagGroups {
			for _, f := range group.Flags {
				if f.EnvVar == "" || counts[f] > 0 {
					continue
				}
				s, ok := os.LookupEnv(f.EnvVar)
				if !ok {
					continue
				}
				if err := setFlag(cmd, f, s); err != nil {
					return err
				}
				counts[f]++
			}
		}
	}
	return nil
}

// validateNArgs verifies each flag along path was given as many times as it
// requires. Every flag that became active along the descended path is
// checked, so an ancestor's Required flag still binds when a subcommand is
// invoked.
//
// active, the deepest command reached, is what every error here is
// reported against, whichever command in path actually declared the
// offending flag -- the same command instructions were applied against
// throughout, since checking counts is the last of the work apply does.
//
// The offending flag goes at the end of these messages, which is where
// Go's flag package, argparse and getopt all put it when nothing follows
// it. A flag leads the message only when a wrapped error follows, as in
// "--ip: invalid IP: 256.0.0.1", where it scopes what comes after the
// colon; see docs/adr/human-readable-errors.md.
func validateNArgs(active *ir.Command, path []*ir.Command, counts map[*ir.Flag]int) error {
	for _, cmd := range path {
		for _, group := range cmd.FlagGroups {
			for _, f := range group.Flags {
				n := counts[f]
				if f.MinCount > 0 && n < f.MinCount {
					switch {
					case f.MinCount == 1:
						return ir.NewArgumentErrorf(nil, active, f, "",
							"missing required argument: %s", f)
					case f.MinCount == f.MaxCount:
						return ir.NewArgumentErrorf(nil, active, f, "",
							"expected %d arguments, got %d: %s",
							f.MinCount, n, f)
					default:
						return ir.NewArgumentErrorf(nil, active, f, "",
							"expected at least %d arguments, got %d: %s",
							f.MinCount, n, f)
					}
				}
				if f.MaxCount > 0 && n > f.MaxCount {
					return ir.NewArgumentErrorf(nil, active, f, "",
						"argument specified too many times: %s", f)
				}
			}
		}
	}
	return nil
}

// invocationFor returns the Invocation apply reports for cmd having become
// active, naming every command in path from the one Parse was called on to
// cmd itself.
func invocationFor(cmd *ir.Command, path []*ir.Command, forwarded []string, helpRequested bool) *ir.Invocation {
	names := make([]string, len(path))
	for i, c := range path {
		names[i] = c.Name
	}
	return &ir.Invocation{
		Cmd:           cmd,
		Path:          names,
		Forwarded:     forwarded,
		HelpRequested: helpRequested,
		Stdin:         cmd.Stdin,
		Stdout:        cmd.Stdout,
		Stderr:        cmd.Stderr,
	}
}
