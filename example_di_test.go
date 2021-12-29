// This example demonstrates a pattern for injecting dependencies into your command handlers.
package xflags

import (
	"context"
	"database/sql"
	"fmt"
	"os"
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
// injects them into fn.
func Wrap(fn func(ctx context.Context, db *sql.DB) error) HandlerFunc {
	return func(args []string) (exitCode int) {
		// build a context
		ctx := context.Background()

		// build a database connection
		var db *sql.DB = nil

		// call the handler with all dependencies
		if err := fn(ctx, db); err != nil {
			fmt.Fprint(os.Stderr, err)
			return 1
		}
		return 0
	}
}

// Get is a custom handler for GetCommand
func Get(ctx context.Context, db *sql.DB) error {
	fmt.Println("Issued a get query")
	return nil
}

// Delete is a custom handler for DeleteCommand
func Delete(ctx context.Context, db *sql.DB) error {
	fmt.Println("Issued a delete query")
	return nil
}

func Example_dependencyInjection() {
	RunWithArgs(DBClient, "get")
	// Output: Issued a get query
}
