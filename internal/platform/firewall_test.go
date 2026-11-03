package platform

import (
	"context"
	"encoding/json"
	"github.com/Thuya-Myint/socklens/internal/firewall"
	"strings"
	"testing"
	"time"
)

func TestNFTPolicyParsing(t *testing.T) {
	data := []byte(`{"nftables":[{"chain":{"family":"inet","table":"filter","name":"input","hook":"input","policy":"drop"}},{"chain":{"family":"inet","table":"filter","name":"output","hook":"output","policy":"accept"}},{"rule":{"family":"inet","table":"filter","chain":"input","handle":1,"expr":[{"match":{"op":"==","left":{"payload":{"protocol":"tcp","field":"dport"}},"right":5432}},{"match":{"op":"==","left":{"payload":{"protocol":"ip","field":"saddr"}},"right":{"prefix":{"addr":"192.168.1.0","len":24}}}},{"accept":null}]}},{"rule":{"family":"inet","table":"filter","chain":"output","handle":2,"expr":[{"match":{"op":"==","left":{"payload":{"protocol":"udp","field":"dport"}},"right":53}},{"drop":null}]}},{"rule":{"family":"inet","table":"filter","chain":"custom","handle":3,"expr":[{"jump":{"target":"other"}}]}}]}`)
	s := firewall.Empty()
	if err := decodeNFT(data, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Rules) != 3 || len(s.Profiles) != 2 || s.Profiles[0].Inbound != "drop" || s.Profiles[1].Outbound != "accept" {
		t.Fatal(s)
	}
	a, b, c := s.Rules[0], s.Rules[1], s.Rules[2]
	if a.Direction != "inbound" || a.LocalPorts[0] != "5432" || a.RemoteAddresses[0] != "192.168.1.0/24" || a.Action != "allow" {
		t.Fatal(a)
	}
	if b.Direction != "outbound" || b.RemotePorts[0] != "53" || len(b.LocalPorts) != 0 || b.Action != "block" {
		t.Fatal(b)
	}
	if c.Direction != "unknown" || c.Action != "unknown" || c.Raw == "" {
		t.Fatal("jump/custom-chain uncertainty lost", c)
	}
	for _, bad := range []string{`{}`, `{"nftables":[{"rule":true}]}`, `broken`} {
		s := firewall.Empty()
		if decodeNFT([]byte(bad), &s) == nil {
			t.Fatal("bad ruleset accepted")
		}
	}
}
func TestTextPolicyParsing(t *testing.T) {
	s := firewall.Empty()
	decodeIPTables([]byte("*filter\n:INPUT DROP [0:0]\n-A INPUT -s 10.0.0.0/8 -p tcp --dport 5432 -j ACCEPT\n-A OUTPUT -p udp --dport 53 -j DROP\n-A INPUT ! -s 10.0.0.0/8 -p tcp --dport 22 -j DROP\nCOMMIT\n"), "iptables-save", &s)
	if len(s.Rules) != 3 || s.Rules[0].LocalPorts[0] != "5432" || s.Rules[1].RemotePorts[0] != "53" || len(s.Rules[2].LocalPorts) != 0 {
		t.Fatal(s)
	}
	s = firewall.Empty()
	decodePF([]byte("pass in proto tcp from any to any port = 5432 label \"out\"\nblock out all\nanchor \"nested\" all\n"), &s)
	if len(s.Rules) != 3 || s.Rules[0].Direction != "inbound" || s.Rules[1].Action != "block" || s.Rules[2].Action != "unknown" {
		t.Fatal(s)
	}
	s = firewall.Empty()
	decodeALF([]byte("ALF: total number of apps = 2\n1 : /Applications/Example.app\n ( Allow incoming connections )\n2 : /usr/bin/example\n ( Block incoming connections )\n"), &s)
	if len(s.Rules) != 2 || s.Rules[0].Program != "/Applications/Example.app" || s.Rules[1].Action != "block" {
		t.Fatal(s)
	}
}
func TestWindowsPolicyData(t *testing.T) {
	s := firewall.Empty()
	data := []byte(`{"profiles":[],"connections":[],"rules":[{"id":"a","source":"windows-active-store","direction":"inbound","action":"allow","protocols":["TCP"],"local_ports":["80,443"],"conditions":["Private profile"]}],"notes":[]}`)
	if err := decodeWindowsFirewall(data, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Rules) != 1 || len(s.Rules[0].LocalPorts) != 2 {
		t.Fatal(s)
	}
	if decodeWindowsFirewall([]byte(`{}`), &s) == nil {
		t.Fatal("incomplete output accepted")
	}
	// Query scripts must never alter firewall policy or bypass execution policy.
	for _, forbidden := range []string{"Set-NetFirewall", "New-NetFirewall", "Remove-NetFirewall", "ExecutionPolicy", "Start-Process"} {
		if strings.Contains(windowsFirewallScript+windowsCOMFirewallScript, forbidden) {
			t.Fatal(forbidden)
		}
	}
}
func TestFirewallCollectionNative(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s := CollectFirewall(ctx, nil)
	if s.Coverage != "partial" && s.Coverage != "unknown" {
		t.Fatalf("unjustified coverage: %+v", s)
	}
	if len(s.Sources) == 0 {
		t.Fatal("collector gave no source evidence")
	}
	encoded, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"Paths"`) {
		t.Fatal("internal process path map leaked")
	}
}

func TestPolicyOutputBound(t *testing.T) {
	var buffer boundedPolicyBuffer
	if _, err := buffer.Write(make([]byte, maxPolicyOutput)); err != nil {
		t.Fatal(err)
	}
	if _, err := buffer.Write([]byte("x")); err == nil || !buffer.overflow {
		t.Fatal("oversized policy output accepted")
	}
}
