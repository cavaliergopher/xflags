// Package legacy stands in for an older internal library that declared its
// options with Go's flag package before orbital adopted xflags.
// xflags.FromFlagSet lets orbital mount those options as an ordinary flag
// group without the library itself changing.
package legacy

import (
	"flag"

	"github.com/cavaliergopher/xflags"
)

// FlagSet holds the options this stand-in legacy library declares itself,
// exactly as it would with Go's flag package.
var FlagSet = flag.NewFlagSet("legacy", flag.ContinueOnError)

var metricsAddr = FlagSet.String(
	"legacy-metrics-addr",
	":9090",
	"Address the legacy metrics sidecar listens on (superseded by --trace)",
)

// FlagGroup imports FlagSet into an xflags FlagGroup for mounting on a
// command with Command.FlagGroups.
func FlagGroup() *xflags.FlagGroup {
	return xflags.FromFlagSet("legacy", "Legacy options (deprecated)", FlagSet)
}

// MetricsAddr returns the configured legacy metrics address.
func MetricsAddr() string { return *metricsAddr }
