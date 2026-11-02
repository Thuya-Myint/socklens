package audit

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

var snapshotTime = time.Date(2026, 11, 2, 3, 0, 0, 0, time.UTC)

func TestAddressScopes(t *testing.T) {
	for _, tc := range []struct{ local, scope, family string }{
		{"127.0.0.1:8080", "loopback", "ipv4"}, {"127.9.2.1:8080", "loopback", "ipv4"},
		{"[::1]:8080", "loopback", "ipv6"}, {"[::ffff:127.0.0.1]:8080", "loopback", "ipv4-mapped-ipv6"},
		{"0.0.0.0:8080", "wildcard", "ipv4"}, {"[::]:8080", "wildcard", "ipv6"},
		{"[::ffff:0.0.0.0]:8080", "wildcard", "ipv4-mapped-ipv6"},
		{"10.1.2.3:8080", "private", "ipv4"}, {"172.16.0.1:8080", "private", "ipv4"}, {"172.31.255.254:8080", "private", "ipv4"},
		{"192.168.1.2:8080", "private", "ipv4"}, {"[fd01::2]:8080", "private", "ipv6"},
		{"169.254.1.2:8080", "link-local", "ipv4"}, {"[fe80::1%eth0]:8080", "link-local", "ipv6"},
		{"224.0.0.1:8080", "multicast", "ipv4"}, {"[ff02::1%eth0]:8080", "multicast", "ipv6"},
		{"172.32.0.1:8080", "specific-address", "ipv4"}, {"203.0.113.1:8080", "specific-address", "ipv4"},
	} {
		t.Run(tc.local, func(t *testing.T) {
			report, err := Analyze([]platform.Socket{{Protocol: "tcp", Local: tc.local, State: "LISTEN"}}, snapshotTime)
			if err != nil || len(report.Listeners) != 1 {
				t.Fatalf("%+v %v", report, err)
			}
			got := report.Listeners[0]
			if got.BindingScope != tc.scope || got.AddressFamily != tc.family {
				t.Fatalf("%+v", got)
			}
			if report.ExternalReachability != "not-tested" || got.Findings[0].Confidence != "observed" {
				t.Fatal("scope observation presented without uncertainty")
			}
			if tc.local == "[::]:8080" && !strings.Contains(strings.Join(got.Findings[0].Evidence, " "), "IPv4 acceptance by this IPv6 socket was not tested") {
				t.Fatal("IPv6 wildcard implies IPv4 reachability")
			}
			if tc.local == "[::ffff:0.0.0.0]:8080" && !strings.Contains(got.Findings[0].Title, "IPv4") {
				t.Fatal("mapped IPv4 wildcard labeled IPv6")
			}
		})
	}
}

func TestListenerRulesAndSummary(t *testing.T) {
	rows := []platform.Socket{
		{Protocol: "tcp", Local: "0.0.0.0:5432", State: "LISTEN", PIDs: []int{2}, Processes: []platform.Process{{PID: 2, Name: "example"}}},
		{Protocol: "tcp", Local: "127.0.0.1:5432", State: "LISTEN", PIDs: []int{3}},
		{Protocol: "tcp", Local: "10.0.0.1:3306", State: "LISTEN"},
		{Protocol: "udp", Local: "[::]:5432", State: "UNCONN"},
		{Protocol: "udp", Local: "10.0.0.2:161", State: "BOUND"},
		{Protocol: "tcp", Local: "10.0.0.3:8080", State: "LISTEN", PIDs: []int{8}, Processes: []platform.Process{{PID: 8, Name: "server"}}},
		{Protocol: "tcp", Local: "invalid", State: "ESTABLISHED"},
		{Protocol: "udp", Local: "127.0.0.1:6379", State: "CONNECTED"},
	}
	report, err := Analyze(rows, snapshotTime)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary != (Summary{Listeners: 6, ReviewListeners: 4, InformationListeners: 2, Findings: 9, MissingProcessDetails: 4}) {
		t.Fatalf("summary: %+v", report.Summary)
	}
	byLocal := map[string]Listener{}
	for _, listener := range report.Listeners {
		byLocal[listener.Local] = listener
	}
	postgres := byLocal["0.0.0.0:5432"]
	if len(postgres.Findings) != 2 || postgres.ServiceHint.Name != "PostgreSQL" || postgres.ServiceHint.Verified || postgres.Findings[1].Confidence != "heuristic" {
		t.Fatalf("service hint misrepresented: %+v", postgres)
	}
	if len(byLocal["127.0.0.1:5432"].Findings) != 1 || byLocal["127.0.0.1:5432"].Findings[0].Level != "information" {
		t.Fatal("local-only service hint treated as non-loopback")
	}
	if byLocal["[::]:5432"].ServiceHint != nil {
		t.Fatal("TCP hint applied to arbitrary UDP port")
	}
	if byLocal["10.0.0.2:161"].ServiceHint.Name != "SNMP" {
		t.Fatal("UDP hint omitted")
	}
	if byLocal["0.0.0.0:5432"].ProcessVisibility != "observed" || byLocal["127.0.0.1:5432"].ProcessVisibility != "partial" || byLocal["10.0.0.1:3306"].ProcessVisibility != "unknown" {
		t.Fatal("missing process details mislabeled")
	}
}

func TestAnalyzeDeterminismAndSnapshotOwnership(t *testing.T) {
	rows := []platform.Socket{{Protocol: "tcp", Local: "127.0.0.1:80", State: "LISTEN", PIDs: []int{5, 2, 5}, Processes: []platform.Process{{PID: 5, Name: "z"}, {PID: 2, Name: "a"}}}, {Protocol: "udp", Local: "0.0.0.0:53", State: "BOUND"}}
	original, _ := json.Marshal(rows)
	first, err := Analyze(rows, snapshotTime)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Analyze([]platform.Socket{rows[1], rows[0]}, snapshotTime.In(time.FixedZone("offset", 3600)))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatal("input order or time zone changes the report")
	}
	var owned *Listener
	for i := range first.Listeners {
		if first.Listeners[i].Local == "127.0.0.1:80" {
			owned = &first.Listeners[i]
		}
	}
	if owned == nil || !reflect.DeepEqual(owned.PIDs, []int{2, 5}) {
		t.Fatal("owners not normalized")
	}
	owned.PIDs[0] = 99
	owned.Processes[0].Name = "changed"
	after, _ := json.Marshal(rows)
	if string(original) != string(after) {
		t.Fatal("analysis/report mutation changed the caller's snapshot")
	}
}

func TestInvalidListenerAndEmptyJSON(t *testing.T) {
	for _, local := range []string{"bad", "localhost:8080", "127.0.0.1:99999"} {
		if _, err := Analyze([]platform.Socket{{Protocol: "tcp", Local: local, State: "LISTEN"}}, snapshotTime); err == nil {
			t.Fatal("invalid numeric endpoint accepted", local)
		}
	}
	report, err := Analyze(nil, snapshotTime)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"listeners":[]`) || report.Summary.Listeners != 0 || report.Scope != "local" {
		t.Fatal("empty report is ambiguous", string(encoded))
	}
}

func TestReadingOrder(t *testing.T) {
	// Intentionally mixed protocols, string/numeric port order, binding scopes,
	// heuristic hints, and process visibility. Missing metadata is not severity.
	rows := []platform.Socket{
		{Protocol: "tcp", Local: "127.0.0.1:22", State: "LISTEN"},
		{Protocol: "tcp", Local: "10.0.0.10:80", State: "LISTEN"},
		{Protocol: "udp", Local: "[::]:56169", State: "BOUND"},
		{Protocol: "tcp", Local: "10.0.0.1:22", State: "LISTEN"},
		{Protocol: "tcp", Local: "0.0.0.0:5432", State: "LISTEN"},
		{Protocol: "udp", Local: "0.0.0.0:161", State: "BOUND"},
		{Protocol: "tcp", Local: "0.0.0.0:443", State: "LISTEN"},
		{Protocol: "tcp", Local: "0.0.0.0:80", State: "LISTEN"},
		{Protocol: "udp", Local: "0.0.0.0:80", State: "BOUND"},
		{Protocol: "tcp", Local: "10.0.0.2:80", State: "LISTEN"},
		{Protocol: "udp", Local: "127.0.0.1:9", State: "BOUND"},
	}
	want := []string{
		"udp 0.0.0.0:161", "tcp 0.0.0.0:5432", "tcp 10.0.0.1:22",
		"tcp 0.0.0.0:80", "udp 0.0.0.0:80", "tcp 0.0.0.0:443", "udp [::]:56169",
		"tcp 10.0.0.2:80", "tcp 10.0.0.10:80",
		"udp 127.0.0.1:9", "tcp 127.0.0.1:22",
	}
	for rotation := range rows {
		input := append(append([]platform.Socket{}, rows[rotation:]...), rows[:rotation]...)
		report, err := Analyze(input, snapshotTime)
		if err != nil {
			t.Fatal(err)
		}
		got := []string{}
		for _, listener := range report.Listeners {
			got = append(got, listener.Protocol+" "+listener.Local)
			informationSeen := false
			for _, finding := range listener.Findings {
				if finding.Level == "information" {
					informationSeen = true
				} else if informationSeen {
					t.Fatalf("review after information: %+v", listener)
				}
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("rotation %d: got %v; want %v", rotation, got, want)
		}
	}
}
