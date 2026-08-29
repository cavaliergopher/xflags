package config

import (
	"context"
	"fmt"

	"github.com/cavaliergopher/xflags"
)

// getCommand returns "orbital config get KEY".
func getCommand() *xflags.Command {
	var key string
	return xflags.NewCommand("get", "Print the value of a configuration key").
		Flags(
			xflags.String(&key, "KEY", "", "Configuration key to read").
				Positional().
				Required(),
		).
		HandleFunc(func(ctx context.Context, inv *xflags.Invocation) error {
			v, ok := store[key]
			if !ok {
				// A plain error: nothing about this is a usage mistake the
				// parser could have caught, so it is returned as-is and
				// exits 1 rather than naming its own code.
				return fmt.Errorf("unknown configuration key: %s", key)
			}
			fmt.Fprintln(inv.Stdout, v)
			return nil
		})
}
