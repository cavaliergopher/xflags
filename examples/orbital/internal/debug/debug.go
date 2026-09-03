// Package debug implements a hidden diagnostics command: reachable on the
// command line but left out of help output by Command.Hidden, which is
// how orbital keeps unsupported internal tooling out of the way of
// day-to-day users without actually restricting who can run it.
//
// It registers the command into climux.DefaultRegistry rather than
// exporting a constructor for main.go to call, so a program carries these
// diagnostics by linking the package in and mounting the registry. That
// is how a shared library contributes a subcommand: the program that
// mounts it names nothing.
package debug

import (
	"context"
	"fmt"

	"go.hotsrc.dev/climux"
)

func init() {
	climux.DefaultRegistry.Subcommands(command())
}

// command returns the hidden "debug" command.
func command() *climux.Command {
	return climux.NewCommand("debug", "Internal diagnostics, not for regular use").
		Hidden().
		HandleFunc(func(ctx context.Context, inv *climux.Invocation) error {
			fmt.Fprintln(inv.Stdout, "debug: dumping internal state (stub)")
			return nil
		})
}
