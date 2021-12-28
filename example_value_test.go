package xflags

import (
	"fmt"
	"net"
)

// ipValue implements the Value interface for net.IP.
type ipValue net.IP

func (p *ipValue) Set(s string) error {
	ip := net.ParseIP(s)
	if ip == nil {
		return fmt.Errorf("invalid IP: %s", s)
	}
	*p = ipValue(ip)
	return nil
}

// IPVar returns a FlagBuilder that can be used to define a net.IP flag with
// specified name, default value, and usage string. The argument p points to a
// net.IP variable in which to store the value of the flag.
func IPVar(p *net.IP, name string, value net.IP, usage string) *FlagBuilder {
	*p = value
	return Var((*ipValue)(p), name, usage)
}

func ExampleValue() {
	var ip net.IP

	cmd := NewCommand("ping", "").
		Flags(
			// configure a net.IP flag with our custom Value type
			IPVar(&ip, "ip", net.IPv6loopback, "IP address to ping"),
		).
		HandleFunc(func(args []string) (exitCode int) {
			fmt.Printf("ping: %s\n", ip)
			return
		})

	RunWithArgs(cmd, "--ip=ff02:0000:0000:0000:0000:0000:0000:0001")

	// Output: ping: ff02::1
}
