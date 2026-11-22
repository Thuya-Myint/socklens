package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/Thuya-Myint/socklens/internal/audit"
	"github.com/Thuya-Myint/socklens/internal/firewall"
	"github.com/Thuya-Myint/socklens/internal/platform"
)

func TestAuditActionableCases(t *testing.T) {
	for _, tc := range []struct {
		name, local, protocol, state           string
		owned, policy                          bool
		wantExposure, wantUnknown, wantCommand string
	}{
		{"loopback", "127.0.0.1:8080", "tcp", "LISTEN", true, false, "Loopback-bound; direct socket access is local", "firewall not collected", "socklens inspect --protocol tcp 8080"},
		{"wildcard", "0.0.0.0:5432", "tcp", "LISTEN", true, false, "All interfaces in this address family; remote reachability is not established", "firewall not collected", "socklens inspect --protocol tcp 5432"},
		{"missing-owner", "[::1]:5353", "udp", "BOUND", false, false, "Loopback-bound", "owner metadata incomplete", "socklens inspect --protocol udp 5353"},
		{"partial-firewall", "0.0.0.0:8443", "tcp", "LISTEN", true, true, "remote reachability is not established", "traffic decision not established (partial firewall coverage)", "socklens audit --protocol tcp --port 8443 --details"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := platform.Socket{Protocol: tc.protocol, Local: tc.local, State: tc.state, PIDs: []int{42}}
			if tc.owned {
				row.Processes = []platform.Process{{PID: 42, Name: "example"}}
			}
			report, err := audit.Analyze([]platform.Socket{row}, time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			if tc.policy {
				snapshot := firewall.Empty()
				snapshot.Coverage = "partial"
				snapshot.Sources = []firewall.Source{{Name: "fixture", Status: "unavailable", Detail: "permission denied"}}
				snapshot.Rules = []firewall.Rule{{Name: "CONDITIONAL_ALLOW", Direction: "inbound", Action: "allow", Protocols: []string{"tcp"}, LocalPorts: []string{"8443"}}}
				report.Firewall = &snapshot
				policy := firewall.Explain(snapshot, tc.protocol, tc.local, row.PIDs, netip.Addr{})
				report.Listeners[0].Firewall = &policy
			}
			before, _ := json.Marshal(report)
			var out bytes.Buffer
			if err := writeAuditReportMode(&out, report, 0, 3, false); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{tc.wantExposure, tc.wantUnknown, "Check: " + tc.wantCommand, "External reachability: not tested"} {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("missing %q: %s", want, &out)
				}
			}
			if strings.Count(out.String(), "  Check:") != 1 {
				t.Fatalf("expected one focused check: %s", &out)
			}
			if !tc.owned && (!strings.Contains(out.String(), "Identify the owner first") || strings.Contains(out.String(), "REVIEW UDP")) {
				t.Fatalf("missing owner was hidden or treated as severity: %s", &out)
			}
			after, _ := json.Marshal(report)
			if !bytes.Equal(before, after) {
				t.Fatal("presentation changed structured evidence")
			}
			out.Reset()
			if err := writeAuditReportMode(&out, report, 0, 3, true); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), "  Exposure:") || strings.Contains(out.String(), "  Check:") {
				t.Fatal("compact-only additions changed detailed output")
			}
		})
	}
}

func TestAuditCheckCommandsArePassiveAndPortScoped(t *testing.T) {
	for _, policyPresent := range []bool{false, true} {
		listener := audit.Listener{Protocol: "tcp", Local: "[fe80::1%15]:8080", ProcessVisibility: "observed", Processes: []audit.Process{{PID: 42, Name: "$(do-not-run)"}}}
		if policyPresent {
			listener.Firewall = &firewall.Context{Coverage: "partial", Peer: "192.0.2.1"}
		}
		_, command := auditFirstCheck(listener)
		if strings.ContainsAny(command, "%$;[]") || strings.Contains(command, "192.0.2.1") {
			t.Fatalf("untrusted metadata entered command: %q", command)
		}
		parts := strings.Fields(command)
		var out, stderr bytes.Buffer
		calls := 0
		list := func() ([]platform.Socket, error) {
			calls++
			return []platform.Socket{{Protocol: "tcp", Local: listener.Local, State: "LISTEN"}}, nil
		}
		code := 0
		if parts[1] == "inspect" {
			code = socketsWithSource("inspect", parts[2:], &out, &stderr, list)
		} else {
			code = auditWithMetadata(context.Background(), parts[2:], &out, &stderr, list, func(context.Context, []platform.Socket) firewall.Snapshot { return firewall.Empty() }, func(context.Context, []platform.Socket) *platform.ServiceSnapshot { return nil })
		}
		if code != 0 || calls != 1 {
			t.Fatalf("generated command rejected: %s code=%d calls=%d stderr=%s", command, code, calls, &stderr)
		}
	}
	for _, listener := range []audit.Listener{{Protocol: "tcp;command", Local: "127.0.0.1:80"}, {Protocol: "tcp", Local: "bad"}, {Protocol: "tcp", Local: "127.0.0.1:0"}} {
		if _, command := auditFirstCheck(listener); command != "" {
			t.Fatalf("invalid observation produced command: %q", command)
		}
	}
}

func TestAuditActionsUnknownCoverageAndOwners(t *testing.T) {
	listener := audit.Listener{Protocol: "udp", Local: "127.0.0.1:5353", ProcessVisibility: "partial", Firewall: &firewall.Context{}}
	if got := auditUnknownSummary(listener); !strings.Contains(got, "owner metadata incomplete") || !strings.Contains(got, "unknown firewall coverage") {
		t.Fatal(got)
	}
	_, command := auditFirstCheck(listener)
	if command != "socklens inspect --protocol udp 5353" {
		t.Fatal("missing owner should be investigated before policy", command)
	}
	listener.ProcessVisibility = "observed"
	listener.Firewall.Coverage = "complete"
	if got := auditUnknownSummary(listener); !strings.Contains(got, "traffic decision not established") {
		t.Fatal("coverage became a traffic verdict", got)
	}
	for _, scope := range []string{"private", "link-local", "multicast", "specific-address"} {
		listener.BindingScope = scope
		if auditExposureSummary(listener) == "" {
			t.Fatal("missing exposure explanation", scope)
		}
	}
}
