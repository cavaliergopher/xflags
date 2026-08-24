# Run reports outcomes as exit codes 0, 1 and 2

Status: accepted, 2026-08-23.

## Context

A caller that cannot read the program's output still has to know what
happened. Shell scripts, CI steps and agents all branch on the exit code,
and the distinction they need is between "you invoked it wrong", which the
caller can fix by changing its arguments, and "it ran and failed", which it
cannot. The two demand different responses: retry with different argv, or
report the fault upwards.

v0's handler returned an `int`, so the code was whatever each program chose
and the library promised nothing. The execution model changed handlers to
return an `error`, which moved the mapping from failure to exit code inside
the library and forced the question.

Prior art is settled enough to follow rather than invent. The standard
library's `flag` package exits with 2 when it cannot parse a command line,
and 2 for usage errors is common across POSIX tooling. Handlers that shell
out to another program have a third case: an exit code already exists and
should survive.

## Decision

`Run` returns one of three codes, and a handler may name any other:

    0  the handler returned nil, or -h or --help was given
    1  the handler returned an error
    2  the command line was wrong, or named a command with no handler

Code 2 covers everything decided before the handler runs — parse errors,
argument errors, and a command that has no handler to run — so a caller
seeing 2 knows nothing was executed.

A handler's error chain is searched with `errors.As` for

```go
type ExitCoder interface {
    error
    ExitCode() int
}
```

and the code it names is used; without one the code is 1. The error is
reported on the command's stderr either way. `Exit(err, code)` wraps an
error with a code, and `UsageErrorf` reports misuse the parser cannot see —
contradictory flags, argument semantics — with the same code the parser
would have used had it been able to see it.

An interface rather than a required error type, because the common case
should stay untyped: most handlers just return errors and mean 1. The
interface also lets errors participate that were never written for this
package, and `*exec.ExitError` already satisfies it, so a handler that
returns a subprocess failure unchanged exits with the child's code and no
wrapping at all.

Help is success. `flag` exits 2 for `-h` when no such flag is defined,
which conflates asking for help with getting the invocation wrong; help was
requested and delivered, so it is 0.

## Consequences

- The three codes are a compatibility promise from v1, as is `ExitCoder`.
  Programs and their callers will encode them.
- A handler wanting a specific code must wrap its error. Everything
  unwrapped is 1, which is the right default and the reason wrapping is
  opt-in.
- `Exit(err, 0)` prints an error to stderr and exits successfully. The
  library does not second-guess a code the handler named, so the
  contradiction is the handler's to avoid.
- An error deep in a chain can decide the exit code from a distance, since
  `errors.As` searches the whole chain. That is what makes `*exec.ExitError`
  work through wrapping, and it means a handler wrapping a subprocess
  failure inherits its code unless it wraps again with its own.
- Usage errors are indistinguishable from each other by code alone. An agent
  that needs to know which flag was wrong reads stderr, or `ArgumentError`
  if it is calling `Parse` itself.
- A parse error and a `UsageErrorf` share a code but not a prefix: the
  first prints `Argument error:` and the second `Error:`, because only the
  parser produces an `ArgumentError`. Anything reading stderr rather than
  the code sees two shapes for one class of failure, which is worth
  reconciling.
