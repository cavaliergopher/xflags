// Package identity holds the audit identity that orbital threads through
// its command tree: the root --actor flag sets it once, and any package's
// middleware can read it back without importing main.
package identity

// Actor is set from the --actor global flag on the root command. It names
// who is running orbital, for the audit check in
// examples/orbital/internal/middleware.
var Actor string
