package desc

// FlagGroup describes a nominal grouping of flags, which affects how the
// flags are shown in help messages.
type FlagGroup struct {
	Name  string `json:"name,omitempty"`
	Title string `json:"title,omitempty"`

	// Mounted reports that the command took this group from a shared
	// set of flags contributed by another package, rather than declaring
	// it itself.
	Mounted bool `json:"mounted,omitempty"`

	Flags []*Flag `json:"flags,omitempty"`
}
