// Package logscmd implements "orbital logs SERVICE...", which takes an
// unbounded positional argument -- at least one service, with no fixed
// upper bound -- and shows a handler checking ctx.Done() the way a real
// streaming implementation would. The positional also demonstrates
// dynamic completion: Flag.Complete asks the fleet client which services
// exist, so tab completion tracks the fleet rather than a hardcoded list.
package logscmd

import (
	"context"
	"fmt"
	"time"

	"github.com/cavaliergopher/xflags"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/fleet"
	"github.com/cavaliergopher/xflags/ir"
)

// Command returns the "logs" command.
func Command(client *fleet.Client) *xflags.Command {
	var (
		services []string
		follow   bool
		since    time.Duration
	)
	return xflags.NewCommand("logs", "Print recent log lines for one or more services").
		Flags(
			xflags.Strings(&services, "SERVICE", nil, "Services to tail").
				Positional().
				NArgs(1, 0).
				Complete(func(inv *xflags.Invocation, word string) ([]string, ir.CompDirective) {
					return client.Services(), ir.CompNoFileComp
				}),
			xflags.Bool(&follow, "follow", false, "Keep streaming until interrupted").
				Aliases("f"),
			xflags.Duration(&since, "since", 10*time.Minute, "How far back to start showing logs").
				ShowDefault(),
		).
		HandleFunc(func(ctx context.Context, inv *xflags.Invocation) error {
			for _, svc := range services {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				fmt.Fprintf(inv.Stdout, "%s: showing log lines from the last %s\n", svc, since)
			}
			if follow {
				// A real implementation would loop here, selecting on
				// ctx.Done() against the interrupt NotifyContext installs,
				// rather than returning once the backlog is printed.
				fmt.Fprintln(inv.Stdout, "(--follow: this stub does not actually stream)")
			}
			return nil
		})
}
