// Command orbital is a fictional platform team's deploy and operations
// CLI. It shows how a large engineering org assembles one xflags binary
// out of packages owned by different teams: main only wires the tree
// together, and each internal package owns its own commands, flags and
// handlers.
package main

import (
	"context"
	"os"

	"github.com/cavaliergopher/xflags"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/config"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/debug"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/deploy"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/execcmd"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/fleet"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/identity"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/legacy"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/logscmd"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/middleware"
)

// version is what a build stamps into the binary. Both --version and the
// "version" subcommand print it, so the two never disagree.
const version = "1.4.2"

func main() {
	client := fleet.NewClient("us-west-2")

	root := xflags.NewCommand("orbital", "Deploy and operate services on the fleet").
		// Declared first, by convention, so both head the list of
		// options. Both are interrupts, so both answer before --actor is
		// missed: "orbital --version" works without an identity.
		HelpFlag().
		VersionFlag(version).
		EnableCompletion().
		// Declared once here and inherited by every command in the tree,
		// so --trace times whichever one runs and --out redirects it. The
		// audit check is not: only the commands that change fleet state
		// declare it.
		Middleware(middleware.Timing, middleware.Output).
		Description(
			"orbital is the platform team's command line front end to the\n"+
				"fleet API. Each subcommand below is owned and versioned by the\n"+
				"team named in its help text; orbital itself only assembles\n"+
				"them into one binary.",
		).
		FlagGroups(legacy.FlagGroup()).
		GroupSets(xflags.CommandLine).
		Flags(
			xflags.String(&identity.Actor, "actor", "", "Identity performing this action, recorded for the audit trail").
				Required().
				Env("ORBITAL_ACTOR"),
			middleware.OutputFlag(),
		).
		Subcommands(
			config.Command(),
			deploy.Command(client),
			logscmd.Command(client),
			execcmd.Command(),
			debug.Command(),
			// An ordinary command, unlike the flag above, so the tree's
			// rules still apply: "orbital version" wants --actor where
			// "orbital --version" does not.
			xflags.VersionCommand(version),
		)

	ctx, stop := xflags.NotifyContext(context.Background())
	defer stop()
	os.Exit(xflags.Run(ctx, root))
}
