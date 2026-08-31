package desc

// FlagGroup describes a nominal grouping of flags, which affects how the
// flags are shown in help messages.
type FlagGroup struct {
	Name  string `json:"name,omitempty"`
	Title string `json:"title,omitempty"`

	Flags []*Flag `json:"flags,omitempty"`
}
