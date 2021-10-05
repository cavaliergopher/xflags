package xflags

import (
	"testing"
)

func TestNormalize(t *testing.T) {
	args := []string{
		"", "-", "--",
		"-x", "-xVar", "-x=Var", "-x=",
		"--x", "--xVar", "--x=Var", "--x=",
		"--foo", "--foo=bar", "--foo=",
	}
	expect := []string{
		"", "-", "--", "-x",
		"-x", "Var", "-x", "Var", "-x", "",
		"--x", "--xVar", "--x", "Var", "--x", "",
		"--foo", "--foo", "bar", "--foo", "",
	}
	actual := normalize(args)
	if len(actual) != len(expect) {
		t.Errorf("expected %q, got %q", expect, actual)
		return
	}
	for i := 0; i < len(expect); i++ {
		if expect[i] != actual[i] {
			t.Errorf("expected %q, got %q", expect, actual)
			return
		}
	}
}
