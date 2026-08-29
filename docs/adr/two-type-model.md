# Two types per concept, and the ir package

Status: accepted, 2026-08-28.

## Context

Every concept in this package needs two shapes. A command as the program
author declares it is a builder: chained setters, defaults filled in,
half-built while the tree is assembled. A command as the parser sees it is
resolved: ancestry known, mounted flag groups flattened in, every name
settled. Serving both from one type is what the abandoned
`deprecated-pod-data-model` attempt tried and could not do.

The first answer to this was the `desc` package: a projection of the
configuration tree that dropped all behavior -- no `Value`, no handler, no
writers -- on the reasoning that an artifact is either inert and portable
or live and local, and that machine-readable output (see the
machine-readable description item) needs the inert one.

Splitting parsing into a lexing pass and an applier tested that reasoning
and it failed. Because `desc` carried no `Value`, the lexer could not
resolve an argument to anything the applier could act on, so the split
grew a private parallel tree -- `boundFlag`, `boundCommand`, `bind()` --
pairing each description with the source it was projected from. Roughly
eighty lines and a second tree walk, spent entirely on keeping one field
off a struct.

The premise was wrong. Marshaling does not need a behavior-free type: Go
does not marshal unexported fields, so a settable value and a handler held
in unexported fields are already absent from the JSON. Field visibility
solves what a whole package was carved out to solve.

Keeping the behavior fields unexported, though, still meant no one outside
the package could set them, which cost a `FlagConfig`/`NewFlag` and a
`CommandConfig`/`NewCommand` to build a node, plus accessor methods --
`Path`, `Stdin`, `Stdout`, `Stderr` -- to read back what visibility hid.
For a package with no users yet and no stability promise, that is a
permanent API surface bought for a marginal guarantee: the same guarantee
is available for the price of a struct tag and a test.

## Decision

Every concept has a **configuration** type and an **implementation**
type. The configuration types stay in the root package. The
implementation types are public, and live in a new package, `ir`.

`ir` is the compiler's word for it, and the analogy is exact: the
configuration tree is the syntax the author writes, `Command.Compile`
is the lowering pass, and what comes out is an intermediate
representation that later passes -- validation, lexing, applying,
formatting, completion, marshaling -- all consume. The types keep their
plain names and take the qualifier for disambiguation, as
`ast.File` does against `types.Package`: `ir.Command` beside `Command`.

Every field on an `ir` type is exported. The handful that carry behavior
-- `Command.Handler`, `.FormatFunc` and its three streams, `Flag.Value`
and `.ValidateFunc` -- are tagged `json:"-"` instead of staying
unexported, so `encoding/json` skips them the same way it would skip an
unexported field. The guarantee moves from the type system to
`TestMarshalOmitsBehavior`, which compiles a tree exercising every
behavior field and asserts none of them survive a round trip through
`encoding/json`: a behavior field added later without its tag fails
there, not silently in a program's machine-readable output.

`ir` is also the fully resolved form of a tree, not just the public one.
Compiling walks ancestry into `Parent`, renders a flag's default into a
string, and flattens a command's own flag groups with every mounted
group into one list -- and now, while lowering, computes each command's
full name and resolves its input and output streams too, so nothing about
a compiled node has to walk itself again afterward.

The bound tree is deleted. There is one compiled tree, and the lexer
reads it directly.

The package boundary follows one rule: **the root package is everything
you need to create a standard program.** A program that declares
commands and flags, writes handlers and calls `Run` never imports `ir`.
`Invocation` and `HandlerFunc` are therefore aliased into the root
package, since they appear in the signature of every handler. Everything
else an advanced consumer reaches for -- `ir.Command`, `ir.Value` for a
custom flag type, `ir.CompleteFunc`, `ir.FormatFunc`, the error types --
is reached through the import, which keeps the root package's
documentation to the surface a standard program uses.

## Consequences

`desc` is retired. `Command.Describe` becomes `Command.Compile`, which
lowers, validates and returns the compiled node.

Validation runs on the compiled tree rather than the configuration tree.
That is the compiler order -- lower, then check -- and it is what lets a
`ConfigError` carry the compiled node it concerns, which is the
"errors carry descriptions rather than sources" change arriving as a
consequence rather than as work.

The lexer loses a guarantee. It could not previously call `Set`, because
the type it read had no value to set; now it can, and must not. A
structural property becomes a documented one, which is a real if small
loss: completion evaluates a broken command line and must apply nothing.

Help rendering moves into `ir`. The earlier reasoning that kept it in the
root package -- that `desc` earns its place by being data only, so
admitting one consumer invites all of them -- lapses with the premise it
rested on.

Two imports appear where one was enough, for the advanced consumer only.
That is the cost deliberately accepted for keeping the root package's
documentation legible.

The extra type per concept is the standing cost. It is the same cost the
`desc` package already charged; what changes is that the second type now
earns it, by being the thing the parser actually runs on.
