# Registries carry library contributions

Status: accepted, 2026-09-03.

## Context

A large program's command line is written by more than one team. The
program's author composes the binary; the libraries linked into it own
flags bound to variables those libraries own, the middleware that gives
those flags meaning, and sometimes a subcommand of their own.

What a library offers is a bundle rather than a set of parts. A
`--timeout` flag is inert without the wrapper that reads it and bounds the
handler, so a program able to mount the flag and miss the wrapper gets a
flag that silently does nothing, which is worse than not having it. The
same holds for a diagnostics subcommand and the flags it reads.

Contribution therefore needs a unit that carries all three kinds of thing
and travels as one, and a program needs a way to accept that unit without
naming what is in it — naming the parts is exactly what lets a bundle be
split.

## Decision

A `Registry` is that unit. It carries flag groups, middleware and
subcommands; a library registers into one, and `Command.Mount` takes
everything a registry holds.

- **A registry is not a node in the command tree.** It holds
  contributions and claims nothing. `Registry.Subcommands` sets no
  parent, unlike `Command.Subcommands`, and the mounting command is the
  parent in the compiled tree alone.

  A `Command` carries all three kinds of contribution already, so the
  standing alternative is for the shared registry to be one. It cannot
  be. `Command.Subcommands` parents a command it accepts, so a
  contribution registered into a command-shaped registry names that
  registry as its parent; the program mounting it then claims the same
  command as a child, and the parent check in `lower` reports the
  mismatch. Defining the mount to re-parent instead mutates the
  contributed commands, which costs `Compile` its purity and makes a
  registry mountable exactly once — and a tree is built per run, not once
  per process, per `a-tree-reads-one-command-line.md`. Not being a node
  avoids all of it, and is what lets two programs, or two tests, mount
  the same registry without either writing to it.

- **Registered middleware wraps outside the mounting command's own.**
  Ordering from outermost in: what the command inherited from its
  ancestors, then what a mounted registry contributed, then what the
  command declared itself. A wrapper registered beside the flag it reads
  has to bound the handlers the mounting program wrote, or a program
  declares its way out of the wrapper while keeping the flag.

- **Everything else follows what the mounting command declared.** Flag
  groups after its own, subcommands after its own children.

- **Two of a command's children answering to one name are a
  configuration error.** Dispatch resolves a name to a single command, so
  the second would be unreachable and nothing in the tree says which was
  meant. Ordering alone cannot keep a registry from taking a name the
  program declared, because resolution does not scan in order; this check
  is what does. Flag groups are not claimed the same way, since a
  subcommand mounting the same registry as an ancestor is the ordinary way
  two teams reach one registry.

- **A registered command must arrive unparented.** One already mounted in
  a tree of its own would be lowered under two parents, so mounting a
  registry that holds one is a configuration error.

- **`Middleware` stays a func type.** A wrapper that carries state is a
  struct with a `Wrap` method, registered as the method value. An
  interface would buy nothing a method value does not already give, and
  would force an adapter around every plain wrapper; see
  `middleware-wraps-handlers.md`.

- **`DefaultRegistry` is for the packages that make up one program.** It
  is the well-known registry, not the only one: `Mount` is variadic, and a
  registry mounted on a subcommand reaches that subtree alone. A library
  published for programs it does not own exports a registry of its own, so
  that linking a package in is not by itself a decision about a consumer's
  command line. Acquiring a subcommand, or a wrapper that can refuse
  invocations, belongs to a program rather than to its dependency graph.

- **Registration is by method on a registry, with no package-level
  shorthand for the well-known one.** A shorthand covering one of the
  three kinds leaves a library writing one call in a shape its neighbours
  do not share, and one covering all three makes the global registry the
  headline API, against the rule above.

## Consequences

A library ships a flag and the wrapper honoring it as one registration, so
the two cannot be mounted apart, and a shared subcommand needs no wiring
in the programs carrying it.

`Registry` and `Command` share three method names. That is deliberate: the
same name means the same thing, and the one difference — that a registry
claims no parent — is the place the two must not be confused.

What a registry holds is read afresh on each call and never written back,
as a mounted flag group is. Nothing a program mounts is observable in the
registry afterwards, which is what makes a registry safe to share.

A program's command line is only as trustworthy as the registries it
mounts. `Mount` is where a program consents to a contribution, and there
is no consent below it.
