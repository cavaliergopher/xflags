package desc

// Command describes a command that users may invoke from the command
// line: its own name, the flags it accepts and every subcommand beneath
// it.
//
// A Command names only the flags declared or mounted on it, not those
// inherited from an ancestor. A flag is in scope for a command from the
// point its own command is named onward, so an ancestor's flags are not
// repeated on each of its descendants; see the machine-readable-schema
// ADR.
type Command struct {
	// Name is the command's own name, unqualified by its ancestry: "add"
	// rather than "orbital remote add".
	Name string `json:"name"`

	// FullName is Name joined with each ancestor's, from the root down:
	// "orbital remote add".
	FullName string `json:"fullName"`

	Summary     string `json:"summary,omitempty"`
	Description string `json:"description,omitempty"`

	// Hidden reports that the command is omitted from help output.
	Hidden bool `json:"hidden,omitempty"`

	// ForwardArgs reports that arguments following a "--" terminator are
	// passed through to the command rather than rejected.
	ForwardArgs bool `json:"forwardArgs,omitempty"`

	// Forwarded names the arguments the command hands to its handler
	// unparsed, when the program named them: what follows an interrupt
	// command's name, or a ForwardArgs command's "--" terminator. Absent
	// when the program named nothing, which does not say nothing is
	// forwarded.
	Forwarded *Forwarded `json:"forwarded,omitempty"`

	FlagGroups  []*FlagGroup `json:"flagGroups,omitempty"`
	Subcommands []*Command   `json:"subcommands,omitempty"`
}

// Forwarded names and explains a command's forwarded arguments; see
// Command.Forwarded.
type Forwarded struct {
	// ValueName is how the forwarded arguments are shown where the
	// command's arguments are, already written for a reader.
	ValueName string `json:"valueName"`

	Usage string `json:"usage,omitempty"`
}
