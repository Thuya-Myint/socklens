package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Thuya-Myint/socklens/internal/audit"
	"github.com/Thuya-Myint/socklens/internal/firewall"
	"github.com/Thuya-Myint/socklens/internal/platform"
)

func auditFixture() []platform.Socket {
	return []platform.Socket{
		{Protocol: "tcp", Local: "0.0.0.0:5432", State: "LISTEN", PIDs: []int{42}, Processes: []platform.Process{{PID: 42, Name: "untrusted\x1b[31m\nname"}}},
		{Protocol: "tcp", Local: "127.0.0.1:8080", State: "LISTEN", PIDs: []int{43}, Processes: []platform.Process{{PID: 43, Name: "web"}}},
		{Protocol: "udp", Local: "[::]:5353", State: "BOUND", PIDs: []int{42}},
		{Protocol: "tcp", Local: "127.0.0.1:8080", State: "ESTABLISHED", PIDs: []int{43}},
	}
}

func TestAuditFiltersJSONAndErrors(t *testing.T) {
	for _, tc := range []struct {
		args        []string
		code, count int
		want        string
	}{
		{[]string{"--help"}, 0, -1, "socklens audit --port"},
		{[]string{"--json"}, 0, 3, ""},
		{[]string{"--json", "--protocol", "tcp", "--pid", "42", "--port", "5432"}, 0, 1, ""},
		{[]string{"--json", "--port", "9999"}, 0, 0, ""},
		{[]string{"--json", "--protocol", "udp"}, 0, 1, ""},
		{[]string{"--port", "0"}, 1, -1, "port must"},
		{[]string{"--pid", "0"}, 1, -1, "PID must"},
		{[]string{"--protocol", "icmp"}, 1, -1, "protocol must"},
		{[]string{"8080"}, 1, -1, "unexpected arguments"},
		{[]string{"--host", "example.com"}, 1, -1, "flag provided"},
		{[]string{"--listening=false"}, 1, -1, "flag provided"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			calls := 0
			lister := func() ([]platform.Socket, error) { calls++; return auditFixture(), nil }
			var out, stderr bytes.Buffer
			code := auditTestCommand(context.Background(), tc.args, &out, &stderr, lister)
			if code != tc.code {
				t.Fatalf("%d: %s%s", code, &out, &stderr)
			}
			if tc.want != "" && !strings.Contains(out.String()+stderr.String(), tc.want) {
				t.Fatal("missing diagnostic", &out, &stderr)
			}
			if tc.count >= 0 {
				var report audit.Report
				if err := json.Unmarshal(out.Bytes(), &report); err != nil {
					t.Fatal(err, &out)
				}
				if report.SchemaVersion != 1 || report.Scope != "local" || len(report.Listeners) != tc.count || calls != 1 {
					t.Fatalf("%+v calls=%d", report, calls)
				}
				if strings.Contains(out.String(), "\x1b") {
					t.Fatal("terminal escape bytes in JSON")
				}
			} else if calls != 0 {
				t.Fatal("invalid/help command collected sockets")
			}
		})
	}
}

func TestAuditCancellationCollectionAndOutputFailure(t *testing.T) {
	var out, stderr bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	lister := func() ([]platform.Socket, error) { calls++; return auditFixture(), nil }
	if code := auditTestCommand(ctx, nil, &out, &stderr, lister); code != 130 || calls != 0 {
		t.Fatal("pre-cancelled audit collected sockets", code, calls)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	if code := auditTestCommand(ctx, nil, &out, &stderr, func() ([]platform.Socket, error) { cancel(); return auditFixture(), nil }); code != 130 {
		t.Fatal("late result reported as success", code)
	}
	out.Reset()
	stderr.Reset()
	if code := auditTestCommand(context.Background(), nil, &out, &stderr, func() ([]platform.Socket, error) { return nil, errors.New("permission denied") }); code != 1 || out.Len() != 0 || !strings.Contains(stderr.String(), "permission denied") {
		t.Fatal("discovery failure reported as empty audit", code, &out, &stderr)
	}
	for _, args := range [][]string{nil, {"--json"}} {
		if code := auditTestCommand(context.Background(), args, brokenWriter{}, io.Discard, lister); code != 1 {
			t.Fatal("output failure ignored")
		}
	}
}

func TestAuditTextEscapingAndNarrowLayouts(t *testing.T) {
	report, err := audit.Analyze(auditFixture(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{0, 1, 8, 15, 16, 24, 40, 120} {
		var out bytes.Buffer
		if err := writeAuditReport(&out, report, width); err != nil {
			t.Fatal(err)
		}
		text := out.String()
		if strings.Contains(text, "\x1b") || strings.Contains(text, "\r") {
			t.Fatal("untrusted control bytes reached the terminal")
		}
		if width > 0 {
			for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
				if len(line) > width {
					t.Fatalf("width=%d line=%q", width, line)
				}
			}
		}
		if width == 0 && (!strings.Contains(text, `untrusted\x1b[31m\nname`) || !strings.Contains(text, "External reachability: not tested") || !strings.Contains(text, "REVIEW")) {
			t.Fatal("missing evidence or safe process rendering", text)
		}
	}
}

func TestAuditNativeListenerSnapshots(t *testing.T) {
	for _, tc := range []struct{ name, network, address, scope string }{
		{"tcp-loopback", "tcp4", "127.0.0.1:0", "loopback"},
		{"tcp6-loopback", "tcp6", "[::1]:0", "loopback"},
		{"udp-wildcard", "udp4", "0.0.0.0:0", "wildcard"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			protocol := "tcp"
			var address net.Addr
			var tcpListener net.Listener
			if strings.HasPrefix(tc.network, "udp") {
				protocol = "udp"
				connection, err := net.ListenPacket(tc.network, tc.address)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { connection.Close() })
				address = connection.LocalAddr()
			} else {
				listener, err := net.Listen(tc.network, tc.address)
				if err != nil {
					if tc.network == "tcp6" {
						t.Skipf("IPv6 unavailable: %v", err)
					}
					t.Fatal(err)
				}
				t.Cleanup(func() { listener.Close() })
				tcpListener = listener
				address = listener.Addr()
			}
			_, port, err := net.SplitHostPort(address.String())
			if err != nil {
				t.Fatal(err)
			}
			var out, stderr bytes.Buffer
			args := []string{"audit", "--firewall=false", "--port", port, "--protocol", protocol, "--json", "--color", "always"}
			if code := RunContext(context.Background(), args, strings.NewReader(""), &out, &stderr, "test"); code != 0 {
				t.Fatalf("%d %s", code, &stderr)
			}
			if strings.Contains(out.String()+stderr.String(), "\x1b") {
				t.Fatal("JSON diagnostics contain colors")
			}
			var report audit.Report
			if err := json.Unmarshal(out.Bytes(), &report); err != nil {
				t.Fatal(err, &out)
			}
			found := false
			for _, listener := range report.Listeners {
				if listener.Local == address.String() && listener.BindingScope == tc.scope {
					found = true
				}
			}
			if !found {
				t.Fatalf("real listener %s absent from audit: %+v", address, report)
			}
			if protocol == "tcp" {
				connection, err := net.DialTimeout(tc.network, address.String(), time.Second)
				if err != nil {
					t.Fatal("audit left listener unusable", err)
				}
				defer connection.Close()
				accepted, err := tcpListener.Accept()
				if err != nil {
					t.Fatal(err)
				}
				defer accepted.Close()
				rows, err := platform.List()
				if err != nil {
					t.Fatal(err)
				}
				connected := false
				for _, row := range rows {
					if row.Protocol == "tcp" && row.Local == address.String() && row.State == "ESTABLISHED" {
						connected = true
					}
				}
				if !connected {
					t.Fatal("live TCP connection state missing from discovery")
				}
				out.Reset()
				stderr.Reset()
				if code := RunContext(context.Background(), args, strings.NewReader(""), &out, &stderr, "test"); code != 0 {
					t.Fatal(code, &stderr)
				}
				if err := json.Unmarshal(out.Bytes(), &report); err != nil {
					t.Fatal(err)
				}
				if report.Summary.Listeners != 1 {
					t.Fatalf("connected sockets counted as listeners: %+v", report)
				}
			}
			if value, err := strconv.Atoi(port); err != nil || value <= 0 {
				t.Fatal("invalid fixture port")
			}
		})
	}
}

func TestAuditReadingOrderTextAndJSON(t *testing.T) {
	rows := []platform.Socket{
		{Protocol: "tcp", Local: "127.0.0.1:22", State: "LISTEN"},
		{Protocol: "tcp", Local: "10.0.0.1:80", State: "LISTEN"},
		{Protocol: "udp", Local: "[::]:56169", State: "BOUND"},
		{Protocol: "tcp", Local: "0.0.0.0:5432", State: "LISTEN"},
	}
	for _, structured := range []bool{false, true} {
		args := []string{}
		if structured {
			args = append(args, "--json")
		}
		var out, stderr bytes.Buffer
		if code := auditTestCommand(context.Background(), args, &out, &stderr, func() ([]platform.Socket, error) { return rows, nil }); code != 0 {
			t.Fatal(code, stderr.String())
		}
		if structured {
			var report audit.Report
			if err := json.Unmarshal(out.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if len(report.Listeners) != 4 || report.Listeners[0].Local != "0.0.0.0:5432" || report.Listeners[3].Local != "127.0.0.1:22" {
				t.Fatalf("JSON reading order: %+v", report.Listeners)
			}
		} else {
			previous := -1
			for _, heading := range []string{"REVIEW TCP 0.0.0.0:5432", "REVIEW UDP [::]:56169", "INFO TCP 10.0.0.1:80", "INFO TCP 127.0.0.1:22"} {
				position := strings.Index(out.String(), heading)
				if position <= previous {
					t.Fatalf("heading %q out of order: %s", heading, out.String())
				}
				previous = position
			}
			if !strings.Contains(out.String(), "order is not a vulnerability score") {
				t.Fatal("reading order presented without its limitation")
			}
		}
	}
}

func auditTestCommand(ctx context.Context, args []string, out, errOut io.Writer, list func() ([]platform.Socket, error)) int {
	return auditWithFirewall(ctx, args, out, errOut, list, func(context.Context, []platform.Socket) firewall.Snapshot { return firewall.Empty() })
}

func TestAuditFirewallOptionsAndCoverage(t *testing.T) {
	for _, tc := range []struct {
		args        []string
		code, calls int
	}{
		{[]string{"--json", "--peer", "192.168.1.20"}, 0, 1},
		{[]string{"--firewall=false", "--json"}, 0, 0},
		{[]string{"--peer", "example.com"}, 1, 0},
		{[]string{"--peer", ""}, 1, 0},
		{[]string{"--peer", "192.168.1.20", "--firewall=false"}, 1, 0},
		{[]string{"--firewall-timeout", "0s"}, 1, 0},
		{[]string{"--firewall-timeout", "3m"}, 1, 0},
		{[]string{"--rule-limit", "-1"}, 1, 0},
		{[]string{"--help"}, 0, 0},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			calls := 0
			var out, stderr bytes.Buffer
			collector := func(ctx context.Context, rows []platform.Socket) firewall.Snapshot {
				calls++
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("unbounded policy query")
				}
				s := firewall.Empty()
				s.Coverage = "partial"
				s.Sources = append(s.Sources, firewall.Source{Name: "fixture", Status: "read"})
				s.Rules = append(s.Rules, firewall.Rule{ID: "in", Source: "fixture", Direction: "inbound", Action: "allow", Family: "ipv4", Protocols: []string{"tcp"}, LocalPorts: []string{"5432"}, RemoteAddresses: []string{"192.168.1.0/24"}})
				return s
			}
			code := auditWithFirewall(context.Background(), tc.args, &out, &stderr, func() ([]platform.Socket, error) { return auditFixture(), nil }, collector)
			if code != tc.code || calls != tc.calls {
				t.Fatalf("code=%d calls=%d: %s%s", code, calls, &out, &stderr)
			}
			if code == 0 && strings.Contains(strings.Join(tc.args, " "), "--json") {
				var report audit.Report
				if err := json.Unmarshal(out.Bytes(), &report); err != nil {
					t.Fatal(err)
				}
				if calls == 0 && report.Firewall != nil {
					t.Fatal("disabled policy collector still included")
				}
				if calls > 0 && (report.Firewall == nil || report.Listeners[0].Firewall == nil || len(report.Listeners[0].Firewall.Inbound.Rules) != 1) {
					t.Fatalf("missing context: %+v", report)
				}
			}
		})
	}
}
func TestAuditPolicyTimeoutAndCancellation(t *testing.T) {
	var out, stderr bytes.Buffer
	collector := func(ctx context.Context, _ []platform.Socket) firewall.Snapshot {
		<-ctx.Done()
		return firewall.Empty()
	}
	code := auditWithFirewall(context.Background(), []string{"--json", "--firewall-timeout", "1s"}, &out, &stderr, func() ([]platform.Socket, error) { return auditFixture(), nil }, collector)
	var report audit.Report
	if code != 0 || json.Unmarshal(out.Bytes(), &report) != nil || report.Firewall == nil || report.Firewall.Coverage != "unknown" {
		t.Fatal("policy timeout erased socket report or implied coverage", code, &out, &stderr)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out.Reset()
	stderr.Reset()
	code = auditWithFirewall(ctx, nil, &out, &stderr, func() ([]platform.Socket, error) { return auditFixture(), nil }, func(context.Context, []platform.Socket) firewall.Snapshot { cancel(); return firewall.Empty() })
	if code != 130 || out.Len() != 0 {
		t.Fatal("late policy report after cancellation", code, &out, &stderr)
	}
}
func TestAuditPolicyRenderingLimitsAndEscaping(t *testing.T) {
	report, err := audit.Analyze(auditFixture(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := firewall.Empty()
	s.Coverage = "partial"
	for _, id := range []string{"a", "b", "c", "d"} {
		s.Rules = append(s.Rules, firewall.Rule{ID: id, Name: "rule" + id + "\x1b[31m", Direction: "inbound", Action: "allow", Source: "fixture"})
	}
	report.Firewall = &s
	c := firewall.Explain(s, "tcp", report.Listeners[0].Local, nil, netip.Addr{})
	report.Listeners[0].Firewall = &c
	for _, width := range []int{0, 1, 16, 24, 80} {
		var out bytes.Buffer
		if err := writeAuditReportLimited(&out, report, width, 3); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "\x1b") {
			t.Fatal("unsafe policy text")
		}
		if width > 0 {
			for _, line := range strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n") {
				if len(line) > width {
					t.Fatalf("width=%d line=%q", width, line)
				}
			}
		} else if !strings.Contains(out.String(), "1 additional inbound candidates") {
			t.Fatal("omitted rules not explained")
		}
	}
	var out bytes.Buffer
	if err := writeAuditReportLimited(&out, report, 0, 0); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ruled") || strings.Contains(out.String(), "additional inbound") {
		t.Fatal("all-rule mode omitted records")
	}
}

func TestAuditConciseAndDetailedViews(t *testing.T) {
	report, err := audit.Analyze(auditFixture(), time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	policy := firewall.Empty()
	policy.Coverage = "partial"
	policy.Sources = []firewall.Source{{Name: "windows-policy-com", Status: "read", Detail: "FULL_SOURCE_METADATA"}}
	policy.Profiles = []firewall.Profile{{Name: "Private", Enabled: "True", Inbound: "block", Outbound: "allow", Detail: "FULL_PROFILE_METADATA"}}
	policy.Interfaces = []firewall.Interface{{Name: "ethernet", Addresses: []string{"192.168.1.2/24"}}}
	policy.Notes = []string{"FULL_COLLECTION_NOTE"}
	policy.Rules = []firewall.Rule{{ID: "RULE_IDENTIFIER", Name: "Example database rule", Source: "windows-policy-com", Direction: "inbound", Action: "allow", Family: "ipv4", Protocols: []string{"6"}, LocalPorts: []string{"5432"}, RemotePorts: []string{"Any"}, LocalAddresses: []string{"Any"}, RemoteAddresses: []string{"192.168.1.0/24"}, Program: `C:\Apps\database\server.exe`, Profiles: "Private", Conditions: []string{"FULL_SECURITY_RESTRICTION"}, Raw: "FULL_RAW_POLICY"}}
	report.Firewall = &policy
	// No paths: keep the rule conditional instead of guessing process identity.
	for i := range report.Listeners {
		l := &report.Listeners[i]
		context := firewall.Explain(policy, l.Protocol, l.Local, l.PIDs, netip.MustParseAddr("192.168.1.20"))
		l.Firewall = &context
	}
	var compact, details bytes.Buffer
	if err := writeAuditReportMode(&compact, report, 0, 3, false); err != nil {
		t.Fatal(err)
	}
	if err := writeAuditReportMode(&details, report, 0, 3, true); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"REVIEW TCP 0.0.0.0:5432", "untrusted\\x1b[31m\\nname", "coverage", "decisions unknown", "[conditional]", "local port=5432", "remote=192.168.1.0/24", "server.exe (path in --details)", "--details", "No candidates does not mean allowed or blocked"} {
		if !strings.Contains(compact.String(), required) {
			t.Fatalf("concise view lost %q: %s", required, compact.String())
		}
	}
	for _, fullOnly := range []string{"FULL_SOURCE_METADATA", "FULL_PROFILE_METADATA", "FULL_COLLECTION_NOTE", "FULL_SECURITY_RESTRICTION", "FULL_RAW_POLICY", "RULE_IDENTIFIER"} {
		if strings.Contains(compact.String(), fullOnly) || !strings.Contains(details.String(), fullOnly) {
			t.Fatalf("detail visibility incorrect for %q", fullOnly)
		}
	}
	if compact.Len() >= details.Len() {
		t.Fatal("concise view is not shorter than the detailed report")
	}
	for _, mode := range []bool{false, true} {
		if err := writeAuditReportMode(brokenWriter{}, report, 80, 3, mode); err == nil {
			t.Fatal("output error ignored")
		}
	}
}

func TestAuditDetailsDoesNotChangeJSON(t *testing.T) {
	var first audit.Report
	for _, details := range []bool{false, true} {
		args := []string{"--json"}
		if details {
			args = append(args, "--details")
		}
		var out, stderr bytes.Buffer
		if code := auditTestCommand(context.Background(), args, &out, &stderr, func() ([]platform.Socket, error) { return auditFixture(), nil }); code != 0 {
			t.Fatal(code, &stderr)
		}
		var report audit.Report
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		report.Timestamp = time.Time{}
		if !details {
			first = report
		} else {
			a, _ := json.Marshal(first)
			b, _ := json.Marshal(report)
			if string(a) != string(b) {
				t.Fatal("presentation flag changed structured data")
			}
		}
	}
}

func TestAuditCompactRuleValuesAndPaths(t *testing.T) {
	if got := auditRuleValues(nil, false); got != "unresolved" {
		t.Fatal("unknown restriction presented as any", got)
	}
	if got := auditRuleValues([]string{"6", "17", "256"}, true); got != "TCP,UDP,any" {
		t.Fatal(got)
	}
	if got := auditProgramName(`/Applications/Example.app/Contents/MacOS/example`); got != "example (path in --details)" {
		t.Fatal(got)
	}
	if got := auditProgramName(`C:\Apps\example.exe`); got != "example.exe (path in --details)" {
		t.Fatal(got)
	}
}

func TestAuditBothViewsWrapUntrustedPolicyText(t *testing.T) {
	report, err := audit.Analyze(auditFixture(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := firewall.Empty()
	s.Coverage = "partial"
	s.Sources = []firewall.Source{{Name: "source\x1b[31m", Status: "unavailable", Detail: "failed\nquery"}}
	s.Profiles = []firewall.Profile{{Name: "profile\rname", Enabled: "see detail", Detail: "setting\x1b[31m"}}
	s.Rules = []firewall.Rule{{ID: "id", Name: "rule\x1b[31m\nname", Direction: "both", Action: "allow", Source: "fixture", Conditions: []string{"condition\tvalue"}, Raw: "raw\x1b[31m\npolicy"}}
	report.Firewall = &s
	policy := firewall.Explain(s, "tcp", report.Listeners[0].Local, nil, netip.Addr{})
	report.Listeners[0].Firewall = &policy
	for _, details := range []bool{false, true} {
		for _, width := range []int{0, 1, 8, 15, 16, 24, 80} {
			var out bytes.Buffer
			if err := writeAuditReportMode(&out, report, width, 3, details); err != nil {
				t.Fatal(err)
			}
			if strings.ContainsAny(out.String(), "\x1b\r\t") {
				t.Fatalf("untrusted control bytes reached view details=%v", details)
			}
			if width > 0 {
				for _, line := range strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n") {
					if len(line) > width {
						t.Fatalf("details=%v width=%d line=%q", details, width, line)
					}
				}
			}
		}
	}
}
