package ir

import (
	"strings"
	"testing"
)

// TestValidateNamedOptionsAreMatchable asserts the invariant tying the
// compiled flag's two option lists together: everything NamedOptions
// shows has to appear in ClaimedOptions, or the flag advertises an option
// it will not answer to. Today's dialect upholds it by construction, so
// what this guards is a second one getting the pairing wrong.
func TestValidateNamedOptionsAreMatchable(t *testing.T) {
	for _, tt := range []struct {
		name string
		flag *Flag
		want string
	}{
		{
			name: "Matchable",
			flag: &Flag{
				NamedOptions:   []string{"--verbose", "-v"},
				ClaimedOptions: map[string]string{"--verbose": "--verbose", "-v": "-v", "--no-verbose": "--verbose"},
				Value:          stubValue{},
				MaxCount:       1,
			},
		},
		{
			name: "ShownButUnmatchable",
			flag: &Flag{
				NamedOptions:   []string{"--verbose", "-v"},
				ClaimedOptions: map[string]string{"--verbose": "--verbose"},
				Value:          stubValue{},
				MaxCount:       1,
			},
			want: "option is not matchable: -v",
		},
		{
			// An empty slot is how a flag declares an alias but no short
			// name, so it names no option and claims nothing.
			name: "EmptySlotIsNotAnOption",
			flag: &Flag{
				NamedOptions:   []string{"--verbose", "", "--loud"},
				ClaimedOptions: map[string]string{"--verbose": "--verbose", "--loud": "--loud"},
				Value:          stubValue{},
				MaxCount:       1,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFlag(tt.flag)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("validateFlag() = %v, want no error", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateFlag() = nil, want %q", tt.want)
			}
			if got := err.Error(); !strings.Contains(got, tt.want) {
				t.Errorf("validateFlag() = %q, want it to contain %q", got, tt.want)
			}
		})
	}
}

// TestValidateDuplicateValueName asserts that two positionals shown by
// the same value name are a configuration error. They are shown by that
// name alone, so "missing required argument: FILE" could not say which
// was meant -- and an explicit ValueName is enough to collide, since what
// a reader sees is the whole of the ambiguity.
func TestValidateDuplicateValueName(t *testing.T) {
	cmd := &Command{
		Name: "test",
		FlagGroups: []*FlagGroup{{Flags: []*Flag{
			{ValueName: "FILE", Positional: true, TakesValue: true, Value: stubValue{}, MaxCount: 1},
			{ValueName: "FILE", Positional: true, TakesValue: true, Value: stubValue{}, MaxCount: 1},
		}}},
	}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := err.Error(), "operand declared more than once: FILE"; !strings.Contains(got, want) {
		t.Errorf("Validate() = %q, want it to contain %q", got, want)
	}
}

// TestValidateOperandDoesNotCollideWithOption asserts what a positional
// stopped claiming: it answers to no option, so it cannot collide with
// one. "deploy SERVICE" alongside "--service" is legal, since the two
// share no spelling anywhere a reader sees them.
func TestValidateOperandDoesNotCollideWithOption(t *testing.T) {
	cmd := &Command{
		Name: "test",
		FlagGroups: []*FlagGroup{{Flags: []*Flag{
			{ValueName: "SERVICE", Positional: true, TakesValue: true, Value: stubValue{}, MaxCount: 1},
			{
				NamedOptions:   []string{"--service"},
				ClaimedOptions: map[string]string{"--service": "--service"},
				ValueName:      "SERVICE",
				TakesValue:     true,
				Value:          stubValue{},
				MaxCount:       1,
			},
		}}},
	}
	if err := cmd.Validate(); err != nil {
		t.Errorf("Validate() = %v, want no error", err)
	}
}

// stubValue satisfies Value so a fixture can be validated without binding
// a real one; nothing here ever sets it.
type stubValue struct{}

func (stubValue) String() string   { return "" }
func (stubValue) Set(string) error { return nil }
