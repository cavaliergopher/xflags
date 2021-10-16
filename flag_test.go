package xflags

import (
	"testing"
	"time"
)

func parseFlag(t *testing.T, flag *FlagInfo, args ...string) {
	_, err := Command("test").Flags(flag).Must().Parse(args)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBitField(t *testing.T) {
	var v uint64
	_, err := Command("test").
		Flags(
			BitFieldVar(&v, 0x01, "foo", false, "").Must(),
			BitFieldVar(&v, 0x02, "bar", false, "").Must(),
			BitFieldVar(&v, 0x04, "baz", true, "").Must(),
		).
		Must().
		Parse([]string{"--foo"})
	if err != nil {
		t.Fatal(err)
	}
	assertInt64(t, 0x05, int64(v))
}

func TestBool(t *testing.T) {
	v := false
	parseFlag(t, BoolVar(&v, "foo", false, "").Must(), "--foo")
	assertBool(t, true, v)
}

func TestDuration(t *testing.T) {
	var v time.Duration
	parseFlag(t, DurationVar(&v, "foo", 0, "").Must(), "--foo=1s")
	assertDuration(t, time.Second, v)
}

func TestFloat64(t *testing.T) {
	var v float64
	parseFlag(t, Float64Var(&v, "foo", 0, "").Must(), "--foo=1.0")
	assertFloat64(t, 1.0, v)
}

func TestInt64(t *testing.T) {
	var v int64
	parseFlag(t, Int64Var(&v, "foo", 0, "").Must(), "--foo=1")
	assertInt64(t, 1, v)
}

func TestString(t *testing.T) {
	var v string
	parseFlag(t, StringVar(&v, "foo", "", "").Must(), "--foo=bar")
	assertString(t, "bar", v)
}

func TestStringSlice(t *testing.T) {
	var v []string
	parseFlag(
		t,
		StringSliceVar(&v, "foo", nil, "").Must(),
		"--foo", "baz", "--foo", "qux",
	)
	assertStringSlice(t, []string{"baz", "qux"}, v)
}
