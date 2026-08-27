package xflags

// A GroupSet is an ordered registry of flag groups. A library registers the
// group it exports into a set -- usually CommandLine -- and a command
// mounts every group in the set with Command.GroupSets, so the library's
// flags bind to variables it owns without a global flag namespace.
//
// Registration is eager and append-only, while the set is read only when a
// command tree that mounts it is parsed or described. Deferring the read
// is what makes registration order safe: writes during package
// initialization land before any parse, in any order, so a var declaration
// never snapshots the set before a same-package init function has run.
type GroupSet struct {
	groups []*FlagGroup
}

// FlagGroup appends g to the set and returns it, so a library registers a
// group in the var declaration that builds it:
//
//	var logFlags = xflags.Register(settings.FlagGroup())
//
// FlagGroup never validates. A duplicate flag name surfaces as the usual
// configuration error once a command mounting the set is parsed or
// described, where the whole tree is in view.
func (s *GroupSet) FlagGroup(g *FlagGroup) *FlagGroup {
	s.groups = append(s.groups, g)
	return g
}

// CommandLine is the well-known GroupSet that the package-level Register
// appends to. A program mounts everything its libraries registered with
// one line:
//
//	var App = xflags.NewCommand("myapp", "").GroupSets(xflags.CommandLine)
var CommandLine = new(GroupSet)

// Register appends g to CommandLine and returns it. See GroupSet.FlagGroup.
func Register(g *FlagGroup) *FlagGroup {
	return CommandLine.FlagGroup(g)
}
