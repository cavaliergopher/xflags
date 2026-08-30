// Package middleware holds the wrappers orbital puts around its handlers:
// a timing trace that every command in the binary runs inside, and an
// audit check that only the commands changing fleet state do.
//
// Both are xflags.Middleware, so neither is registered by the handler it
// wraps. main.go declares Timing on the root, which inherits it down the
// whole tree, and each mutating command declares Audit on itself; a
// read-only command such as "deploy status" declares nothing and so is
// not audited. That is the point of the mechanism here: the platform team
// owns what wraps a handler without touching the team that wrote it.
package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/cavaliergopher/xflags"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/identity"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/telemetry"
)

// Audit refuses to run as the placeholder "anonymous" identity, in place
// of a real authorization check against an actor directory. Returning
// without calling next is how a middleware refuses an invocation.
//
// It reads identity.Actor at call time rather than closing over a copy,
// because the root --actor flag it comes from is not applied until the
// command line parses, which is after every command in the tree was
// built.
func Audit(next xflags.HandlerFunc) xflags.HandlerFunc {
	return func(ctx context.Context, inv *xflags.Invocation) error {
		if identity.Actor == "anonymous" {
			return fmt.Errorf(
				"%s: refusing to run as %q; pass --actor or set ORBITAL_ACTOR",
				inv.Cmd.FullName, identity.Actor,
			)
		}
		return next(ctx, inv)
	}
}

// Timing reports how long the handler took, but only when --trace asked
// for it -- another package's settings, read by value at call time rather
// than copied in at declaration.
func Timing(next xflags.HandlerFunc) xflags.HandlerFunc {
	return func(ctx context.Context, inv *xflags.Invocation) error {
		start := time.Now()
		err := next(ctx, inv)
		if telemetry.Flags.Trace {
			fmt.Fprintf(inv.Stderr, "trace: %s took %s\n",
				inv.Cmd.FullName, time.Since(start))
		}
		return err
	}
}
