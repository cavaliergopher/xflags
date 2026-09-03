# Flag names are scoped to a command path

Status: accepted, 2026-08-23. Implemented 2026-08-26.

## Context

The parser descends the command tree as it reads argv, merging each
command's flags into an active table as its name is seen. A flag is
therefore usable from the point its own command is named onward: parent
flags keep working after a subcommand token, and a subcommand's flags do not
work before one. What that leaves open is the namespace the names live in.

Three answers were considered.

**Per command, which is what the code does today.** Duplicate names are
checked within one command and nowhere else, so a subcommand may declare a
name an ancestor already uses. The parser's table then overwrites the
ancestor's entry as it descends, and one spelling sets two different
variables depending on how deep the command line went. Nothing reports it;
the flag simply stops doing what the reader of the parent's declaration
expects.

**Globally unique, as absl does.** Every name is unique across the binary,
which buys a real prize: a flag can appear anywhere in argv, because there
is no position at which its meaning is unsettled, and the ordering rule
above disappears. The price is that two sibling commands cannot both have
`--force`, and every flag name has to be chosen against every other flag
name in the program. That is a large-organization convention imposed on
every tool that uses the package, including the ones with four flags.

It is also the wrong shape for this package specifically. Commands and flag
groups are written by teams that do not control where they are mounted, so
a global namespace makes a collision between two unrelated packages the
problem of whoever composes the binary — the one person who cannot fix it,
because renaming another team's flag is not theirs to do.

**Unique along a path**, which is the middle position, and the one that
keeps single-pass parsing without the naming cost.

## Decision

A flag name may not repeat anywhere along an ancestor–descendant chain.
Shadowing an ancestor is a configuration error that names both commands.
Commands in different subtrees may reuse names freely, so `--force` on
`delete` and `--force` on `push` are unrelated and both fine.

The check runs where the whole tree is in view, with the rest of validation,
not at construction — a command cannot know its ancestors until it is
mounted.

## Consequences

- There is no debugging trap: within any one invocation, a name means one
  flag bound to one variable.
- Validation becomes ancestry-aware rather than per command, so it costs a
  walk down each path rather than a pass over each command.
- A flag group mounted at two depths of the same path is an error. The fix
  is to mount it once, higher up, which is the arrangement that was meant.
  Mounting the same group in sibling subtrees stays legal.
- The ordering rule survives: `myapp --sub-only sub` is still an error,
  because `--sub-only` is not in the table yet. This is the cost of not
  going global, and it is paid down by the error message rather than by the
  rule — an unknown flag that exists somewhere in the current command's
  subtree reports where it is defined, since the tree is static and fully
  known.
- Flag names cannot be validated in isolation, so a package that exports a
  group cannot prove on its own that mounting it will succeed.
