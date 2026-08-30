// Package argv reads a command line against a compiled xflags command
// tree.
//
// Two compilations meet in xflags and it helps to keep them apart. The
// command tree is compiled: (*xflags.Command).Compile lowers what a
// program declared into the ir package, which is what the program means.
// The command line is then read against that result, which is what this
// package does. So ir is the artifact and argv is the machine that runs
// on it.
//
// Reading a command line is two passes. Lexing resolves argv against the
// compiled tree into a flat list of instructions -- set this flag to this
// value, descend into this subcommand -- and applies nothing, so it can
// be run against a line that is broken or only half typed. Applying is
// everything with an effect: Set, environment variables, and checking
// that each flag was given as often as it requires. Completion is the
// same machine stopped after the first pass, which is why it lives here
// too: it needs the state a partial line leaves you in, not the values it
// would have bound.
//
// This package also owns how a flag is written down for a reader, in
// both halves of it. optionOf turns a declared name into the option it is
// shown as, one character taking a single dash and anything longer
// taking two; ValueNameFor writes the name of a flag's value the way a
// synopsis does, so that "log-level" reads as LOG_LEVEL. Compile calls
// both while lowering, so ir.Flag carries the results rather than any
// consumer producing them, and everything downstream prints what it was
// given.
//
// OptionsFor is the entry point that answers both questions about a
// flag's names at once -- how they are written, and everything the flag
// answers to, which is a superset once this dialect generates a spelling
// of its own; see claims.go. It takes the flag's shape rather than a
// decision already made, so a positional argument having no options is
// settled here and not by the caller. ValidateNames is the same authority
// reading names before they are written, since the rules a name has to
// keep are this dialect's too.
//
// Casing belongs here for the same reason dashes do: another convention
// would write <log-level>, and Go's own flag package writes a value's
// type in lower case. What does not belong here is help layout -- which
// sections exist, their order, their columns -- which is the same
// whatever a flag is called, and stays in ir.
//
// Validate is this authority answering the configuration-time question:
// two flags collide when their forms are equal, which only this package
// can say.
//
// What a generated option means for the value it binds stays here too,
// and is not recorded on the compiled flag. ir maps each option a flag
// answers to back to the option it came from, which every convention can
// answer; this package recognizes which of them it wrote by running its
// own generator forward against that source, so a convention with no
// negation -- or with some other modifier -- leaves no field named for
// this one lying dead.
//
// The import graph is what keeps that honest. argv imports ir and ir
// cannot import argv, so ir has no way to reach a dialect's opinion about
// spelling even by accident. A second argv dialect -- Windows slash
// style, or stdlib flag compatibility -- is a replacement for this
// package's spelling and matching, and for nothing else; see
// docs/adr/posix-gnu-argv-dialect.md. Note that help layout is not
// spelling: which sections exist, their order and their columns are
// presentation policy and stay in ir, or a second dialect would have to
// reimplement them.
package argv
