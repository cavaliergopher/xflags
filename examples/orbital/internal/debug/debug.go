// Package debug implements a hidden diagnostics command: reachable on the
// command line but left out of help output by Command.Hidden, which is
// how orbital keeps unsupported internal tooling out of the way of
// day-to-day users without actually restricting who can run it.
package debug

import (
	"context"
	"fmt"

	"go.hotsrc.dev/climux"
)

// Command returns the hidden "debug" command.
func Command() *climux.Command {
	return climux.NewCommand("debug", "Internal diagnostics, not for regular use").
		Hidden().
		HandleFunc(func(ctx context.Context, inv *climux.Invocation) error {
			fmt.Fprintln(inv.Stdout, "debug: dumping internal state (stub)")
			return nil
		})
}
