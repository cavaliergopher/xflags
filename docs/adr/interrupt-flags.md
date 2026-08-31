# Interrupt flags

Status: accepted, 2026-08-30.

## Context

`--help` was not a flag. The lexer matched `-h` and `--help` as string
literals before consulting the option table, emitted an instruction kind of
their own, and `argv.ValidateNames` reserved both names so nothing could
declare them. `ir.Invocation` carried a `HelpRequested` bool no other flag
had, completion appended `--help` to its candidate list by hand, and the
help formatter never mentioned it -- the one flag every command accepted was
the one flag help did not list.

The cost was not the duplication. It was that the flag was invisible to
anything reading the model: a machine-readable description of a program's
command line surface omitted `--help` entirely, a second argv dialect would
have had to reimplement the mechanism rather than only the spelling, and a
program could neither rename `-h` for something of its own nor drop it.

What actually made `--help` special was never its name. It was that naming
it ends the parse and discards the errors it outran, so that
`app --bogus --help` prints help rather than reporting the typo. Nothing
about that is peculiar to help: `--version` wants it too, and so does any
flag that reports on the program rather than operating it.

## Decision

A flag with an `ir.Flag.Handler` is an **interrupt**: naming it on the
command line ends the parse there, discards every lexing error recorded so
far, and runs the handler in place of the handler of whichever command was
active. `ir.Invocation.Interrupt` names the flag that ended the parse, and
replaces `HelpRequested`.

`--help` is one interrupt among others, and nothing about it is privileged.
It is lexed through the option table, validated by the ordinary collision
check, offered by completion because it is in the table, and listed in help
like any other flag. Nothing mounts it: the program does, which is the only
dependency left between help and the command tree.

That dependency is a builder method rather than a default, so the tree is
what a program asked for and nothing else:

    NewCommand("orbital", "").
        HelpFlag().              // --help, -h
        VersionFlag(version).    // --version
        VersionCommand(version)  // orbital version

Declaring them first is a convention and nothing enforces it. It heads the
list of options, which is what argparse does with the same structure -- its
`options:` group is where ungrouped arguments go, exactly like the implicit
group here, and `-h, --help` is its first entry. The alternative worth
knowing is clap's, which puts them last in the last group: `uv pip install`
ends its eighth heading, `Global options`, with `-h, --help`. That works
because the group has real content, which a heading holding only these two
would not. Either shape is a program's to build, since the constructors are
exported and a flag goes wherever a program puts it.

Adding the help flag to the root is enough for the whole tree, because an
ancestor's options stay matchable through a descent: the root's reaches
every command under it and binds to whichever one was named. A command
below may add its own, and collides with the root's if one is there.

An interrupt binds no value. It has no `Value`, no default to restore, and
no negated spelling, and an attached value -- `--help=false` -- is a
malformed token rather than something to set.

An interrupt runs no middleware and depends on no flag being correct. It
answers and takes no other action: a program whose middleware redirects
output to a file writes no file for `--help`, and someone who wants the
help message in a file redirects the shell. This is a guarantee, not a
consequence of middleware happening to wrap handlers -- an interrupt that
ran its ancestors' wrappers could be refused by an authorization check, or
could act on a command line it never finished reading.

The version builders take the string the program supplies, since the
library has none to report, and print it beside the root command's name.

## Consequences

- A program that adds no help flag has no `--help`, where before the parser
  answered one unconditionally. That is the cost of the tree being what was
  asked for, and `Command.HelpFlag` is one call.
- The help flag is listed in help output, and its command's usage line
  gains `[OPTIONS]`, because both are now true. Whether it is *shown* is
  the author's, through `Flags(HelpFlag(...).Hidden())`; hiding also drops
  it from completion, as it does for any hidden flag.
- A command may not declare `-h` or `--help` alongside the help flag: the
  ordinary collision check reports it, naming both ends. Without the help
  flag the names are simply free.
- `VersionCommand` is an ordinary command, so an ancestor's `Required()`
  flag still applies to it where `VersionFlag`, being an interrupt, answers
  without one. The orbital example shows both and says so.
- A description marshaled from the compiled tree carries the interrupt's
  options, its usage and the fact that it takes no value, but not that it
  ends the parse: `Handler` is behavior, tagged `json:"-"`. If a consumer
  ever needs that fact, it wants a separate marshaled field rather than an
  exported func.
