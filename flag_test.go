package xflags

import (
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBitField(t *testing.T) {
	var v uint64
	_, err := NewCommand("test", "").
		Flags(
			BitField(&v, 0x01, "foo", false, "").Must(),
			BitField(&v, 0x02, "bar", false, "").Must(),
			BitField(&v, 0x04, "baz", true, "").Must(),
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
	if assertFlagParses(t, Bool(&v, "foo", false, "").Must(), "--foo") {
		assertBool(t, true, v)
	}
}

func TestDuration(t *testing.T) {
	var v time.Duration
	if assertFlagParses(t, Duration(&v, "foo", 0, "").Must(), "--foo=1s") {
		assertDuration(t, time.Second, v)
	}
}

func TestFloat64(t *testing.T) {
	var v float64
	if assertFlagParses(t, Float64(&v, "foo", 0, "").Must(), "--foo=1.0") {
		assertFloat64(t, 1.0, v)
	}
	if assertFlagParses(t, Float64(&v, "foo-negative", 0, "").Must(), "--foo-negative=-1.0") {
		assertFloat64(t, -1.0, v)
	}
}

func TestInt64(t *testing.T) {
	var v int64
	if assertFlagParses(t, Int64(&v, "foo", 0, "").Must(), "--foo=1") {
		assertInt64(t, 1, v)
	}
	if assertFlagParses(t, Int64(&v, "foo-negative", 0, "").Must(), "--foo-negative=-1") {
		assertInt64(t, -1, v)
	}
}

func TestString(t *testing.T) {
	var v string
	if assertFlagParses(t, String(&v, "foo", "", "").Must(), "--foo=bar") {
		assertString(t, "bar", v)
	}
}

func TestStringSlice(t *testing.T) {
	var v []string
	if assertFlagParses(
		t,
		Strings(&v, "foo", nil, "").Must(),
		"--foo", "baz", "--foo", "qux",
	) {
		assertStrings(t, []string{"baz", "qux"}, v)
	}
}

func TestFlagChoices(t *testing.T) {
	var v string
	flag := String(&v, "foo", "", "").Choices("bar", "baz").Must()
	assertFlagParses(t, flag, "--foo=bar")
	assertFlagParses(t, flag, "--foo=baz")
	assertErrorAs(t, parseFlag(flag, "--foo=qux"), &ArgumentError{})
	assertErrorAs(t, parseFlag(flag, "--foo=ba"), &ArgumentError{})
	assertErrorAs(t, parseFlag(flag, "--foo=barr"), &ArgumentError{})
}

func ExampleFlagBuilder_Validate() {
	var ip string

	cmd := NewCommand("ping", "").
		Output(os.Stdout, os.Stdout). // for tests
		Flags(
			String(&ip, "ip", "127.0.0.1", "IP Address to ping").
				Validate(func(arg string) error {
					if net.ParseIP(arg) == nil {
						return fmt.Errorf("invalid IP: %s", arg)
					}
					return nil
				}),
		).
		HandleFunc(func(args []string) (exitCode int) {
			fmt.Printf("ping: %s\n", ip)
			return
		})

	RunWithArgs(cmd, "--ip=127.0.0.1")

	// 256 is not a valid IPv4 component
	RunWithArgs(cmd, "--ip=256.0.0.1")
	// Output:
	// ping: 127.0.0.1
	// Argument error: --ip: invalid IP: 256.0.0.1
}

func ExampleBitField() {
	const (
		UserRead    uint64 = 0400
		UserWrite   uint64 = 0200
		UserExecute uint64 = 0100
	)

	var mode uint64 = 0444 // -r--r--r--

	cmd := NewCommand("user-allow", "").
		Flags(
			BitField(&mode, UserRead, "r", false, "Enable user read"),
			BitField(&mode, UserWrite, "w", false, "Enable user write"),
			BitField(&mode, UserExecute, "x", false, "Enable user execute"),
		).
		HandleFunc(func(args []string) (exitCode int) {
			fmt.Printf("File mode: %s\n", os.FileMode(mode))
			return
		})

	// Enable user read and write
	RunWithArgs(cmd, "-r", "-w")
	// Output: File mode: -rw-r--r--
}

func ExampleFunc() {
	var ip net.IP

	cmd := NewCommand("ping", "").
		Output(os.Stdout, os.Stdout). // for tests
		Flags(
			Func("ip", "IP address to ping", func(s string) error {
				ip = net.ParseIP(s)
				if ip == nil {
					return fmt.Errorf("invalid IP: %s", s)
				}
				return nil
			}),
		).
		HandleFunc(func(args []string) (exitCode int) {
			fmt.Printf("ping: %s\n", ip)
			return
		})

	RunWithArgs(cmd, "--ip", "127.0.0.1")

	// 256 is not a valid IPv4 component
	RunWithArgs(cmd, "--ip", "256.0.0.1")
	// Output:
	// ping: 127.0.0.1
	// Argument error: --ip: invalid IP: 256.0.0.1
}

func ExampleStrings() {
	var widgets []string

	cmd := NewCommand("create-widgets", "").
		Flags(
			// Configure a repeatable string slice flag that must be specified
			// at least once.
			Strings(&widgets, "name", nil, "Widget name").NArgs(1, 0),
		).
		HandleFunc(func(args []string) (exitCode int) {
			fmt.Printf("Created new widgets: %s", strings.Join(widgets, ", "))
			return
		})

	RunWithArgs(cmd, "--name=foo", "--name=bar")
	// Output: Created new widgets: foo, bar
}
