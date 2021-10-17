package xflags

import (
	"testing"
)

func TestNormalize(t *testing.T) {
	args := []string{
		"-x", "-xVar", "-x=Var", "-x=",
		"--x", "--xVar", "--x=Var", "--x=",
		"--foo", "--foo=bar", "--foo=",
		"", "-", "--",
	}
	expect := []string{
		"-x", "-x", "Var", "-x", "Var", "-x", "",
		"--x", "--xVar", "--x", "Var", "--x", "",
		"--foo", "--foo", "bar", "--foo", "",
		"", "-", "--",
	}
	t.Run("NoTerminator", func(t *testing.T) {
		actual := normalize(args, false)
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
	})
	t.Run("WithTerminator", func(t *testing.T) {
		tArgs := make([]string, len(args)*2)
		copy(tArgs, args)
		copy(tArgs[len(args):], args)
		tExpect := make([]string, len(expect)+len(args))
		copy(tExpect, expect)
		copy(tExpect[len(expect):], args)
		tActual := normalize(tArgs, true)
		assertStringSlice(t, tExpect, tActual)
	})
}

func TestTerminator(t *testing.T) {
	var foo string
	var bar bool
	cmd := Command("test", "").
		Flags(
			StringVar(&foo, "foo", "", "").Must(),
			BoolVar(&bar, "bar", false, "").Must(),
		).
		WithTerminator().
		Must()
	tailArgs := []string{
		"baz",
		"--baz", "--baz=qux", "--baz", "qux",
		"-q", "-q=quux", "-q", "quux",
		"--", "-", "",
	}
	args := append([]string{"--foo=foo", "--bar", "--"}, tailArgs...)
	if _, err := cmd.Parse(args); err != nil {
		t.Fatal(err)
	}
	assertString(t, "foo", foo)
	assertBool(t, true, bar)
	assertStringSlice(t, tailArgs, cmd.Args())
}
