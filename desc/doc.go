// Package desc is the wire format a compiled xflags command tree lowers
// to: plain data describing a program's command line surface, meant to be
// marshaled, published and read by something other than the program
// itself -- a docs generator, a completion script, a diff run in CI, an
// agent reading a binary's surface in one call instead of crawling
// --help.
//
// A Document is the envelope. It carries a schema version and the
// described Command tree beneath it -- every Command's own FlagGroups
// and Flags, and every Subcommand, all the way down. Nothing in this
// package is behavior: a Flag names the kind of value it takes and the
// options that reach it, never a bound variable or a validator, and an
// environment variable appears as a name, never a resolved value.
//
// # Version policy
//
// Document.SchemaVersion identifies the shape of the format, not of any
// one document. Within a version, the format only ever gains keys: a
// field already published keeps its name and meaning, and a consumer
// must ignore any key it does not recognize rather than treat it as an
// error. A change that removes or repurposes a key is a new version.
//
// # Unknown effects
//
// Option.Effect names what typing that option means for the flag's
// value, and the vocabulary is open: a dialect may write an effect this
// package's current version does not enumerate. A consumer that meets an
// effect it does not recognize must not offer that option, having no way
// to know what typing it does.
package desc
