package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Thuya-Myint/socklens/internal/audit"
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
			code := auditCommand(context.Background(), tc.args, &out, &stderr, lister)
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
	if code := auditCommand(ctx, nil, &out, &stderr, lister); code != 130 || calls != 0 {
		t.Fatal("pre-cancelled audit collected sockets", code, calls)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	if code := auditCommand(ctx, nil, &out, &stderr, func() ([]platform.Socket, error) { cancel(); return auditFixture(), nil }); code != 130 {
		t.Fatal("late result reported as success", code)
	}
	out.Reset()
	stderr.Reset()
	if code := auditCommand(context.Background(), nil, &out, &stderr, func() ([]platform.Socket, error) { return nil, errors.New("permission denied") }); code != 1 || out.Len() != 0 || !strings.Contains(stderr.String(), "permission denied") {
		t.Fatal("discovery failure reported as empty audit", code, &out, &stderr)
	}
	for _, args := range [][]string{nil, {"--json"}} {
		if code := auditCommand(context.Background(), args, brokenWriter{}, io.Discard, lister); code != 1 {
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
			args := []string{"audit", "--port", port, "--protocol", protocol, "--json", "--color", "always"}
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
