// Package ir is the compiled form of an xflags command tree.
//
// xflags splits every concept into two types. The configuration type --
// xflags.Command, xflags.Flag, xflags.FlagGroup -- is what a program builds
// with chained setters: ergonomic to write, but opaque, since everything
// it holds -- a bound Value, a handler, a stream -- lives in unexported
// fields a formatter or a parser cannot reach. Compiling a configuration
// tree with (*xflags.Command).Compile lowers it to the type declared
// here, the implementation type: the same information, plus resolution --
// ancestry via Parent, a full name and resolved streams computed once
// while lowering, a flag's default rendered as a string -- with every
// field public, which is what a compiled tree needs to be validated,
// parsed, dispatched, formatted and marshaled all at once.
//
// A handful of fields still carry behavior a marshaler has no use for --
// Command's Handler, FormatFunc and three streams, Flag's Value and
// ValidateFunc -- and each is tagged json:"-" rather than kept unexported,
// so encoding/json or any other reflection-based marshaler sees exactly
// the description of the tree. TestMarshalOmitsBehavior guards the tags:
// a behavior field added later without one fails that test, not silently.
//
// Most programs never import this package. Building and running a command
// tree -- xflags.NewCommand, xflags.Run, (*xflags.Command).Dispatch --
// never touches it: xflags compiles a tree internally wherever compiling
// one is called for. Reach for ir directly only when writing something
// that operates on the compiled form itself, such as a custom Value, a
// CompleteFunc, a FormatFunc, or a tool that walks or marshals a command
// tree's description.
package ir
