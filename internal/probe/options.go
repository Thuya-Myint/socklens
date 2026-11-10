// Package probe performs bounded TCP connection checks for explicit targets.
package probe

import (
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const MaxPorts = 64
const MaxAddresses = 16
const MaxAttempts = 256

type Options struct {
	Host           string
	Ports          []uint16
	Family         string
	ConnectTimeout time.Duration
	Timeout        time.Duration
	Concurrency    int
}

func ParsePorts(value string) ([]uint16, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("--ports requires a comma-separated list of numeric TCP ports")
	}
	ports := []uint16{}
	seen := map[uint16]bool{}
	for _, field := range strings.Split(value, ",") {
		field = strings.TrimSpace(field)
		if field == "" || strings.IndexFunc(field, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return nil, fmt.Errorf("ports must be comma-separated integers from 1 to 65535; ranges and service names are not accepted")
		}
		n, err := strconv.Atoi(field)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("port must be 1-65535")
		}
		port := uint16(n)
		if !seen[port] {
			ports = append(ports, port)
			seen[port] = true
		}
		if len(ports) > MaxPorts {
			return nil, fmt.Errorf("at most %d distinct ports can be requested", MaxPorts)
		}
	}
	slices.Sort(ports)
	return ports, nil
}
func (o Options) Normalize() (Options, error) {
	host, err := normalizeHost(o.Host)
	if err != nil {
		return Options{}, err
	}
	o.Host = host
	if o.Family != "any" && o.Family != "ipv4" && o.Family != "ipv6" {
		return Options{}, fmt.Errorf("family must be any, ipv4, or ipv6")
	}
	if o.ConnectTimeout < 100*time.Millisecond || o.ConnectTimeout > 30*time.Second {
		return Options{}, fmt.Errorf("connect-timeout must be between 100ms and 30s")
	}
	if o.Timeout < time.Second || o.Timeout > 5*time.Minute {
		return Options{}, fmt.Errorf("timeout must be between 1s and 5m")
	}
	if o.Concurrency < 1 || o.Concurrency > 32 {
		return Options{}, fmt.Errorf("concurrency must be between 1 and 32")
	}
	if len(o.Ports) == 0 {
		return Options{}, fmt.Errorf("at least one port is required")
	}
	o.Ports = append([]uint16{}, o.Ports...)
	slices.Sort(o.Ports)
	o.Ports = slices.Compact(o.Ports)
	if len(o.Ports) > MaxPorts {
		return Options{}, fmt.Errorf("at most %d distinct ports can be requested", MaxPorts)
	}
	if o.Ports[0] == 0 {
		return Options{}, fmt.Errorf("port must be 1-65535")
	}
	if address, err := netip.ParseAddr(o.Host); err == nil && !familyMatches(address, o.Family) {
		return Options{}, fmt.Errorf("host address does not match requested family %s", o.Family)
	}
	return o, nil
}
func normalizeHost(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("--host is required")
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("host contains control characters")
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = value[1 : len(value)-1]
		if address, err := netip.ParseAddr(value); err != nil || !address.Is6() {
			return "", fmt.Errorf("brackets require a numeric IPv6 address")
		}
	}
	if address, err := netip.ParseAddr(value); err == nil {
		address = address.Unmap()
		if !unicast(address) {
			return "", fmt.Errorf("host must be a unicast IP address; unspecified, multicast and broadcast addresses are not accepted")
		}
		return address.String(), nil
	}
	name := strings.TrimSuffix(value, ".")
	if len(name) == 0 || len(name) > 253 {
		return "", fmt.Errorf("host must be one numeric IP or ASCII hostname, without URL, CIDR, or port")
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("invalid hostname label")
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
				return "", fmt.Errorf("host must be one numeric IP or ASCII hostname, without URL, CIDR, or port")
			}
		}
	}
	return value, nil
}

// NormalizeHost validates one explicit unicast IP or ASCII DNS name.
func NormalizeHost(value string) (string, error) { return normalizeHost(value) }
func unicast(a netip.Addr) bool {
	return a.IsGlobalUnicast() || a.IsLoopback() || a.IsLinkLocalUnicast()
}
func familyMatches(address netip.Addr, family string) bool {
	address = address.Unmap()
	return family == "any" || family == "ipv4" && address.Is4() || family == "ipv6" && address.Is6()
}
