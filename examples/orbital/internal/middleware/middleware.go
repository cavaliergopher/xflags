// Package middleware demonstrates the same Wrap idiom as xflags' own
// dependency-injection example (see example_di_test.go in the module
// root): a function that returns a HandlerFunc closing over whatever the
// handler needs -- here an audit check and a timing trace -- rather than
// a framework-level middleware type. Commands that mutate fleet state
// register their handler through Chain; read-only commands do not.
package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/cavaliergopher/xflags"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/telemetry"
)

// Chain wraps fn with an audit check on actor and a timing trace.
func Chain(actor *string, fn xflags.HandlerFunc) xflags.HandlerFunc {
	return withTiming(withAudit(actor, fn))
}

// withAudit refuses to run as the placeholder "anonymous" identity, in
// place of a real authorization check against an actor directory.
func withAudit(actor *string, next xflags.HandlerFunc) xflags.HandlerFunc {
	return func(ctx context.Context, inv *xflags.Invocation) error {
		if *actor == "anonymous" {
			return fmt.Errorf(
				"%s: refusing to run as %q; pass --actor or set ORBITAL_ACTOR",
				inv.Cmd.FullName, *actor,
			)
		}
		return next(ctx, inv)
	}
}

// withTiming reports how long the handler took, but only when --trace
// asked for it -- another package's settings, read by value at call time
// rather than copied in at registration.
func withTiming(next xflags.HandlerFunc) xflags.HandlerFunc {
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
