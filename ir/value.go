package ir

// Value is the interface to the dynamic value stored in a flag.
// (The default value is represented as a string.)
//
// Set is called once, in command line order, for each flag present.
type Value interface {
	Set(s string) error
}

// BoolValue is an optional interface to indicate boolean flags that can be
// supplied without a "=value" argument.
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
type Resetter interface {
	Reset()
}

// ValidateFunc is a function that validates an argument before it is parsed.
type ValidateFunc = func(arg string) error
