# The dialect is the POSIX syntax guidelines plus GNU long options

Status: proposed, 2026-08-23. Partly implemented.

## Context

`docs/adr/posix-gnu-argv-dialect.md` chose the dialect and stopped at the
sketch: `--long` for names, `-s` for single characters, a value attached or
in the next argument, `--` for commands that opt in. That settles what a
command line looks like at a glance and leaves every edge to whoever reads
parser.go — whether `-abc` is three flags or one, whether `--count -5` is a
negative value or a missing one, what a bare `-` means, whether an option
may follow an operand. Those edges are most of the dialect. Named without
them, "POSIX/GNU" is a gesture.

There is a written standard to answer them with. POSIX.1-2024 (IEEE Std
1003.1-2024, Base Specifications Issue 8) chapter 12, *Utility
Conventions*, defines options, option-arguments and operands in 12.1 and
gives fourteen *Utility Syntax Guidelines* in 12.2. The GNU Coding
Standards add long options to that base — "It is a good idea to follow the
POSIX guidelines for the command-line options of a program" — and
`getopt_long` is the reference implementation of the combination.

Adopting them by citation rather than by resemblance buys two things. The
guidelines are what users, generated documentation and agents already
expect, so conforming costs nothing and departing costs surprise. And they
are testable: with a numbered list to check the parser against, the places
we depart become an enumerated set with reasons, rather than whatever the
code happens to do.

The guidelines were written for a single utility parsed by `getopt`, and
three things here do not fit that mould: commands nest, so an operand may
be a subcommand name; flags bind to Go variables through `Value`, so
"takes an argument" is a property of the bound type; and names are scoped
to a command path, so the set of legal options changes as argv is read.
Each needs the guideline translated rather than quoted.

## Decision

POSIX's vocabulary maps onto the package as follows, and this ADR uses the
POSIX terms when it means the standard's concept:

    option           a Flag with a name or a short name
    option-argument  the string handed to Value.Set
    operand          a positional Flag, or a subcommand name
    utility          the root Command; subcommands are ours, not POSIX's

### Adopted as written

**Guideline 3 — one alphanumeric character per short name.** `ShortName`
takes a single character from the portable character set: `[A-Za-z0-9]`.
Today the check is `len(shortName) > 1`, which counts bytes and admits
punctuation, so `-é` is rejected for the wrong reason and `-!` is accepted
for none. The reserved `-W` is not adopted; it is a vendor escape hatch in
a standard that has vendors, and this package has authors.

**Guideline 4 — options are introduced by a delimiter.** `-` for short
names, and `--` for long ones as GNU has it. A single dash never introduces
a long name, which is the whole of the earlier ADR.

**Guideline 5 — grouping.** `-abc` is `-a -b -c`. Consumption continues
while each short name is a flag that takes no value; the first that takes
one takes the remainder of the argument as its attached value, so `-abfx`
is `-a -b -f x`. This is `getopt`'s rule and needs the compiled description
in hand, which is why it is not implemented yet.

**Guideline 7 — option-arguments are not optional.** A flag either always
takes a value or never does, decided by its bound `Value`: a boolean flag
takes none, everything else takes one. This is why `--verbose true` sets
`--verbose` and leaves `true` as an operand rather than guessing, and it is
the rule that makes guideline 5 decidable at all.

**Guidelines 11 and 12 — options commute, operands do not.** Two flags may
be given in either order; positional flags bind left to right in
declaration order. Repetition is ordered too: `Value.Set` is called once
per occurrence, in the order the occurrences appear. `--help` is subject to
the same rule and does not obey it today: it short-circuits where it is
found, so `app --bogus --help` reports the unknown flag while
`app --help --bogus` prints help. Help wherever it appears is the rule.

**Guideline 13 — a bare `-` is an operand.** The parser passes it through
untouched, and whether it means standard input, standard output or a file
of that name is the handler's business, exactly as the guideline says.

**Guideline 14 — if it parses as an option, it is one.** An argument
beginning with `-` is never taken as a detached option-argument, so
`--count -5` is a missing value and not negative five. The escape hatch is
the attached form, `--count=-5`, which is unambiguous by construction and
must therefore always work; today it does not, because `normalize` splits
it into two arguments and loses the fact that they arrived as one.

### Adopted with a translation

**Guideline 6 — one argument per option.** Adopted as the recommended form
and the one help output shows. The two attached forms `getopt_long`
accepts are accepted with it: `--name=value` splits at the first `=`, and
`-nvalue` takes the whole remainder of the argument as the value. The
remainder is taken literally, so `-n=value` gives `-n` the value `=value`,
which is how a value beginning with `=` is given attached. Today the parser
strips that `=`, and it should not: the convenience is worth less than
agreeing with every other tool a user has met. Under guideline 5 the
remainder after a short name that takes no value is read as further short
names, so `-a=false` is an unknown option rather than a value.

**Guideline 9 — options precede operands.** Not adopted in POSIX's strict
reading, which stops option processing at the first operand; adopted in
GNU's, where options may appear anywhere among the operands. This is the
existing behavior and the one users expect from git and friends. The
scoping rule qualifies it rather than contradicting it: a flag is legal
from the point its own command is named onward, so `app --sub-only sub` is
an error about ordering that no amount of permutation fixes. See
`docs/adr/path-scoped-flag-names.md`.

**Guideline 10 — `--` ends option processing.** Adopted for commands that
set `WithTerminator`, and everything after it reaches the handler as
`Invocation.Args` rather than binding to operand slots. POSIX has no third
category, but POSIX has no subcommands either, and the arguments a command
means to forward to something else are not the same as the operands it
consumes itself. A command that has not opted in must reject a bare `--`;
today it silently binds it as an operand, which is the worst of both
readings.

### Not adopted

**Guideline 8 — multiple option-arguments in one argument.** Not adopted
as a parser rule. Repetition is how a flag is given more than one value —
`--tag a --tag b`, governed by `NArgs` — because it composes with
validation and counting, and it is what every modern tool does. A `Value`
that wants comma-separated input may split its own argument; that is a
decision about a type, not about argv.

**Guidelines 1 and 2 — utility naming.** Not enforced. Two-to-nine
characters is a limit from an era this package does not live in, and the
root command's name is the program's own. What is enforced is only what
would otherwise break parsing: a long name may not be empty, begin with
`-`, or contain `=` or whitespace. Lowercase words joined by hyphens is
the documented convention, kept by examples and help output rather than by
validation, because breaking a build over `--dryRun` is not this package's
job.

### From GNU rather than POSIX

**Long options**, in both forms: `--name value` and `--name=value`.

**`--help`, and `-h` with it.** Reserved by the parser, handled before
flags are looked up, and exiting zero — `docs/adr/exit-code-contract.md`
says why that is success. Reserved means a command may not declare them;
that is not checked today, so a declared `-h` is silently shadowed.

**Permutation**, covered under guideline 9 above.

**Abbreviated long options are not adopted.** `getopt_long` accepts any
unique prefix, and a command tree makes "unique" a moving target: the
active table grows as the parser descends, so `--ver` may resolve at the
root and be ambiguous two commands down, and adding a flag can break a
script that never changed. Completion and generated documentation cannot
show the accepted spellings either, since there are dozens per flag.
Nothing is lost that completion does not give back.

**`--version` is the program's, not the library's.** The GNU standards ask
every program to have one; the library has no version string to report and
no place to put one that would not be a guess. A convenience for declaring
it is a reasonable thing to add later.

## Consequences

- The parser has a specification to be tested against, guideline by
  guideline, and the deviations above are the test list as much as the
  conformances are.
- Six defects are named by this ADR rather than discovered later:
  `--count=-5` fails; `-n=value` strips the `=`; `--` is consumed as an
  operand by commands that did not opt into it; `--help` is honored only
  until something ahead of it fails; a declared `-h` never fires; and
  short-name validation counts bytes and permits punctuation. None is a
  design question, and all six are cheap once the parser preserves whether
  a value arrived attached.
- Guideline 5 is now a decision and not an open question. It changes what
  `-ab` means for two boolean flags, from a stray positional argument to
  two set flags, and it must land with the schema-aware pass rather than
  in `normalize`, which cannot see whether `-a` takes a value.
- `-n=value` changes meaning, from `value` to `=value`. Every other change
  here turns something into an error or an error into something; this one
  quietly returns a different string, so it is the one to call out in
  release notes. It is worth making while the package is pre-v1: a
  deviation kept for ergonomics is one more place where what a user knows
  about `getopt` is wrong here, and conformance is the whole claim.
- Guideline 7 constrains the data model, not just the parser: an
  optional-valued flag cannot be added later without giving up grouping
  and the detached-value rule together.
- Negated booleans (`--no-verbose`), flag aliases and mutually exclusive
  sets are all within this dialect and none is settled here. They are
  declarations on the data model that change help and validation, not
  argv's shape.
- Conformance is claimable in the README, with the departures listed. That
  is worth more than the word "POSIX" on its own, which is what the
  package can honestly say today.
