// Package execcmd implements "orbital exec --service NAME -- CMD ARGS...",
// the one place orbital forwards raw arguments to something else rather
// than parsing them itself: Command.ForwardArgs makes everything after
// "--" reach the handler unparsed as Invocation.Forwarded.
package execcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/cavaliergopher/xflags"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/middleware"
)

// Command returns the "exec" command.
func Command() *xflags.Command {
	var service string
	return xflags.NewCommand("exec", "Run a one-off command inside a service's container").
		Middleware(middleware.Audit).
		ForwardArgs().
		Flags(
			xflags.String(&service, "service", "", "Service whose container to exec into").
				Aliases("s").
				Required(),
		).
		HandleFunc(
			func(ctx context.Context, inv *xflags.Invocation) error {
				if len(inv.Forwarded) == 0 {
					return fmt.Errorf(
						"no command given; usage: orbital exec --service NAME -- CMD [ARGS...]")
				}
				fmt.Fprintf(inv.Stdout, "%s: would run: %s\n",
					service, strings.Join(inv.Forwarded, " "))
				return nil
			},
		)
}
