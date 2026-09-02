# Run reports outcomes as exit codes 0, 1 and 2

Status: accepted, 2026-08-23.
Amended 2026-08-26: the constant for code 2 is `ExitCodeUsage`. It was
`ExitCodeBadArgument`, which named only one of the three cases below.

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
the library and forced the question. That history isn't a constraint here:
adoption of v0 is effectively zero, the API is pre-v1, and it is free to
break until v1.0.0 ships.

Prior art is settled enough to follow rather than invent. The standard
library's `flag` package exits with 2 when it cannot parse a command line,
and 2 for usage errors is common across POSIX tooling. Handlers that shell
out to another program have a third case: an exit code already exists and
should survive.

## Decision

`Run` returns one of three codes, and a handler may name any other:

    0  the handler returned nil, or -h or --help was given
    1  the handler returned an error
    2  the command line or the command tree was wrong, or there is no handler

Code 2 covers everything decided before the handler runs — configuration
errors, parse errors, argument errors, and a command that has no handler to
run — so a caller seeing 2 knows nothing was executed. A misconfigured tree
is a defect in the program rather than in the command line, but it is still
decided before the handler runs and still means nothing was executed, which
is what the code promises.

A handler's error chain is searched with `errors.As` for

```go
type ExitCoder interface {
    error
    ExitCode() int
}
```

and the code it names is used; without one the code is 1. The error is
reported on the command's stderr either way. `Exit(code, err)` wraps an
error with a code — a handler discovering misuse the parser cannot see,
contradictory flags say, returns `Exitf(ExitCodeUsage, ...)` to
report it with the same code the parser would have used had it been able
to see it.

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
- A misconfigured tree, a parse error and a handler's
  `Exitf(ExitCodeUsage, ...)` all share code 2 but not a prefix:
  they print `Program error:`, `Argument error:` and `Error:`
  respectively. Resolved in `docs/adr/human-readable-errors.md` as a
  feature rather than a defect — the prefix marks who caused the problem,
  so a reader learns from it whether to retype the command or report it to
  the program's author, which the shared exit code cannot tell them.
