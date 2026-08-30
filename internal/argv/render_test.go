package argv

import "testing"

// TestValueNameFor pins how a flag's value is named, including the two
// answers the dialect gives rather than the caller: a flag that takes no
// value has none, and a name too short to describe anything is written as
// a generic placeholder instead of being spelled out.
func TestValueNameFor(t *testing.T) {
	for _, tt := range []struct {
		name       string
		flagName   string
		valueName  string
		positional bool
		takesValue bool
		want       string
	}{
		{"LongName", "log-level", "", false, true, "LOG_LEVEL"},
		{"Override", "log-level", "level", false, true, "LEVEL"},

		// "-c C" says less than "-c VALUE"; see ValueNameFor.
		{"ShortOptionIsGeneric", "c", "", false, true, "VALUE"},

		// An explicit request is honored however short it is: the program
		// said what it wanted the value called.
		{"ShortOverrideIsHonored", "c", "n", false, true, "N"},

		// An operand's name was chosen to describe it rather than to be
		// terse to type, so "app A B" means those letters.
		{"ShortOperandKeepsItsName", "a", "", true, true, "A"},

		{"NoValueHasNoName", "verbose", "", false, false, ""},
		{"NoValueIgnoresOverride", "verbose", "level", false, false, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := ValueNameFor(tt.flagName, tt.valueName, tt.positional, tt.takesValue)
			if got != tt.want {
				t.Errorf("ValueNameFor(%q, %q, %v, %v) = %q, want %q",
					tt.flagName, tt.valueName, tt.positional, tt.takesValue, got, tt.want)
			}
		})
	}
}
