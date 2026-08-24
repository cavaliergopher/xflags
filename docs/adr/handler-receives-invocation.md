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
  command that does not set `WithTerminator`. The signature spends its one
  payload parameter on a minority feature.
- A handler cannot learn where it is mounted. Its own path is decided in
  another package, after registration, by someone who may not be its author.
  Closing over the path when the command is constructed only works when the
  same person writes the command and mounts it. `mitchellh/cli` forces
  exactly that workaround, and Terraform pays it.

Meanwhile `Parse` had just been changed to return an
`*Invocation{Cmd, Path, Args}`, which `Run` was deconstructing in order to
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
- Reaching `Args` both from the parameter and from the invocation would be
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
- `Invocation.Path` earns its place: it is now reachable from the one place
  that wants it. It was very nearly cut for having no reachable consumer.
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
