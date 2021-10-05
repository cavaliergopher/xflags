package xflags

import (
	"strconv"
	"time"
)

type Value interface {
	// Set parses the given argument and stores the parsed value in the
	// underlying variable.
	Set(s string) error

	// Reset is called only the first time a command line argument is seen to
	// reset the state of the value from its default value. This is useful for
	// flags that incrementally build the state of the value, such as a
	// StringSlice.
	Reset()
}

type ValueFunc func(s string) error

func (f ValueFunc) Set(s string) error { return f(s) }

func (f ValueFunc) Reset() {}

// compile-time interface assertion
var _ Value = ValueFunc(func(s string) error { return nil })

type boolValue bool

func (p *boolValue) Reset() {}

func (p *boolValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	*p = boolValue(v)
	return nil
}

type durationValue time.Duration

func (p *durationValue) Reset() {}

func (p *durationValue) Set(s string) error {
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*p = durationValue(v)
	return nil
}

type float64Value float64

func (p *float64Value) Reset() {}

func (p *float64Value) Set(s string) error {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*p = float64Value(v)
	return nil
}

type int64Value int64

func (p *int64Value) Reset() {}

func (p *int64Value) Set(s string) error {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*p = int64Value(v)
	return nil
}

type stringValue string

func (p *stringValue) Reset() {}

func (p *stringValue) Set(s string) error {
	*p = stringValue(s)
	return nil
}

type stringSliceValue []string

func (p *stringSliceValue) Reset() {
	*p = make([]string, 0)
}

func (p *stringSliceValue) Set(s string) error {
	*p = append(*p, s)
	return nil
}
