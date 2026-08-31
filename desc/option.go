package desc

// Option is one spelling a flag answers to on the command line, and what
// naming it that way means for the value.
type Option struct {
	// Option is the option as a reader would type it: "--force" or "-f".
	Option string `json:"option"`

	// Effect names what typing this option does to the flag's value,
	// such as "negate" for the boolean opposite a getopt_long-style
	// convention writes. It is absent for the ordinary case: naming the
	// flag this way sets it. See the package doc for how to treat an
	// effect not recognized.
	Effect string `json:"effect,omitempty"`
}
