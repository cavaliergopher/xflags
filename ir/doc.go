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
// field public, which is what lets a formatter, a marshaler and the
// machine that reads a command line all walk one tree.
//
// This package is what a program means, not what its command line says.
// Reading a command line against a compiled tree -- lexing, applying and
// completion -- belongs to the internal argv package, which imports this
// one and is the only place that knows how a flag is written down -- that
// a name is spelled with dashes, and that the value it takes is shown as
// SERVICE. A compiled flag carries what argv wrote for it, in Forms and
// ValueName, and everything here prints what it was given.
//
// A handful of fields still carry behavior a marshaler has no use for --
// Command's Handler, UsageFunc and three streams, Flag's Value,
// ValidateFunc and CompleteFunc -- and each is tagged json:"-" rather
// than kept unexported,
// so encoding/json or any other reflection-based marshaler sees exactly
// the description of the tree. TestMarshalOmitsBehavior guards the tags:
// a behavior field added later without one fails that test, not silently.
//
// Most programs never import this package. Building and running a command
// tree -- xflags.NewCommand, xflags.Run, (*xflags.Command).Dispatch --
// never touches it: xflags compiles a tree internally wherever compiling
// one is called for. Reach for ir directly only when writing something
// that operates on the compiled form itself, such as a custom Value, a
// CompleteFunc, a UsageFunc, or a tool that walks or marshals a command
// tree's description.
package ir
