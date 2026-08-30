package deploy

import (
	"context"
	"fmt"

	"github.com/cavaliergopher/xflags"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/fleet"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/middleware"
)

// runCommand returns "orbital deploy run", the one command in this binary
// that changes what is running, so it declares middleware.Audit. The
// timing trace it also runs inside is the root's, inherited.
func runCommand(client *fleet.Client) *xflags.Command {
	var (
		service    string
		version    string
		env        string
		strategy   string
		tags       []string
		confirm    bool
		skipHealth bool
	)
	return xflags.NewCommand("run", "Roll out a new version of a service").
		Middleware(middleware.Audit).
		Flags(
			xflags.String(&service, "service", "", "Service to deploy").
				Aliases("s").
				Required(),
			xflags.String(&version, "version", "", "Version to deploy, such as a git SHA").
				Required().
				Validate(validVersion),
			xflags.String(&env, "env", "staging", "Environment to deploy to").
				Choices("staging", "production").
				ShowDefault(),
			xflags.String(&strategy, "strategy", "rolling", "Rollout strategy").
				Choices("rolling", "blue-green", "canary").
				ShowDefault(),
			xflags.Strings(&tags, "tag", nil, "Metadata tag to attach to this rollout (repeatable)").
				NArgs(0, 5),
			xflags.Bool(&confirm, "confirm", false, "Confirm a deploy to production"),
			xflags.Bool(&skipHealth, "unsafe-skip-health-checks", false, "Skip post-deploy health checks").
				Hidden(),
		).
		HandleFunc(
			func(ctx context.Context, inv *xflags.Invocation) error {
				if env == "production" && !confirm {
					// A misuse the parser could not catch on its own --
					// two flags whose combination matters -- names its
					// own exit code rather than the generic failure one.
					return xflags.Exitf(4,
						"refusing to deploy %s to production without --confirm", service)
				}
				if err := client.Deploy(ctx, service, version); err != nil {
					return err
				}
				fmt.Fprintf(inv.Stdout, "%s: deployed %s using the %s strategy (tags: %v)\n",
					service, version, strategy, tags)
				if skipHealth {
					fmt.Fprintln(inv.Stderr, "warning: health checks skipped (--unsafe-skip-health-checks)")
				}
				return nil
			},
		)
}

// validVersion is a stand-in for real version validation.
func validVersion(arg string) error {
	if len(arg) < 4 {
		return fmt.Errorf("version %q is too short to identify a build", arg)
	}
	return nil
}
