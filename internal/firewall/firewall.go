// Package firewall explains candidate local policy rules, never reachability.
package firewall

import (
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
)

type Source struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}
type Profile struct {
	Name     string `json:"name"`
	Enabled  string `json:"enabled"`
	Inbound  string `json:"default_inbound"`
	Outbound string `json:"default_outbound"`
	Detail   string `json:"detail,omitempty"`
}
type Interface struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
	Profile   string   `json:"profile,omitempty"`
}
type Rule struct {
	IdentityConditions []string `json:"identity_conditions,omitempty"`
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Source             string   `json:"source"`
	Direction          string   `json:"direction"`
	Action             string   `json:"action"`
	Family             string   `json:"family"`
	Protocols          []string `json:"protocols"`
	LocalPorts         []string `json:"local_ports"`
	RemotePorts        []string `json:"remote_ports"`
	LocalAddresses     []string `json:"local_addresses"`
	RemoteAddresses    []string `json:"remote_addresses"`
	Program            string   `json:"program,omitempty"`
	Profiles           string   `json:"profiles,omitempty"`
	Interfaces         string   `json:"interfaces,omitempty"`
	Conditions         []string `json:"conditions"`
	Raw                string   `json:"raw,omitempty"`
}
type Snapshot struct {
	Coverage   string         `json:"coverage"`
	Sources    []Source       `json:"sources"`
	Profiles   []Profile      `json:"profiles"`
	Interfaces []Interface    `json:"interfaces"`
	Notes      []string       `json:"notes"`
	Rules      []Rule         `json:"-"`
	Paths      map[int]string `json:"-"`
}
type Candidate struct {
	Relevance string   `json:"relevance"`
	Rule      Rule     `json:"rule"`
	Match     string   `json:"match"`
	Reasons   []string `json:"reasons"`
}
type Direction struct {
	Assessment string      `json:"assessment"`
	Summary    string      `json:"summary"`
	Rules      []Candidate `json:"rules"`
}
type Context struct {
	Coverage          string    `json:"coverage"`
	Peer              string    `json:"peer,omitempty"`
	PeerCompatibility string    `json:"peer_compatibility"`
	LocalAddresses    []string  `json:"local_addresses"`
	Inbound           Direction `json:"inbound"`
	Outbound          Direction `json:"outbound"`
}

func Empty() Snapshot {
	return Snapshot{Coverage: "unknown", Sources: []Source{}, Profiles: []Profile{}, Interfaces: []Interface{}, Notes: []string{}, Rules: []Rule{}, Paths: map[int]string{}}
}

func ParsePeer(value string) (netip.Addr, error) {
	a, err := netip.ParseAddr(value)
	if err == nil {
		a = a.Unmap()
	}
	if err != nil || !(a.IsGlobalUnicast() || a.IsLoopback() || a.IsLinkLocalUnicast()) {
		return netip.Addr{}, fmt.Errorf("peer must be a numeric unicast IP address (no hostname or port)")
	}
	return a.Unmap(), nil
}

// Explain filters only conditions that can be disproved from this snapshot.
// Remaining rules are conditional; no allow/deny verdict is inferred from them.
func Explain(s Snapshot, protocol, local string, pids []int, peer netip.Addr) Context {
	c := Context{Coverage: s.Coverage, PeerCompatibility: "not-specified", LocalAddresses: []string{},
		Inbound: Direction{Assessment: "unknown", Rules: []Candidate{}}, Outbound: Direction{Assessment: "unknown", Rules: []Candidate{}}}
	endpoint, err := netip.ParseAddrPort(local)
	if err != nil {
		return c
	}
	addr := endpoint.Addr().Unmap()
	if peer.IsValid() {
		c.Peer = peer.String()
		c.PeerCompatibility = "same-family; interface and routing not evaluated"
		if peer.Is4() != addr.Is4() {
			c.PeerCompatibility = "different-family; dual-stack acceptance not established"
		} else if addr.IsLoopback() && !peer.IsLoopback() {
			c.PeerCompatibility = "local-only binding; peer is not loopback"
		}
	}
	if addr.IsUnspecified() {
		for _, iface := range s.Interfaces {
			for _, a := range iface.Addresses {
				prefix, e := netip.ParsePrefix(a)
				if e == nil && prefix.Addr().Unmap().Is4() == addr.Is4() {
					c.LocalAddresses = append(c.LocalAddresses, iface.Name+"="+prefix.Addr().String())
				}
			}
		}
	} else {
		c.LocalAddresses = append(c.LocalAddresses, addr.String())
	}
	sort.Strings(c.LocalAddresses)
	if strings.HasPrefix(c.PeerCompatibility, "different-family") || strings.HasPrefix(c.PeerCompatibility, "local-only") {
		c.Inbound.Summary = "No same-family inbound candidates evaluated for this peer; this is not a firewall verdict."
		c.Outbound.Summary = "No same-family outbound candidates evaluated for this peer; this is not a firewall verdict."
		return c
	}
	for _, rule := range s.Rules {
		if rule.Family == "ipv4" && !addr.Is4() || rule.Family == "ipv6" && addr.Is4() {
			continue
		}
		if !protocolMayMatch(rule.Protocols, protocol) || !portMayMatch(rule.LocalPorts, endpoint.Port()) {
			continue
		}
		if !addr.IsUnspecified() && !addressMayMatch(rule.LocalAddresses, addr) {
			continue
		}
		if peer.IsValid() && !addressMayMatch(rule.RemoteAddresses, peer) {
			continue
		}
		// Never identify an application from its basename. All owners must have a
		// path before a program-restricted rule can be excluded on path mismatch.
		reasons := []string{"Candidate only: rule order, active interface/profile, state, identity, and other filters may change the outcome."}
		if rule.Program != "" && !strings.EqualFold(rule.Program, "any") && rule.Program != "*" {
			known, match := len(pids) > 0, false
			for _, pid := range pids {
				path := s.Paths[pid]
				if path == "" {
					known = false
				}
				if programEqual(path, rule.Program, rule.Source) {
					match = true
				}
			}
			if known && !match {
				continue
			}
			if !known {
				reasons = append(reasons, "Owning executable path is unavailable; application restriction is unresolved.")
			} else {
				reasons = append(reasons, "Executable path observed; PID reuse and socket/process timing remain possible.")
			}
		}
		if addr.IsUnspecified() && !allAny(rule.LocalAddresses) {
			reasons = append(reasons, "Wildcard binding: this rule restricts local addresses; applicability varies by interface/address.")
		}
		if !peer.IsValid() {
			reasons = append(reasons, "Peer IP was not specified; inspect the rule's remote address restrictions.")
		}
		if !allAny(rule.RemotePorts) {
			reasons = append(reasons, "Remote port is not specified; this rule's remote-port restriction is unresolved.")
		}
		relevance := "unverified"
		identityUnresolved := len(rule.IdentityConditions) > 0
		if rule.Program != "" && !strings.EqualFold(rule.Program, "any") && rule.Program != "*" {
			matched := false
			for _, pid := range pids {
				matched = matched || programEqual(s.Paths[pid], rule.Program, rule.Source)
			}
			if matched && !identityUnresolved {
				relevance = "process"
			}
		} else if knownPortRestriction(rule.LocalPorts) && !identityUnresolved {
			relevance = "endpoint"
		}
		if identityUnresolved {
			reasons = append(reasons, "Application package, service, user or security restrictions are unresolved; this rule is not established as relevant to the socket owner.")
		}
		candidate := Candidate{Rule: rule, Match: "conditional", Reasons: reasons, Relevance: relevance}
		if rule.Direction == "inbound" || rule.Direction == "both" || rule.Direction == "unknown" {
			c.Inbound.Rules = append(c.Inbound.Rules, candidate)
		}
		if rule.Direction == "outbound" || rule.Direction == "both" || rule.Direction == "unknown" {
			c.Outbound.Rules = append(c.Outbound.Rules, candidate)
		}
	}
	for _, direction := range []*Direction{&c.Inbound, &c.Outbound} {
		sort.SliceStable(direction.Rules, func(i, j int) bool {
			a, b := direction.Rules[i].Rule, direction.Rules[j].Rule
			if (a.Action == "block") != (b.Action == "block") {
				return a.Action == "block"
			}
			if a.Source != b.Source {
				return a.Source < b.Source
			}
			if a.ID != b.ID {
				return a.ID < b.ID
			}
			return a.Name < b.Name
		})
		allow, block := false, false
		for _, r := range direction.Rules {
			allow = allow || r.Rule.Action == "allow"
			block = block || r.Rule.Action == "block"
		}
		switch {
		case allow && block:
			direction.Summary = "Allow and block candidates coexist; overlapping applicability and precedence are unresolved."
		case len(direction.Rules) > 0:
			direction.Summary = "Conditional rule candidates found; no final allow/block verdict."
		default:
			direction.Summary = "No candidates found in collected rules; this does not imply allowed or blocked."
		}
	}
	return c
}
func programEqual(a, b, source string) bool {
	if strings.HasPrefix(source, "windows-") {
		return strings.EqualFold(a, b)
	}
	if source == "macos-application-firewall" && strings.HasSuffix(b, ".app") && strings.HasPrefix(a, b+"/") {
		return true
	}
	return a == b
}
func allAny(values []string) bool {
	return len(values) == 1 && (strings.EqualFold(values[0], "any") || values[0] == "*")
}
func protocolMayMatch(values []string, protocol string) bool {
	if len(values) == 0 {
		return true
	}
	for _, v := range values {
		switch strings.ToLower(v) {
		case "any", "*", "all", "0", "256":
			return true
		case "tcp", "6":
			if protocol == "tcp" {
				return true
			}
		case "udp", "17":
			if protocol == "udp" {
				return true
			}
		default:
			if _, e := strconv.Atoi(v); e != nil {
				return true
			}
		}
	}
	return false
}
func portMayMatch(values []string, port uint16) bool {
	if len(values) == 0 {
		return true
	}
	unknown := false
	for _, v := range values {
		v = strings.TrimSpace(v)
		if strings.EqualFold(v, "any") || v == "*" {
			return true
		}
		if a, b, ok := strings.Cut(v, "-"); ok {
			lo, e1 := strconv.Atoi(a)
			hi, e2 := strconv.Atoi(b)
			if e1 != nil || e2 != nil || lo < 0 || hi > 65535 || lo > hi {
				unknown = true
			} else if int(port) >= lo && int(port) <= hi {
				return true
			}
			continue
		}
		n, e := strconv.Atoi(v)
		if e != nil || n < 0 || n > 65535 {
			unknown = true
		} else if n == int(port) {
			return true
		}
	}
	return unknown
}
func addressMayMatch(values []string, address netip.Addr) bool {
	if len(values) == 0 {
		return true
	}
	unknown := false
	address = address.WithZone("").Unmap()
	for _, v := range values {
		v = strings.TrimSpace(v)
		if strings.EqualFold(v, "any") || v == "*" {
			return true
		}
		if ipValue, maskValue, ok := strings.Cut(v, "/"); ok {
			ip, ipErr := netip.ParseAddr(ipValue)
			mask, maskErr := netip.ParseAddr(maskValue)
			if ipErr == nil && maskErr == nil && ip.Unmap().Is4() && mask.Unmap().Is4() {
				raw := mask.Unmap().As4()
				ones, bits := net.IPMask(raw[:]).Size()
				if bits == 32 {
					if netip.PrefixFrom(ip.Unmap(), ones).Contains(address) {
						return true
					}
					continue
				}
			}
		}
		if prefix, e := netip.ParsePrefix(v); e == nil {
			if prefix.Addr().Is4In6() {
				if prefix.Bits() < 96 {
					unknown = true
					continue
				}
				prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
			}
			if prefix.Contains(address) {
				return true
			}
			continue
		}
		if a, b, ok := strings.Cut(v, "-"); ok {
			lo, e1 := netip.ParseAddr(a)
			hi, e2 := netip.ParseAddr(b)
			if e1 != nil || e2 != nil || lo.BitLen() != hi.BitLen() || lo.Compare(hi) > 0 {
				unknown = true
			} else if address.BitLen() == lo.Unmap().BitLen() && address.Compare(lo.Unmap()) >= 0 && address.Compare(hi.Unmap()) <= 0 {
				return true
			}
			continue
		}
		if ip, e := netip.ParseAddr(v); e == nil {
			if ip.WithZone("").Unmap() == address {
				return true
			}
		} else {
			unknown = true
		}
	}
	return unknown
}

// A known local-port restriction establishes endpoint relation, not permission.
// Missing values, dynamic keywords and unrestricted ranges stay unverified.
func knownPortRestriction(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if a, b, ok := strings.Cut(value, "-"); ok {
			lo, e1 := strconv.Atoi(a)
			hi, e2 := strconv.Atoi(b)
			if e1 != nil || e2 != nil || lo < 0 || hi > 65535 || lo > hi || (lo <= 1 && hi == 65535) {
				return false
			}
			continue
		}
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 65535 {
			return false
		}
	}
	return true
}
