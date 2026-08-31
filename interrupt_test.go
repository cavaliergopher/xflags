package xflags

import (
	"context"
	"testing"
)

// asInterrupt marks cmd as an interrupt the way a library constructor
// such as VersionCommand does, for a test that needs one shaped to its
// own scenario rather than reaching for VersionCommand itself.
func asInterrupt(cmd *Command) *Command {
	cmd.interrupt = true
	return cmd
}

// TestInterruptCommandSkipsAncestorRequiredFlag asserts that naming an
// interrupt subcommand answers even when an ancestor's Required flag is
// missing, the way an interrupt flag already does; see
// TestHelpWinsOverAnEarlierArgumentError in parser_test.go for the flag
// case this mirrors.
func TestInterruptCommandSkipsAncestorRequiredFlag(t *testing.T) {
	var tr tracer
	app := NewCommand("app", "").
		Flags(String(new(string), "name", "", "").Required()).
		Subcommands(asInterrupt(NewCommand("version", "").HandleFunc(tr.handler("version", nil))))

	if err := Dispatch(context.Background(), app, "version"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := tr.String(), "version"; got != want {
		t.Errorf("steps = %q, want %q", got, want)
	}
}

// TestInterruptCommandSkipsMiddleware asserts that middleware declared on
// an ancestor does not wrap an interrupt command's handler, the way none
// wraps a request for help.
func TestInterruptCommandSkipsMiddleware(t *testing.T) {
	var tr tracer
	app := NewCommand("app", "").
		Middleware(tr.step("root")).
		Subcommands(asInterrupt(NewCommand("version", "").HandleFunc(tr.handler("handler", nil))))

	if err := Dispatch(context.Background(), app, "version"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := tr.String(), "handler"; got != want {
		t.Errorf("steps = %q, want %q", got, want)
	}
}

// TestNonInterruptSiblingStillEnforcesRules asserts that a sibling which
// is not an interrupt is unaffected by one that is: it still enforces an
// ancestor's Required flag and still runs the inherited middleware.
func TestNonInterruptSiblingStillEnforcesRules(t *testing.T) {
	var tr tracer
	app := NewCommand("app", "").
		Flags(String(new(string), "name", "", "").Required()).
		Middleware(tr.step("root")).
		Subcommands(
			asInterrupt(NewCommand("version", "").HandleFunc(tr.handler("version", nil))),
			NewCommand("sibling", "").HandleFunc(tr.handler("sibling", nil)),
		)

	if err := Dispatch(context.Background(), app, "version"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := tr.String(), "version"; got != want {
		t.Errorf("steps = %q, want %q", got, want)
	}

	tr.steps = nil
	if _, err := Parse(app, "sibling"); err == nil {
		t.Fatal("expected error, got nil")
	}

	tr.steps = nil
	if err := Dispatch(context.Background(), app, "--name=x", "sibling"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := tr.String(), "root in, sibling, root out"; got != want {
		t.Errorf("steps = %q, want %q", got, want)
	}
}

// TestInterruptCommandIgnoresItsOwnBadFlag decides the shape of "a wrong
// flag given to the interrupt command itself": mirroring "cmd --bogus
// --help", where an interrupt flag answers a line carrying an unrelated
// mistake, an option the interrupt command does not recognize does not
// stop it running either -- a lex error is discarded the same way
// whatever command on the line it names.
func TestInterruptCommandIgnoresItsOwnBadFlag(t *testing.T) {
	var tr tracer
	app := NewCommand("app", "").
		Subcommands(asInterrupt(NewCommand("version", "").HandleFunc(tr.handler("version", nil))))

	if err := Dispatch(context.Background(), app, "version", "--nonexistent"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := tr.String(), "version"; got != want {
		t.Errorf("steps = %q, want %q", got, want)
	}
}
