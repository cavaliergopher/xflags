package xflags

import (
	"testing"
	"time"
)

func parseFlag(t *testing.T, flag *FlagInfo, args ...string) {
	_, err := Command("test").Flags(flag).MustBuild().Parse(args)
	if err != nil {
		t.Fatal(err)
	}
}

func assertBool(t *testing.T, expect, actual bool) bool {
	if expect == actual {
		return true
	}
	t.Errorf("expected bool: %v, got: %v", expect, actual)
	return false
}

func assertDuration(t *testing.T, expect, actual time.Duration) bool {
	if expect == actual {
		return true
	}
	t.Errorf("expected duration: %v, got: %v", expect, actual)
	return false
}

func assertFloat64(t *testing.T, expect, actual float64) bool {
	if expect == actual {
		return true
	}
	t.Errorf("expected float64: %v, got: %v", expect, actual)
	return false
}

func assertInt64(t *testing.T, expect, actual int64) bool {
	if expect == actual {
		return true
	}
	t.Errorf("expected int64: %v, got: %v", expect, actual)
	return false
}

func assertString(t *testing.T, expect, actual string) bool {
	if expect == actual {
		return true
	}
	t.Errorf("expected string: \"%s\", got: \"%s\"", expect, actual)
	return false
}

func assertStringSlice(t *testing.T, expect, actual []string) bool {
	if len(expect) != len(actual) {
		t.Errorf("expected string slice: %q, got: %q", expect, actual)
		return false
	}
	for i := 0; i < len(expect); i++ {
		if expect[i] != actual[i] {
			t.Errorf("expected string slice: %q, got: %q", expect, actual)
			return false
		}
	}
	return true
}

func TestBool(t *testing.T) {
	v := false
	parseFlag(t, BoolVar(&v, "foo", false, "").MustBuild(), "--foo")
	assertBool(t, true, v)
}

func TestDuration(t *testing.T) {
	var v time.Duration
	parseFlag(t, DurationVar(&v, "foo", 0, "").MustBuild(), "--foo=1s")
	assertDuration(t, time.Second, v)
}

func TestFloat64(t *testing.T) {
	var v float64
	parseFlag(t, Float64Var(&v, "foo", 0, "").MustBuild(), "--foo=1.0")
	assertFloat64(t, 1.0, v)
}

func TestInt64(t *testing.T) {
	var v int64
	parseFlag(t, Int64Var(&v, "foo", 0, "").MustBuild(), "--foo=1")
	assertInt64(t, 1, v)
}

func TestString(t *testing.T) {
	var v string
	parseFlag(t, StringVar(&v, "foo", "", "").MustBuild(), "--foo=bar")
	assertString(t, "bar", v)
}

func TestStringSlice(t *testing.T) {
	var v []string
	parseFlag(
		t,
		StringSliceVar(&v, "foo", nil, "").MustBuild(),
		"--foo", "baz", "--foo", "qux",
	)
	assertStringSlice(t, []string{"baz", "qux"}, v)
}
