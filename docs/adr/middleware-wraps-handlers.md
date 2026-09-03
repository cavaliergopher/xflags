# Middleware wraps handlers

Status: accepted, 2026-08-30.

## Context

The motivating case is CLI telemetry: a binary reporting how often each
subcommand is invoked and how long it takes. It is the clearest instance
of a wider class — work every command in a subtree has to do, such as an
authorization check, or a resource opened and closed around the handler.

Telemetry is the case a program author cannot solve alone, for three
reasons that compound:

- **Incomplete analytics are worse than none.** Coverage of most
  subcommands cannot distinguish a command nobody invokes from one nobody
  instrumented, and every decision taken on the data inherits that
  ambiguity.
- **The party that wants the data does not own the handlers.** Whoever
  assembles the binary wants it; the teams that wrote each subcommand get
  nothing from remembering to opt in, on every command they add, forever.
- **Omission is silent.** A missing measurement raises no error. It is
  discovered when someone deprecates a command on data that never covered
  it.

Without a mechanism for it, the only answer is the `Wrap` idiom shown in
`example_di_test.go` — a closure returning a `HandlerFunc`, which climux
does not provide so much as permit — applied by each command where it
declares its handler. That leaves the decision with the wrong party.
climux exists so that many teams can compose one binary, and the team
deciding that a subtree must be measured, or audited, is the one
assembling it rather than each team that wrote a handler.

The cost arrives as soon as a binary is large enough to want the
mechanism. A concern applied handler by handler makes every package
carrying it import the package defining it and thread the same state
through its own `HandleFunc` calls, and a flag such as `--trace`, which
advertises "a timing trace for every command", traces only the commands
that remembered to opt in — with nothing in the flag's own description to
tell a reader which those are.

## Decision

    type Middleware func(HandlerFunc) HandlerFunc

    func (c *Command) Middleware(mw ...Middleware) *Command

A middleware is declared on a command and inherited by every command
beneath it. `Compile` composes the whole path's middleware while lowering
and applies it there, so `ir.Command.Handler` is the command's own handler
already wrapped, and running a command is calling it.

The details that follow from that, each of which had an alternative:

- **The outermost wrapper is the one declared highest in the tree**, and
  within one command they nest in the order given. This is the order a
  reader assumes from `net/http`, and the only one in which a root
  authorization check can refuse before a subcommand's wrapper runs.
- **`ir` does not know that middleware exists.** No type, no field, no
  method. Middleware is a mechanism for *building* `Handler`, and the
  compiled tree models what a program means rather than how it was
  assembled, so a field carrying the composed chain would be scaffolding
  left in the artifact. The chain is threaded down `lower` as a parameter
  instead, which is where scaffolding belongs.
- **`Handler` is never nil.** A command that declared none gets one
  reporting the usage error, substituted while lowering. That keeps the
  "missing subcommand" wording in the root package, where dispatch policy
  lives, and leaves `ir` with nothing to branch on. The substitute is not
  wrapped: a command that only groups subcommands must not run its
  ancestors' middleware.
- **A wrapper runs only around a handler.** An interrupt, an unparsable
  command line and a command that only groups subcommands run no handler,
  so they run no middleware either. A wrapper exists to wrap something.
  For an interrupt this is a guarantee rather than a consequence; see
  `docs/adr/interrupt-flags.md`.
- **Middleware is in the core package, not a subpackage.** A subpackage
  cannot reach a handler in order to wrap it: `Command.handlerFunc` is
  unexported, and `Subcommands` takes already-built commands another team
  owns and may mount elsewhere. It would have to hand-roll dispatch on top
  of `Parse`, giving up `Run`'s error reporting, exit-code contract, help
  handling and completion hook — all unexported — or the core would have to
  open a seam for it, at which point the seam is the feature. The only
  question left is whether that seam is a parameter to `Run` or a field on
  `Command`, and a parameter cannot vary by subtree, which is the whole
  point. A subpackage would still earn its place if climux ever shipped
  middleware *implementations*, which is a separate decision.

## Consequences

- Wrapping a handler in a function of your own stays a usage idiom rather
  than becoming a library feature. A handler is a plain function type, so
  an author who wants to hand one dependencies of its own writes a closure
  and needs nothing from climux. Middleware neither replaces that nor is
  asked to: a `Middleware` is a `HandlerFunc` on both sides, so it cannot
  change what a handler is given.
- A nil middleware is a configuration error, reported by `Compile` against
  the command that declared it, and skipped when the chain is composed.
  Composition leaves nothing on the compiled node to check afterwards, so
  it is checked while lowering, where it is still a list of what one
  command declared, and lowering finishes so the rest of the tree's errors
  are reported in the same run.
- Middleware runs after the command line is applied, so a wrapper may read
  the flags the invocation set. That is what lets `orbital`'s audit check
  read the root `--actor` flag, which is not set until parsing.
- Anything holding a compiled tree runs a command by calling `Handler`,
  and gets the middleware whether or not it knows there is any. There is
  no second way to run a command to get wrong.
- `ir.Command` gains no field, so there is nothing new for
  `TestMarshalOmitsBehavior` to guard. A machine-readable description of a
  command says nothing about what wraps it, which is right: middleware
  changes what happens, not what the command line accepts.
- A `Middleware` must be a pure function of the handler it is given,
  doing its work in the handler it returns. Applying while lowering means
  wrappers are applied once per `Compile`, and a run compiles more than
  once — `Dispatch`, then `handleErr` on the error path. A wrapper that
  registered a metric before returning would do so repeatedly. This is a
  documented contract rather than a guard, and a compile-once `Run`
  shrinks the exposure.
- Additive. No existing program changes, and `orbital` converted to it by
  deleting a `Chain` helper and moving three registrations onto the
  commands themselves.
