# Streams are Run's environment

Status: accepted, 2026-09-01.

## Context

climux prints on its own behalf: help, errors, a version, shell completion
replies. Where that output goes must be injectable -- a test wants to read
it, an embedding program wants to place it -- which is the need
`flag.FlagSet.SetOutput` answers in the stdlib.

The previous design went further: `Command.Stdin`, `Command.Stdout` and
`Command.Stderr` setters on every command, inherited down the tree and
resolved by `Compile`. Three problems emerged:

- Per-command configuration answers no question. Every print the library
  makes belongs to one dialog per process, between the program and the
  operator; no command ever needs a different answer than the root, so the
  per-node setters, their inheritance walk, and the sharp edge of a setter
  that is only meaningful at the root all paid for nothing.
- A tree's streams could not be trusted while the tree itself failed to
  compile, so config errors needed a separate untrusted-stream fallback
  path in reporting.
- The setters invited a promise the tree cannot keep. `Command.Stdout`
  read as "whatever this command writes", but a handler reaching for
  `os.Stdout` escapes any redirection, and nothing detects it.

The stdlib also sets a precedent on write failures: `flag` discards the
error on every print to its configured output and lets the exit code say
what happened to the command line, not to the report.

## Decision

Streams are process environment, injected once at the entry point,
alongside argv:

	Run(ctx, cmd)                          // os.Args, and the process streams
	Run(ctx, cmd, WithArgs("sub", "-v"),
	    WithStdout(&out), WithStderr(&err))

`RunWithArgs` folds into `WithArgs`. `Dispatch` takes the same options,
being `Run` without the reporting and the exit code.

An option is what distinguishes a command line the caller did not supply
from one it supplied as empty -- `Run(ctx, cmd)` reads `os.Args`, and
`Run(ctx, cmd, WithArgs())` reads nothing -- which a variadic `args
...string` could not express on the one entry point that defaults to the
process.

`Invocation` keeps `Stdin`, `Stdout` and `Stderr`, and the entry point
fills them in. The parser leaves them nil, having no business knowing
what a process was given. A handler goes on writing `inv.Stdout`, so a
caller redirecting a command captures what it writes, and an interrupt is
an ordinary `HandlerFunc` needing no writer passed beside it.

Nothing enforces that a handler uses them. That is a convention the
library teaches and a program keeps, not a guarantee the library makes:
what a command team prints past the invocation is between them and
whoever composes the binary.

Write failures are ignored, per `flag`'s precedent: a report that cannot
be written never changes the exit code. Config errors report on Run's
stderr -- it comes from the caller rather than from the tree, so it is
trustworthy even when the tree is not.

## Consequences

- One entry point. Tests and embedders pass options instead of mutating
  the tree, and the same tree runs under any environment.
- The compile-time inheritance walk, the stream fields on `ir.Command`,
  and the fallback-to-os.Stderr reporting path are gone.
- `WithStdin` exists because `Invocation.Stdin` does. The library reads no
  input of its own, so this is the one option that serves only handlers --
  and a handler that reads input needs a redirectable source for the same
  reason it needs a redirectable sink.
- A program that wants a command's output somewhere else for one run says
  so at that run, rather than mutating a tree that other teams also mount.
- Breaking: `Command.Stdin`, `Command.Stdout`, `Command.Stderr`, the
  stream fields on `ir.Command`, and `RunWithArgs` are removed, and
  `Dispatch` takes options in place of arguments. The package is pre-v1;
  no migration is offered.
