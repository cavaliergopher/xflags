# Handlers receive the invocation

Status: accepted, 2026-08-23.

## Context

xflags exists so that many teams can compose one binary. A team ships a
`*Command`; somebody else decides where in the tree it hangs, and may rename
or move it later. That is the premise the whole package is built on.

The v0 handler signature was `func(args []string) int`, and the first draft
of the execution model kept its shape as
`func(ctx context.Context, args []string) error`. Two problems with it:

- `args` holds only what follows a `--` terminator, so it is empty for every
  command that does not set `ForwardArgs`. The signature spends its one
  payload parameter on a minority feature.
- A handler cannot learn where it is mounted. Its own path is decided in
  another package, after registration, by someone who may not be its author.
  Closing over the path when the command is constructed only works when the
  same person writes the command and mounts it. `mitchellh/cli` forces
  exactly that workaround, and Terraform pays it.

Meanwhile `Parse` had just been changed to return an
`*Invocation{Cmd, Path, Forwarded}`, which `Run` was deconstructing in order to
pass `Args` alone. The information was being assembled and then discarded.

The alternative considered was to leave the signature at `(ctx, args)` and
attach the invocation to the context, with a `FromContext` accessor. Its
real merit is that it survives intermediaries: a house wrapper that adapts
handlers to some other signature passes the context along without having to
know what is inside it.

## Decision

    type HandlerFunc func(ctx context.Context, inv *Invocation) error

The invocation is passed as a parameter, not carried in the context.

- Context values are for request-scoped data crossing API boundaries, not
  for the parameters of a call this package makes itself.
- The signature is the documentation. An author, or an agent authoring a
  CLI, sees `inv.` and finds the whole surface; `FromContext` has to be
  discovered in prose first.
- `FromContext` would have to answer for a context that never passed through
  `Run` and return nil or a zero value that lies. Tests, where handlers are
  called with a bare context, are where that bites.
- Reaching the forwarded arguments both from the parameter and from the
  invocation would be
  one representation twice, which the data model exists to avoid.
- The signature is breaking in this release regardless, so the parameter
  costs nothing now and would cost compatibility after v1.

Prior art agrees on handing the handler something that knows its position:
cobra's `RunE(cmd *cobra.Command, args []string) error` with
`cmd.CommandPath()`, urfave/cli v3's `func(context.Context, *cli.Command)
error`, and kong injecting a `*kong.Context` whose `Command()` is the
resolved path. None of them put it in the context; cobra does the inverse
and hangs the context off the command. Where we differ is that they hand
over the configuration object, which then needs getters, and we hand over a
parse result, which keeps the source type sealed.

## Consequences

- Every handler in every program changes. The package is pre-v1 with no
  meaningful adoption, so no migration is offered.
- Naming the command that is running becomes reachable from the one place
  that wants it, through `Invocation.Cmd`: `Cmd.FullName` reads as
  "app remote add" and `Cmd.Ancestry` is the commands themselves. An
  `Invocation.Path` holding the same names was tried and cut, since it
  answered a question `Cmd` already answers.
- Middleware (`func(HandlerFunc) HandlerFunc`) inherits the invocation for
  free, which is where a telemetry label naming the command belongs.
- Handlers that ignore `inv` carry one unused parameter, as they carried an
  unused `args` before.
- `Invocation` is now load-bearing API, so its fields are part of the v1
  surface. Adding fields stays compatible; renaming them will not be.
- Attaching the invocation to the context remains available later as a pure
  addition, if a wrapper that cannot be changed ever needs it.
- `Path[0]` is whatever `NewCommand` was given, commonly `os.Args[0]`, so a
  message built from it may show a full binary path. That is the composing
  author's call, not the package's.

## Note: the streams travel with the invocation

Added 2026-08-23, with the same status as the decision above.

`Command.Output(stdout, stderr)` was read only by help and error messages,
so a program that pointed a command at a buffer captured the
`Argument error:` line and nothing the handler printed, which is most of
what a CLI emits. There was no stdin in the API at all. `Invocation`
therefore carries `Stdin`, `Stdout` and `Stderr`, resolved from the invoked
command and its ancestors, and `Output` gives way to `Command.Stdin`,
`Command.Stdout` and `Command.Stderr`, one setter per stream.

This follows from the decision above rather than qualifying it. The premise
was that a command is mounted by someone other than its author; where its
output goes is another thing that author cannot know, decided in the same
place and at the same time as the path. The invocation is already where a
handler looks for such answers, so there is nothing new to discover, and a
handler that writes to `os.Stdout` cannot be tested by the very party who
mounted it.

Consequences, beyond those above:

- `Invocation` is no longer only "what the command line said". It is what
  the handler needs in order to run, which is the wider promise and the one
  worth keeping: anything else the package resolves on a handler's behalf
  belongs here too.
- Three more field names join the v1 surface, and handlers should use them.
  Nothing enforces it, as nothing can: `fmt.Println` in a handler is a
  defect the package can document but not detect. This mirrors `flag`,
  which offers `SetOutput` and cannot stop anyone printing past it.
- Each stream is set and resolved on its own, so redirecting stdout leaves
  errors on stderr. Taking both writers at once meant inheriting them as a
  set: `Output(&buf, nil)` resolved a nil stderr and panicked on the first
  error message, and saying "leave this one alone" required passing a nil.
- One word now follows each stream from `Command.Stdout` through the field
  and its resolver to `Invocation.Stdout`. That is worth more than matching
  `flag.FlagSet.SetOutput`, which has a single output stream and so never
  had to name a second one.
- `Compile` resolves `Stdin`, `Stdout` and `Stderr` while lowering the
  tree, falling back to `os.Stdin`, `os.Stdout` and `os.Stderr` for
  whichever the command and its ancestors left unset. `Parse` compiles a
  fresh tree on every call, so the streams it hands the invocation are
  still the process streams as they stood when `Parse` was called; only a
  caller that holds a compiled tree across a later reassignment of
  `os.Stdout` would see the difference, since that tree keeps the stream
  it compiled with.
