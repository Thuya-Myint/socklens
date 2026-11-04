package firewall

import (
	"net/netip"
	"strings"
	"testing"
)

func testRule() Rule {
	return Rule{ID: "test", Source: "windows-active-store", Direction: "inbound", Action: "allow", Family: "ipv4", Protocols: []string{"tcp"}, LocalPorts: []string{"5432"}, RemotePorts: []string{"Any"}, LocalAddresses: []string{"Any"}, RemoteAddresses: []string{"192.168.1.0/24"}, Conditions: []string{"Profile Private; service restrictions unresolved"}}
}
func TestPeerAndRuleFiltering(t *testing.T) {
	for _, tc := range []struct {
		name, peer, protocol, local string
		change                      func(*Rule)
		count                       int
	}{
		{"subnet match", "192.168.1.20", "tcp", "0.0.0.0:5432", nil, 1},
		{"subnet mismatch", "192.168.2.20", "tcp", "0.0.0.0:5432", nil, 0},
		{"protocol mismatch", "192.168.1.20", "udp", "0.0.0.0:5432", nil, 0},
		{"port mismatch", "192.168.1.20", "tcp", "0.0.0.0:443", nil, 0},
		{"range match", "192.168.1.20", "tcp", "0.0.0.0:5432", func(r *Rule) {
			r.RemoteAddresses = []string{"192.168.1.10-192.168.1.30"}
			r.LocalPorts = []string{"5400-5500"}
		}, 1},
		{"range mismatch", "192.168.1.40", "tcp", "0.0.0.0:5432", func(r *Rule) { r.RemoteAddresses = []string{"192.168.1.10-192.168.1.30"} }, 0},
		{"dynamic unknown", "203.0.113.1", "tcp", "0.0.0.0:5432", func(r *Rule) { r.RemoteAddresses = []string{"LocalSubnet"}; r.LocalPorts = []string{"RPC"} }, 1},
		{"mapped peer", "::ffff:192.168.1.20", "tcp", "0.0.0.0:5432", nil, 1},
		{"mapped prefix", "192.168.1.20", "tcp", "0.0.0.0:5432", func(r *Rule) { r.RemoteAddresses = []string{"::ffff:192.168.1.0/120"} }, 1},
		{"family mismatch", "2001:db8::1", "tcp", "0.0.0.0:5432", nil, 0},
		{"loopback restriction", "192.168.1.20", "tcp", "127.0.0.1:5432", nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := testRule()
			if tc.change != nil {
				tc.change(&r)
			}
			s := Empty()
			s.Coverage = "partial"
			s.Rules = []Rule{r}
			peer, e := ParsePeer(tc.peer)
			if e != nil {
				t.Fatal(e)
			}
			c := Explain(s, tc.protocol, tc.local, nil, peer)
			if len(c.Inbound.Rules) != tc.count {
				t.Fatalf("%+v", c)
			}
			if c.Inbound.Assessment != "unknown" || c.Outbound.Assessment != "unknown" {
				t.Fatal("policy context claims final decision")
			}
			for _, candidate := range c.Inbound.Rules {
				if candidate.Match != "conditional" {
					t.Fatal(candidate)
				}
			}
		})
	}
}
func TestProgramDirectionConflictAndVisibility(t *testing.T) {
	s := Empty()
	s.Coverage = "partial"
	r := testRule()
	r.Program = `C:\App\server.exe`
	s.Rules = []Rule{r}
	s.Paths[42] = `C:\Other\server.exe`
	peer := netip.MustParseAddr("192.168.1.20")
	if len(Explain(s, "tcp", "0.0.0.0:5432", []int{42}, peer).Inbound.Rules) != 0 {
		t.Fatal("basename used as executable identity")
	}
	s.Paths[42] = `c:\app\SERVER.exe`
	r.Action = "block"
	r.ID = "block"
	s.Rules = append(s.Rules, r)
	c := Explain(s, "tcp", "0.0.0.0:5432", []int{42}, peer)
	if len(c.Inbound.Rules) != 2 || c.Inbound.Rules[0].Rule.Action != "block" || !strings.Contains(c.Inbound.Summary, "coexist") {
		t.Fatal(c)
	}
	delete(s.Paths, 42)
	if len(Explain(s, "tcp", "0.0.0.0:5432", []int{42}, peer).Inbound.Rules) != 2 {
		t.Fatal("missing path treated as nonmatch")
	}
	s.Rules[0].Direction = "outbound"
	s.Rules = s.Rules[:1]
	c = Explain(s, "tcp", "0.0.0.0:5432", nil, peer)
	if len(c.Inbound.Rules) != 0 || len(c.Outbound.Rules) != 1 {
		t.Fatal(c)
	}
	s.Rules = nil
	c = Explain(s, "tcp", "0.0.0.0:5432", nil, peer)
	if !strings.Contains(c.Inbound.Summary, "does not imply") {
		t.Fatal("empty rules imply safety")
	}
}
func TestWildcardAddressesAndIPv6(t *testing.T) {
	s := Empty()
	r := testRule()
	r.Family = "ipv6"
	r.RemoteAddresses = []string{"2001:db8::/32"}
	s.Rules = []Rule{r}
	s.Interfaces = []Interface{{Name: "eth0", Addresses: []string{"10.0.0.1/24", "2001:db8::2/64"}}}
	c := Explain(s, "tcp", "[::]:5432", nil, netip.MustParseAddr("2001:db8::1"))
	if len(c.Inbound.Rules) != 1 || len(c.LocalAddresses) != 1 || !strings.Contains(c.LocalAddresses[0], "2001:db8::2") {
		t.Fatal(c)
	}
	c = Explain(s, "tcp", "[::]:5432", nil, netip.MustParseAddr("192.168.1.20"))
	if len(c.Inbound.Rules) != 0 || !strings.Contains(c.PeerCompatibility, "dual-stack") {
		t.Fatal(c)
	}
}
func TestPeerValidation(t *testing.T) {
	for _, value := range []string{"", "example.com", "192.168.1.1:80", "0.0.0.0", "::", "224.0.0.1", "ff02::1", "255.255.255.255", "::ffff:224.0.0.1"} {
		if _, err := ParsePeer(value); err == nil {
			t.Fatal("accepted", value)
		}
	}
}

func TestWindowsDottedNetmask(t *testing.T) {
	if !addressMayMatch([]string{"127.0.0.0/255.0.0.0"}, netip.MustParseAddr("127.0.0.1")) {
		t.Fatal("Windows dotted netmask not recognized")
	}
	if addressMayMatch([]string{"127.0.0.0/255.0.0.0"}, netip.MustParseAddr("192.168.1.20")) {
		t.Fatal("dotted netmask retained unrelated peer")
	}
	if !addressMayMatch([]string{"10.0.0.0/255.0.255.0"}, netip.MustParseAddr("192.168.1.20")) {
		t.Fatal("invalid noncontiguous mask treated as definite nonmatch")
	}
}
func TestRuleRelevanceAndUnresolvedIdentity(t *testing.T) {
	r := testRule()
	s := Empty()
	s.Rules = []Rule{r}
	peer := netip.MustParseAddr("192.168.1.20")
	got := Explain(s, "tcp", "0.0.0.0:5432", []int{42}, peer)
	if got.Inbound.Rules[0].Relevance != "endpoint" {
		t.Fatal(got)
	}
	s.Rules[0].Program = `C:\Apps\server.exe`
	got = Explain(s, "tcp", "0.0.0.0:5432", []int{42}, peer)
	if got.Inbound.Rules[0].Relevance != "unverified" {
		t.Fatal("missing path established process relevance")
	}
	s.Paths[42] = `C:\Apps\server.exe`
	got = Explain(s, "tcp", "0.0.0.0:5432", []int{42}, peer)
	if got.Inbound.Rules[0].Relevance != "process" {
		t.Fatal(got)
	}
	s.Rules[0].IdentityConditions = []string{"LocalAppPackageId=another-app"}
	got = Explain(s, "tcp", "0.0.0.0:5432", []int{42}, peer)
	if got.Inbound.Rules[0].Relevance != "unverified" || got.Inbound.Assessment != "unknown" {
		t.Fatal("unresolved identity promoted to applicable permission")
	}
	s.Rules[0] = r
	s.Rules[0].LocalPorts = []string{"Any"}
	got = Explain(s, "tcp", "0.0.0.0:5432", []int{42}, peer)
	if got.Inbound.Rules[0].Relevance != "unverified" {
		t.Fatal("broad rule promoted to endpoint-specific")
	}
}
