package ir

// Value is the interface to the dynamic value stored in a flag.
// (The default value is represented as a string.)
//
// Set is called once, in command line order, for each flag present. In
// POSIX terms, s is the option-argument written after an option, or the
// operand bound to a positional flag.
//
// This is Go's flag.Value without String, so a value already written for
// the flag package satisfies it and may be bound with xflags.Var
// unchanged. String is not required because a compiled flag carries its
// default already rendered, captured when the flag was constructed. The
// likeness stops at the interface: command lines are POSIX/GNU and not
// the flag package's dialect, so a program migrating keeps its values and
// changes the arguments its users type.
type Value interface {
	Set(s string) error
}

// BoolValue is an optional interface to indicate boolean flags that can be
// supplied without a "=value" argument.
//
// It names the flag package's IsBoolFlag convention. A value reporting
// true is an option that takes no option-argument, so naming it alone
// stands for true and a following argument is left as an operand. It
// still accepts a value attached with "=", which is how "--verbose=false"
// sets false; that much follows Go rather than getopt_long.
type BoolValue interface {
	Value
	IsBoolFlag() bool
}

// Resetter is an optional interface for a Value whose Set method cannot
// restore its default: one that accumulates each value it is given, or one
// that shares state with other values. Parse restores every flag to its
// default before reading any arguments, so that parsing the same tree
// twice yields the same values; a value implementing Resetter is restored
// by Reset, and any other by re-applying the default its flag's
// constructor captured.
//
// The flag package has no counterpart, having no notion of parsing a set
// of flags more than once.
type Resetter interface {
	Reset()
}

// ValidateFunc is a function that validates an argument before it is
// parsed. arg is the option-argument or operand about to be handed to
// Value.Set.
type ValidateFunc = func(arg string) error
