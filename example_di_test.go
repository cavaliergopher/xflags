// This example demonstrates a pattern for injecting dependencies into your command handlers.
package xflags

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// DBClient is our main command with two subcommands.
var DBClient = NewCommand("db-client", "query a database").
	Subcommands(
		GetCommand,
		DeleteCommand,
	)

// GetCommand is a subcommand with dependencies injected into the handler by Wrap.
var GetCommand = NewCommand("get", "Get DB resources").
	HandleFunc(Wrap(Get))

// DeleteCommand is a subcommand with dependecnies injected into the handler by Wrap.
var DeleteCommand = NewCommand("delete", "Delete DB resources").
	HandleFunc(Wrap(Delete))

// Wrap returns a HandlerFunc that initialises common dependencies for command handlers and then
// injects them into fn. The invocation is passed along too, so a handler behind a wrapper can
// still tell how it was called.
func Wrap(fn func(ctx context.Context, inv *Invocation, db *sql.DB) error) HandlerFunc {
	return func(ctx context.Context, inv *Invocation) error {
		// build a database connection
		var db *sql.DB = nil

		// call the handler with the invocation and all dependencies
		return fn(ctx, inv, db)
	}
}

// Get is a custom handler for GetCommand
func Get(ctx context.Context, inv *Invocation, db *sql.DB) error {
	fmt.Fprintf(inv.Stdout, "%s: issued a get query\n", strings.Join(inv.Path, " "))
	return nil
}

// Delete is a custom handler for DeleteCommand
func Delete(ctx context.Context, inv *Invocation, db *sql.DB) error {
	fmt.Fprintf(inv.Stdout, "%s: issued a delete query\n", strings.Join(inv.Path, " "))
	return nil
}

func Example_dependencyInjection() {
	RunWithArgs(context.Background(), DBClient, "get")
	// Output: db-client get: issued a get query
}
