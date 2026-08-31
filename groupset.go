package xflags

// A GroupSet is an ordered registry of flag groups. A library registers the
// group it exports into a set -- usually CommandLine -- and a command
// mounts every group in the set with Command.GroupSets, so the library's
// flags bind to variables it owns without a global flag namespace.
//
// A set is read when a command that mounts it runs, not when the group is
// registered, so registration order never matters and a group registered
// during package initialization is always seen.
type GroupSet struct {
	groups []*FlagGroup
}

// FlagGroup appends g to the set and returns it, so a library registers a
// group in the var declaration that builds it:
//
//	var logFlags = xflags.Register(settings.FlagGroup())
//
// Nothing is validated here. A duplicate flag name is reported as a
// configuration error when a command that mounts the set runs.
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
