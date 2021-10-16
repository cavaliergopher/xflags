package xflags

import (
	"fmt"
	"strconv"
	"time"
)

// Value is the interface to the dynamic value stored in a flag.
// (The default value is represented as a string.)
//
// Set is called once, in command line order, for each flag present.
type Value interface {
	String() string
	Set(s string) error
}

// BoolValue is an optional interface to indicate boolean flags that can be
// supplied without a "=value" argument.
type BoolValue interface {
	Value
	IsBoolFlag() bool
}

func isBoolValue(v Value) bool {
	if bv, ok := v.(BoolValue); ok {
		return bv.IsBoolFlag()
	}
	return false
}

type bitFieldValue struct {
	p    *uint64
	mask uint64
}

func newBitFieldValue(val bool, p *uint64, mask uint64) *bitFieldValue {
	v := &bitFieldValue{p: p, mask: mask}
	v.set(val)
	return v
}

func (p *bitFieldValue) IsBoolFlag() bool { return true }

func (p *bitFieldValue) String() string { return fmt.Sprintf("0x%0x", *p.p) }

func (p *bitFieldValue) Get() interface{} { return *p.p }

func (p *bitFieldValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	p.set(v)
	return nil
}

func (p *bitFieldValue) set(v bool) {
	if v {
		*p.p |= p.mask
	} else {
		*p.p &= ^p.mask
	}
}

type boolValue bool

func newBoolValue(val bool, p *bool) *boolValue {
	*p = val
	return (*boolValue)(p)
}

func (p *boolValue) IsBoolFlag() bool { return true }

func (p *boolValue) String() string { return strconv.FormatBool((bool)(*p)) }

func (p *boolValue) Get() interface{} { return (bool)(*p) }

func (p *boolValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	*p = boolValue(v)
	return nil
}

type durationValue time.Duration

func newDurationValue(val time.Duration, p *time.Duration) *durationValue {
	*p = val
	return (*durationValue)(p)
}

func (p *durationValue) String() string { return (time.Duration)(*p).String() }

func (p *durationValue) Get() interface{} { return (time.Duration)(*p) }

func (p *durationValue) Set(s string) error {
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*p = durationValue(v)
	return nil
}

type float64Value float64

func newFloat64Value(val float64, p *float64) *float64Value {
	*p = val
	return (*float64Value)(p)
}

func (p *float64Value) String() string {
	return strconv.FormatFloat((float64)(*p), 'e', -1, 64)
}

func (p *float64Value) Get() interface{} { return (float64)(*p) }

func (p *float64Value) Set(s string) error {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*p = float64Value(v)
	return nil
}

type int64Value int64

func newInt64Value(val int64, p *int64) *int64Value {
	*p = val
	return (*int64Value)(p)
}

func (p *int64Value) String() string {
	return strconv.FormatInt((int64)(*p), 10)
}

func (p *int64Value) Get() interface{} { return (int64)(*p) }

func (p *int64Value) Set(s string) error {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*p = int64Value(v)
	return nil
}

type stringValue string

func newStringValue(val string, p *string) *stringValue {
	*p = val
	return (*stringValue)(p)
}

func (p *stringValue) String() string { return (string)(*p) }

func (p *stringValue) Get() interface{} { return (string)(*p) }

func (p *stringValue) Set(s string) error {
	*p = stringValue(s)
	return nil
}

type stringSliceValue struct {
	p   *[]string
	hot bool
}

func newStringSliceValue(val []string, p *[]string) *stringSliceValue {
	*p = val
	return &stringSliceValue{p: p}
}

func (p *stringSliceValue) String() string {
	return fmt.Sprintf("%v", *p.p)
}

func (p *stringSliceValue) Get() interface{} { return *p.p }

func (p *stringSliceValue) Set(s string) error {
	if !p.hot {
		*p.p = make([]string, 0, 1)
		p.hot = true
	}
	*p.p = append(*p.p, s)
	return nil
}
