# The dialect is the POSIX syntax guidelines plus GNU long options

Status: accepted, 2026-08-23. Amended 2026-08-26, three times: twice over
how an attached value is read, see *Attached values follow Go, not getopt*;
and once over guideline 10, where `--` now ends option processing by
default rather than a command that did not opt in rejecting it, and the
opt-in is `ForwardArgs`, which hands the arguments to
`Invocation.Forwarded`. Implemented apart from the three gaps named in the
consequences.

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

**Guideline 3 — one alphanumeric character per short name.** A name of one
character is spelled with a single dash, and validation confines it to the
portable character set: `[A-Za-z0-9]`.
The check counted bytes and admitted punctuation until 6195ed8, so `-é` was
rejected for the wrong reason and `-!` was accepted for none. Guideline 5
now leans on the rule rather than merely agreeing with it: `=` is read as a
delimiter after a boolean precisely because it can never be a name. The
reserved `-W` is not adopted; it is a vendor escape hatch in
a standard that has vendors, and this package has authors.

**Guideline 4 — options are introduced by a delimiter.** `-` for short
names, and `--` for long ones as GNU has it. A single dash never introduces
a long name, which is the whole of the earlier ADR.

**Guideline 5 — grouping.** `-abc` is `-a -b -c`. Consumption continues
while each short name is a flag that takes no value; the first that takes
one takes the remainder of the argument as its attached value, so `-abfx`
is `-a -b -f x`. This is `getopt`'s rule and needs the compiled description
in hand, which is why it could not live in a pass over argv alone. The one
place the remainder is not spent on names is an `=` following a boolean;
see *Attached values follow Go, not getopt* below.

**Guideline 7 — option-arguments are not optional.** A flag either always
takes a value or never does, decided by its bound `Value`: a boolean flag
takes none, everything else takes one. This is why `--verbose true` sets
`--verbose` and leaves `true` as an operand rather than guessing, and it is
the rule that makes guideline 5 decidable at all. It governs detached
values only: an attached `--verbose=false` sets false, for which see
*Attached values follow Go, not getopt* below.

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
must therefore always work. It did not until e53ae05: a normalization pass
split it into two arguments and lost the fact that they arrived as one.

### Adopted with a translation

**Guideline 6 — one argument per option.** Adopted as the recommended form
and the one help output shows. The two attached forms `getopt_long`
accepts are accepted with it: `--name=value` splits at the first `=`, and
`-nvalue` takes the whole remainder of the argument as the value. How the
two forms interact is where this ADR departs from `getopt_long` for reasons
of its own; see *Attached values follow Go, not getopt* below.

**Guideline 9 — options precede operands.** Not adopted in POSIX's strict
reading, which stops option processing at the first operand; adopted in
GNU's, where options may appear anywhere among the operands. This is the
existing behavior and the one users expect from git and friends. The
scoping rule qualifies it rather than contradicting it: a flag is legal
from the point its own command is named onward, so `app --sub-only sub` is
an error about ordering that no amount of permutation fixes. See
`docs/adr/path-scoped-flag-names.md`.

**Guideline 10 — `--` ends option processing.** Adopted, in the standard's
own reading, as the default: every argument after `--` is an operand
however many dashes it starts with, so a command can be given an operand
named `-rf`. This is the escape hatch guideline 14 depends on for a
detached value, the attached form being the other.

A command that sets `ForwardArgs` takes the second reading instead, and
everything after `--` reaches the handler as `Invocation.Forwarded` rather
than binding to operand slots. POSIX has no third category, but POSIX has
no subcommands either, and the arguments a command means to forward to
something else are not the same as the operands it consumes itself.

The two readings disagree about where the arguments go, so a command has
one or the other and `ForwardArgs` says which. Only the first `--` is
special either way; after it, a second is an ordinary operand, as is a
`-h` that would otherwise ask for help.

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

### Attached values follow Go, not getopt

Three rules about values attached to their option depart from `getopt_long`
deliberately. `-n=value` sets `value` and not `=value`; a boolean long
option accepts an attached value, so `--verbose=false` sets false; and an
`=` following a short boolean is a delimiter rather than another name, so
`-v=false` sets false too. They are recorded here because they are
departures and not because they are fixes.

Measured, rather than recalled:

    implementation           -n=value    --verbose=false
    C getopt_long (POSIX)    =value      error: doesn't allow an argument
    Python getopt            =value      n/a
    git parse-options        =value      n/a
    Python argparse          value       error: ignored explicit argument
    Go flag                  value       false
    pflag / cobra            value       false

The standard does not in fact rule on `-n=value`. POSIX has no long options
and never mentions `=` in any position; guideline 6 gives short options one
attached form, the remainder of the argument, and with no delimiter in that
grammar `=` can only be data. GNU then defined `=` freely for long options
because they were new and carry no remainder rule — `--namevalue` is simply
unrecognized — while short options could not be changed without altering
the meaning of every program already linked against `getopt`. The literal
reading is therefore an inherited compatibility constraint rather than a
principle, and this package has no installed base to inherit it from.

What decides it instead is who arrives here. Users of a Go flags package
come from `flag`, and every widely used modern parser above strips the `=`.
A Go developer writing `-n=value` and silently receiving `=value` has a
wrong value rather than an error, which is the worse of the two failure
modes; the value that is lost is one that begins with `=`, given attached,
for which `-n =value` and `--name==value` both remain available.

The boolean rules have the same shape and one more reason: without them
there is no way to set a boolean false from argv at all, since a detached
value is never consumed by a boolean and negated booleans are not settled.
`getopt_long` and argparse both reject the long form, so this departure is
the larger one and belongs in the README's list.

The short boolean costs something the long one does not, and is worth
stating plainly: guideline 5 spends the whole remainder of a short argument
on further names, so reading `=` as a delimiter takes a character back from
grouping. It costs no ambiguity, because guideline 3 confines a short name
to `[A-Za-z0-9]` and `=` can therefore never be one. What it buys is that a
flag's two spellings agree — `-v=false` and `--verbose=false` both set
false — where conforming would have made the short form the only spelling
that cannot express it. An asymmetry between a flag's own two names is a
worse thing to explain than a departure from a guideline.

Neither rule touches detached values. Guideline 14 still decides those, so
`--count -5` is a missing value, `--verbose false` still leaves `false` as
an operand, and both departures are confined to a single argument that
already carries its own value.

## Consequences

- The parser has a specification to be tested against, guideline by
  guideline, and the deviations above are the test list as much as the
  conformances are.
- Five defects were named by this ADR rather than discovered later, and
  three are fixed. `--count=-5` failed and `--verbose=false` was the same
  defect wearing different clothes, both cured by preserving whether a
  value arrived attached (e53ae05); short-name validation counted bytes and
  permitted punctuation (6195ed8); and `--` was consumed as an operand by
  commands that had not opted into it, which ending option processing by
  default resolves without the rejection this ADR first called for. Two
  remain: `--help` is honored only until something ahead of it fails, and a
  declared `-h` never fires. Neither is a design question. A third gap is a
  long name going unchecked for `=` and whitespace, which break parsing
  rather than merely departing from convention.
- Guideline 5 is now a decision and not an open question. It changes what
  `-ab` means for two boolean flags, from a stray positional argument to
  two set flags. It could not live in `normalize`, which could not see
  whether `-a` takes a value; removing that pass is what made it a loop
  over one argument rather than a second pass over argv.
- No change here silently returns a different value. Every one turns
  something into an error, or an error into something, which is what makes
  them safe to land pre-v1 without release notes reading like a warning.
  That is a consequence of keeping the attached-value rules rather than
  conforming them, and it was not the reason for keeping them.
- Guideline 7 constrains the data model, not just the parser: an
  optional-valued flag cannot be added later without giving up grouping
  and the detached-value rule together.
- Negated booleans (`--no-verbose`), flag aliases and mutually exclusive
  sets are all within this dialect and none is settled here. They are
  declarations on the data model that change help and validation, not
  argv's shape. Negation is wanted rather than needed now that
  `--verbose=false` works; under the conforming rule it would have been the
  only way to turn a boolean off, and so a prerequisite for v1.
- Conformance is claimable in the README, with the departures listed. That
  is worth more than the word "POSIX" on its own, which is what the
  package can honestly say today.
