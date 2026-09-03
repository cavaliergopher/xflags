package climux

// A Registry is an ordered registry of contributions to a command tree:
// flag groups, middleware, and subcommands. A library registers what it
// exports into a registry, and a command mounts everything registered
// with Command.Mount, so the library's flags bind to variables it owns
// without a global flag namespace, and the program mounting it names none
// of them.
//
// Register into DefaultRegistry to contribute to one program that the
// registering package is part of, and export a Registry of your own to
// offer contributions to programs you do not own.
//
// A registry is read when a command that mounts it runs, not when a
// contribution is registered, so registration order never matters and
// anything registered during package initialization is always seen.
//
// A registry is not a node in the command tree. It holds contributions
// and claims nothing: a subcommand registered here keeps whatever parent
// it has, and the command that mounts the registry is its parent in the
// compiled tree alone. Two programs, or two tests, may therefore mount
// the same registry and neither writes to it.
type Registry struct {
	groups      []*FlagGroup
	middleware  []Middleware
	subcommands []*Command
}

// FlagGroups appends groups to the registry, which every command that
// mounts it presents after its own.
//
// Registration fits in a var declaration, so a library needs no init
// function to contribute flags:
//
//	var logFlags = &logSettings{}
//
//	var _ = climux.DefaultRegistry.FlagGroups(logFlags.FlagGroup())
//
// A group binds flags to variables the library owns, so what it registers
// here is the group and not the settings behind it; the library keeps its
// own handle on those. Either way a blank import is enough to contribute,
// since nothing the program writes names the group.
//
// Nothing is validated here. A duplicate flag name is reported as a
// configuration error when a command that mounts the registry runs.
func (r *Registry) FlagGroups(groups ...*FlagGroup) *Registry {
	r.groups = append(r.groups, groups...)
	return r
}

// Middleware appends wrappers to the registry. Every command that mounts
// it, and every command beneath one, runs inside them.
//
// Register middleware alongside the flags it reads, so that a flag and
// the code giving it meaning arrive together and neither can be mounted
// without the other:
//
//	func init() {
//	    climux.DefaultRegistry.
//	        FlagGroups(timeouts.FlagGroup()).
//	        Middleware(timeouts.Wrap)
//	}
//
// A registered wrapper runs outside the mounting command's own, so it
// wraps everything that command and its subcommands declare. See
// Command.Middleware for what a wrapper may do.
func (r *Registry) Middleware(mw ...Middleware) *Registry {
	r.middleware = append(r.middleware, mw...)
	return r
}

// Subcommands appends commands to the registry. Every command that mounts
// it takes them as children of its own, after those it declared.
//
// Use it for a subcommand a library contributes to whichever programs
// mount the registry, rather than one each program is expected to mount
// by hand. Unlike Command.Subcommands it claims no parent, since a
// registry is not a node: the mounting command is the parent, and a
// command registered here must not already be a subcommand of anything.
// Mounting a registry that holds one is a configuration error.
func (r *Registry) Subcommands(cmds ...*Command) *Registry {
	r.subcommands = append(r.subcommands, cmds...)
	return r
}

// DefaultRegistry is the well-known Registry, for the packages that make
// up one program: a binary split across internal packages, each
// contributing its own part of the command line, mounts everything they
// registered with one line.
//
//	var App = climux.NewCommand("myapp", "").Mount(climux.DefaultRegistry)
//
// A library published for programs it does not own should export a
// Registry of its own instead, and let each program decide whether to
// mount it. Registering into this one means a program acquires flags,
// wrappers and subcommands by linking a package in, which is not a
// decision a dependency should make on a program's behalf.
var DefaultRegistry = new(Registry)
