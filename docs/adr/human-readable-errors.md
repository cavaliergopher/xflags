# String() is for humans, Error() is for Go, agents get JSON

Status: accepted, 2026-08-25. JSON error output not yet implemented.
Amended 2026-08-26: `Run`'s prefix names who caused the failure, which adds
`Program error:` for `ConfigError` and drops the rule that a handler's own
`*ArgumentError` prints as `Error:`.
Amended 2026-08-27: `ConfigError.String()` prefixes `Cmd.pathString()`, not
`Cmd.String()`, so a deep subcommand reads as `app remote add: ...` and not
the ambiguous bare `add: ...`. `Flag.check()` below is also renamed to
`Flag.validate()`.

## Context

`ArgumentError` and `ConfigError` carry structured context — which command,
which flag, which argument, and the underlying error, if any — because the
package always meant that context to be useful beyond a human reading
stderr. There are really three audiences for one of these errors, and they
want different things:

- A human at a terminal, running a binary someone else built with xflags,
  needs a short, unix-ish sentence.
- A Go caller — code embedding the library, catching the error from
  `Parse` or a handler — wants to know, unambiguously, that this error
  came from xflags, the moment it prints or logs it without going through
  `Run`'s own reporting.
- An agent or script driving the binary from outside, with only its
  stdout/stderr to read, wants structure: which flag, which argument, not
  a sentence to parse.

A prior pass tried to serve the first and third audience from one string,
rendering `String()` as `cmd=ping flag=--ip arg="256.0.0.1" msg="..."`. It
also hardcoded `"xflags: "` onto the front of both types' `Error()`. Both
choices were wrong for the same reason: these strings are printed by
*other people's programs*. A debug dump reads as noise to the human who
mistyped a flag, and a library announcing its own name inside a host
program's stderr is a leak of an implementation detail nobody asked to
see, in the one case — the human's — where nobody benefits from it.

Separately, `docs/adr/exit-code-contract.md` already named a related loose
end in its Consequences: a parse error prints `Argument error:` and a
handler-raised usage error prints `Error:`, "because only the parser
produces an `ArgumentError`" — true at the time, but no longer the whole
story once a handler has a real reason to construct one itself.

## Decision

Three audiences, three surfaces, not one string doing three jobs:

**Humans — `String()`, printed by `Run`.** `ArgumentError.String()` is
`Message`, followed by `": " + Err.Error()` when it wraps another error.
Call sites fold the specifics into `Message` itself, e.g.
`unrecognized argument: --nope`, or use the flag's own `String()` —
`--ip` — as the message when wrapping a validation error, so it reads
`--ip: invalid IP: 256.0.0.1`. `ConfigError.String()` prefixes
`Cmd.pathString()` when the error has a command, falls back to
`Flag.String()` when it doesn't (a flag can fail its own `validate()`
before it's attached to any command), and has no prefix at all if it has
neither. `Run` prints
`String()` with its own humanized prefix, never `Error()`, so none of this
ever carries xflags' own name into a host program's console.

The prefix names who caused the failure, because that is what tells a
reader whether it is theirs to fix:

    Program error:   ConfigError -- the command tree is malformed, so
                     whoever composed the binary has a bug to fix
    Argument error:  ArgumentError -- the command line is wrong, so
                     whoever ran it can retype it
    Error:           anything else -- the handler ran and failed

Someone who cannot act on the message itself still learns from the prefix
whether to retype the command, report it to the program's author, or read
it as an ordinary failure. It carries more than the exit code does:
`ConfigError` and `ArgumentError` both exit 2, so the prefix is the only
thing telling a malformed program apart from a malformed command line.

**Go callers — `Error()`.** `Error()` is `"xflags: " + String()`, on both
types. It exists for a Go caller that catches the error value directly —
`fmt.Println(err)`, `%w`/`%v` into its own logs — without going through
`Run`'s reporting, where knowing at a glance that xflags produced it is
useful rather than noise. Because `Run` always prefers `String()` for its
own output, this prefix reaches a real program's console only when that
program chose to print the raw error itself instead of using `Run`.
`fallbackToStderr` is unrelated to this: it hardcodes its own `"xflags: "`
because it reports a plain I/O failure — a write that failed — not one of
these error types, and it needed that literal branding before either type
existed.

**Agents and scripts — JSON, behind a flag.** Nothing serves this
audience today. The direction: a flag (name TBD) switches `Run` into a
mode where an error is reported as JSON instead of `String()`'s sentence —
carrying `Cmd`, `Flag`, `Arg`, `Message` and the wrapped error as data, not
prose. Exit codes are unchanged; only the message format on the same
stream changes. `Cmd` and `Flag` detail should project through the
existing `desc.Command`/`desc.Flag` types `Describe()` already produces,
rather than a second schema for describing a command or flag. Left open,
tracked as `wip/TODO.md` item 28: the flag's exact name and scope — which
overlaps item 18's still-undecided `--xflags-describe`-style flag and is
worth designing once, not twice — and whether the JSON goes to stdout or
stderr.

The prefix follows the error's type, not who constructed it. An earlier
pass qualified this — `Argument error:` only when the `*ArgumentError`
carried a `Cmd`, so that a handler raising one still printed `Error:` —
which the code never implemented and which this amendment drops. A bad
argument is a bad argument whoever noticed it, and the reader the prefix
serves is the person who typed it, who does not care which layer caught
them out.

There is still no `UsageErrorf`. A handler that discovers a usage problem
the parser couldn't see — two mutually exclusive flags, say — returns
`Exitf(ExitCodeUsage, ...)`, which reports it with the same exit
code the parser would have used. A handler wanting the `Argument error:`
prefix as well constructs an `ArgumentError`, which is exported precisely
so it can. Neither needs a third spelling.

One decision carried over from the first pass at this ADR, unchanged:

`ConfigError.ExitCode()` is `ExitCodeUsage` (2), matching
`docs/adr/exit-code-contract.md` — "code 2 covers everything decided
before the handler runs" — and restoring what `wip/TODO.md` records as a
previous, deliberate choice that a refactor had quietly reverted to 1.

## Consequences

- A Go caller wanting structured detail — which flag, which argument —
  reads `Cmd`/`Flag`/`Arg` off the error with `errors.As`, not by parsing
  either string. Neither `String()`'s nor `Error()`'s wording is a
  compatibility promise and either may change between versions; the
  fields are the stable surface.
- A Go caller that just wants to identify the error's source when logging
  it directly now can, via `Error()`, without reaching into `errors.As`.
- An agent or script that only sees a CLI's stdout/stderr, rather than
  calling into the library, still has nothing structured to read. The
  direction is decided — a flag, JSON, built on `desc` — but not built;
  `wip/TODO.md` item 28 tracks what's left open.
- Three prefixes now map onto three causes, so the wording of a message is
  no longer the only thing telling a misconfigured program apart from a
  mistyped command line. Exit codes cannot: both are 2.
- The prefixes are wording, not API, and the same caveat as `String()`
  applies — a program must not branch on them. A caller needing to tell
  the cases apart uses `errors.As`, and an agent reading only stderr still
  has nothing structured until the JSON mode is built.
- `Program error:` is worded for the person reading the terminal, who is
  usually not the person who can fix it. It tells them the fault is in the
  program rather than in what they typed, which is the most useful thing
  they can learn from it.
- `ConfigError`'s audience is whoever composed the binary, not whoever ran
  it, but it's still written to the command's configured stderr rather than
  unconditionally to `os.Stderr` like `fallbackToStderr`. The prefix makes
  it legible, not correctly addressed. That's a separate, still-open
  question, recorded in `wip/TODO.md`.
