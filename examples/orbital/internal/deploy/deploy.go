// Package deploy implements "orbital deploy status|run", the platform
// team's rollout commands. Each depends on a *fleet.Client, passed in as a
// constructor parameter rather than reached through a package global --
// the same closure-based dependency injection as example_di_test.go in
// the xflags module root.
package deploy

import (
	"github.com/cavaliergopher/xflags"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/fleet"
)

// Command returns the "deploy" subcommand, wired to client.
func Command(client *fleet.Client) *xflags.Command {
	return xflags.NewCommand("deploy", "Deploy and inspect services running on the fleet").
		Description(
			"Subcommands here talk to the fleet API on behalf of the caller\n"+
				"named by --actor. \"run\" changes what is deployed and declares\n"+
				"an audit middleware; \"status\" only reads and does not.",
		).
		Subcommands(statusCommand(client), runCommand(client))
}
