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

Command lines are POSIX/GNU: `--name` for long flags, `-n` for short ones,
short flags grouping into one argument (`-abc`), a value either attached
(`--name=value`, `-nvalue`, `-n=value`) or given as the following argument,
and `--` terminating flag parsing for commands that opt into it.
Single-dash long names are not accepted. Which parts of POSIX this adopts,
and where it departs, is settled in `posix-argument-conventions.md`.

Compatibility with `flag` is at the API and migration layer only — the
`Value` interface, the constructor signatures, and importing a
`flag.FlagSet` — never at argv. This should be said plainly wherever
migration is documented, because it is the one place the resemblance
misleads.

Other dialects are deferred, not refused. Nothing in the model names a
POSIX category: a flag holds a list of names, and `ir.FormOf` is the one
place a name becomes a command line spelling, from the shape of the name
rather than the slot it was declared in. Everything that matches, prints or
completes a flag reads the spellings it produced, in `ir.Flag.Forms`, so a
second dialect replaces the speller and the matcher and nothing else. The
design for that seam is in `wip/lexer.md`; none of it is built.

## Consequences

- A program migrating from `flag` changes the command lines its own users
  type. Single-dash long flags stop working, and that breakage lands on the
  program's users, not on the author doing the migration. It has to be in
  the migration notes.
- Windows slash-style flags and a stdlib-compatible mode are possible later
  on the strength of the seam above. The data model paid for that once, in
  dropping its short and long names for a list; it does not pay again per
  dialect.
- Help output, completion and generated documentation render flags from one
  source rather than in one way: each reads the forms the dialect produced,
  and none of them spells a flag itself.
- What a dialect matches may exceed what anything renders, since a dialect
  may match by rule — without regard to case, say. No rendered list is a
  complete account of what the parser accepts.
