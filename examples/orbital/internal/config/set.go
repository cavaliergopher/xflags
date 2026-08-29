package config

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cavaliergopher/xflags"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/identity"
	"github.com/cavaliergopher/xflags/examples/orbital/internal/middleware"
)

// setCommand returns "orbital config set KEY VALUE". It mutates the local
// store, so its handler goes through middleware.Chain like the other
// mutating commands.
func setCommand() *xflags.Command {
	var key, value string
	return xflags.NewCommand("set", "Set a configuration key to a value").
		Flags(
			xflags.String(&key, "KEY", "", "Configuration key to set").
				Positional().
				Required().
				Validate(validKey),
			xflags.String(&value, "VALUE", "", "New value").
				Positional().
				Required(),
		).
		HandleFunc(middleware.Chain(&identity.Actor,
			func(ctx context.Context, inv *xflags.Invocation) error {
				store[key] = value
				fmt.Fprintf(inv.Stdout, "%s = %s\n", key, value)
				return nil
			},
		))
}

// validKey rejects a key the store doesn't already recognize, naming the
// legal ones so a typo is easy to fix.
func validKey(arg string) error {
	if _, ok := store[arg]; ok {
		return nil
	}
	keys := make([]string, 0, len(store))
	for k := range store {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Errorf("unknown key %q, expected one of: %s", arg, strings.Join(keys, ", "))
}
