package ir

import (
	"testing"

	"github.com/cavaliergopher/xflags/desc"
)

// TestFlagDescribeOptions asserts the option order desc.Flag.Options
// documents: NamedOptions in order, skipping an empty slot, then every
// generated option ClaimedOptions holds, in the order of the named
// options each was generated from -- and that a positional argument,
// which claims no option, describes none.
func TestFlagDescribeOptions(t *testing.T) {
	for _, tt := range []struct {
		name string
		flag *Flag
		want []desc.Option
	}{
		{
			name: "NamedThenGenerated",
			flag: &Flag{
				NamedOptions: []string{"--verbose", "-v"},
				ClaimedOptions: map[string]Claim{
					"--verbose":    {Source: "--verbose"},
					"-v":           {Source: "-v"},
					"--no-verbose": {Source: "--verbose", Effect: "negate"},
				},
			},
			want: []desc.Option{
				{Option: "--verbose"},
				{Option: "-v"},
				{Option: "--no-verbose", Effect: "negate"},
			},
		},
		{
			// Derivatives order by their sources' rank, not the
			// alphabet: --no-zap precedes --no-add because --zap
			// precedes --add.
			name: "GeneratedFollowSourceOrder",
			flag: &Flag{
				NamedOptions: []string{"--zap", "--add"},
				ClaimedOptions: map[string]Claim{
					"--zap":    {Source: "--zap"},
					"--add":    {Source: "--add"},
					"--no-zap": {Source: "--zap", Effect: "negate"},
					"--no-add": {Source: "--add", Effect: "negate"},
				},
			},
			want: []desc.Option{
				{Option: "--zap"},
				{Option: "--add"},
				{Option: "--no-zap", Effect: "negate"},
				{Option: "--no-add", Effect: "negate"},
			},
		},
		{
			name: "EmptySlotSkipped",
			flag: &Flag{
				NamedOptions: []string{"--colour", ""},
				ClaimedOptions: map[string]Claim{
					"--colour": {Source: "--colour"},
				},
			},
			want: []desc.Option{{Option: "--colour"}},
		},
		{
			name: "Positional",
			flag: &Flag{Positional: true, ValueName: "ARG"},
			want: nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.flag.Describe().Options
			if len(got) != len(tt.want) {
				t.Fatalf("Options = %+v, want %+v", got, tt.want)
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("Options[%d] = %+v, want %+v", i, got[i], want)
				}
			}
		})
	}
}

// TestFlagDescribePositional asserts that a positional argument's name
// and value name still appear, though it names no option.
func TestFlagDescribePositional(t *testing.T) {
	flag := &Flag{
		Name:       "arg",
		ValueName:  "ARG",
		Positional: true,
		MaxCount:   1,
	}
	got := flag.Describe()
	if got, want := got.Name, "arg"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
	if got, want := got.ValueName, "ARG"; got != want {
		t.Errorf("ValueName = %q, want %q", got, want)
	}
	if got, want := got.Positional, true; got != want {
		t.Errorf("Positional = %v, want %v", got, want)
	}
}
