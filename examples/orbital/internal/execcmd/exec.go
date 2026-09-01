// Package execcmd implements "orbital exec --service NAME -- CMD ARGS...",
// the one place orbital forwards raw arguments to something else rather
// than parsing them itself: Command.ForwardArgs makes everything after
// "--" reach the handler unparsed as Invocation.Forwarded.
package execcmd

import (
	"context"
	"fmt"
	"strings"

	"go.hotsrc.dev/climux"
	"go.hotsrc.dev/climux/examples/orbital/internal/middleware"
)

// Command returns the "exec" command.
func Command() *climux.Command {
	var service string
	return climux.NewCommand("exec", "Run a one-off command inside a service's container").
		Middleware(middleware.Audit).
		ForwardArgs().
		Forwarded("cmd", "Command to run inside the container, after --").
		Flags(
			climux.String(&service, "service", "", "Service whose container to exec into").
				Aliases("s").
				Required(),
		).
		HandleFunc(
			func(ctx context.Context, inv *climux.Invocation) error {
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
