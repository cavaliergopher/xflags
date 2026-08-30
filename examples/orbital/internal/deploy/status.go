package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/cavaliergopher/xflags"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/fleet"
)

// statusCommand returns "orbital deploy status SERVICE". It is read-only,
// so it declares no middleware.Audit; it still runs inside the timing
// trace the root declared, which every command in the tree inherits.
func statusCommand(client *fleet.Client) *xflags.Command {
	var (
		service string
		timeout time.Duration
	)
	return xflags.NewCommand("status", "Show the rollout status of a service").
		Flags(
			xflags.String(&service, "SERVICE", "", "Service to inspect").
				Positional().
				Required(),
			xflags.Duration(&timeout, "timeout", 5*time.Second, "How long to wait for the fleet API").
				ShowDefault(),
		).
		HandleFunc(func(ctx context.Context, inv *xflags.Invocation) error {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(50 * time.Millisecond): // stand-in for a fleet API round trip
			}
			fmt.Fprintf(inv.Stdout, "%s: healthy (region=%s)\n", service, client.Region)
			return nil
		})
}
