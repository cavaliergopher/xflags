# Command lines are POSIX/GNU, not stdlib flag

Status: accepted, 2026-08-23.

## Context

This package is deliberately `flag`-shaped: `Value` is the standard
library's interface, the typed constructors keep its argument order, and a
`flag.FlagSet` can be imported wholesale. That resemblance invites the
assumption that command lines are compatible too, and they are not.

The standard library treats one dash and two as the same thing, so
`-verbose` and `--verbose` both work, and it has no notion of a short name
distinct from a long one. Everything else a user is likely to have met —
git, docker, kubectl, every GNU utility — uses `--long` for names and `-s`
for single characters, and users, generated documentation and agents all
carry that expectation.

The two dialects cannot both be honored by one parser. If a single dash may
introduce a long name, then `-force` is either the flag `force` or the short
flag `f` with the value `orce`, and no amount of lookahead settles it. The
choice has to be made once, at the top.

## Decision

Command lines are POSIX/GNU: `--name` for long flags, `-n` for short ones, a
value either attached (`--name=value`, `-nvalue`, `-n=value`) or given as
the following argument, and `--` terminating flag parsing for commands that
opt into it. Single-dash long names are not accepted.

Compatibility with `flag` is at the API and migration layer only — the
`Value` interface, the constructor signatures, and importing a
`flag.FlagSet` — never at argv. This should be said plainly wherever
migration is documented, because it is the one place the resemblance
misleads.

Other dialects are deferred, not refused. The parser is the only component
that would have to change: it resolves argv against the compiled
description, and everything downstream — the data model, help, generated
docs, machine-readable output — reads the description rather than the
command line. Splitting tokenizing from interpretation, already planned for
its own reasons, is what would make a second dialect cheap. Nobody has asked
for one, so none is built.

## Consequences

- A program migrating from `flag` changes the command lines its own users
  type. Single-dash long flags stop working, and that breakage lands on the
  program's users, not on the author doing the migration. It has to be in
  the migration notes.
- Short-flag grouping (`-abc` meaning `-a -b -c`) is not supported. Today
  everything after the short name is its attached value, so `-abc` is `-a`
  with the value `bc`, and when `-a` is boolean and takes no value the
  remainder falls through as a stray positional. Whether to add grouping is
  a question inside this dialect, and this decision does not settle it.
- Windows slash-style flags and a stdlib-compatible mode are possible later
  without touching the data model, on the strength of the seam described
  above.
- Help output, completion and generated documentation can render flags one
  way, since there is one way.
