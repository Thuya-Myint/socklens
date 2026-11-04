package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Thuya-Myint/socklens/internal/probe"
	"github.com/Thuya-Myint/socklens/internal/terminal"
)

type probeConnection struct {
	net.Conn
	endpoint string
}

func (c probeConnection) Close() error { return nil }
func (c probeConnection) LocalAddr() net.Addr {
	return net.TCPAddrFromAddrPort(netip.MustParseAddrPort("127.0.0.1:30000"))
}
func (c probeConnection) RemoteAddr() net.Addr {
	return net.TCPAddrFromAddrPort(netip.MustParseAddrPort(c.endpoint))
}
func TestProbeOptionsValidateBeforeNetwork(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"--help"}, 0, "socklens probe --host"},
		{nil, 1, "--ports requires"},
		{[]string{"--ports", "80"}, 1, "--host is required"},
		{[]string{"--host", "https://example.com", "--ports", "80"}, 1, "without URL"},
		{[]string{"--host", "127.0.0.1", "--ports", "0"}, 1, "port must"},
		{[]string{"--host", "127.0.0.1", "--ports", "80-90"}, 1, "ranges"},
		{[]string{"--host", "127.0.0.1", "--ports", "80", "--protocol", "udp"}, 1, "flag provided"},
		{[]string{"--host", "127.0.0.1", "--ports", "80", "--family", "ipv6"}, 1, "does not match"},
		{[]string{"--host", "127.0.0.1", "--ports", "80", "--timeout", "0s"}, 1, "timeout must"},
		{[]string{"--host", "127.0.0.1", "--ports", "80", "--connect-timeout", "99ms"}, 1, "connect-timeout must"},
		{[]string{"--host", "127.0.0.1", "--ports", "80", "--concurrency", "33"}, 1, "concurrency must"},
		{[]string{"127.0.0.1", "--ports", "80"}, 1, "no positional"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			calls := 0
			engine := probe.Engine{LookupIP: func(context.Context, string) ([]net.IPAddr, error) {
				calls++
				return nil, errors.New("unexpected lookup")
			}, Dial: func(context.Context, string, string) (net.Conn, error) {
				calls++
				return nil, errors.New("unexpected dial")
			}}
			var out, stderr bytes.Buffer
			code := probeCommand(context.Background(), tc.args, &out, &stderr, engine)
			if code != tc.code || calls != 0 || !strings.Contains(out.String()+stderr.String(), tc.want) {
				t.Fatal(code, calls, &out, &stderr)
			}
		})
	}
}
func TestProbeJSONAndExitCodes(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
	}{{"connected", 0}, {"refused", 2}, {"resolution-error", 1}, {"interrupted", 130}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			engine := probe.Engine{LookupIP: func(context.Context, string) ([]net.IPAddr, error) {
				if tc.name == "resolution-error" {
					return nil, errors.New("no DNS answer")
				}
				return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
			}, Dial: func(_ context.Context, _, endpoint string) (net.Conn, error) {
				switch tc.name {
				case "refused":
					return nil, syscall.ECONNREFUSED
				case "interrupted":
					cancel()
					return nil, context.Canceled
				}
				return probeConnection{endpoint: endpoint}, nil
			}}
			var out, stderr bytes.Buffer
			code := probeCommand(ctx, []string{"--host", "example.test", "--ports", "8080", "--json"}, terminal.Wrap(&out, "always"), &stderr, engine)
			var report probe.Report
			if code != tc.code || json.Unmarshal(out.Bytes(), &report) != nil || report.SchemaVersion != 1 || strings.Contains(out.String()+stderr.String(), "\x1b") {
				t.Fatal(code, &out, &stderr)
			}
			if report.Status != tc.name && tc.name != "connected" && tc.name != "refused" {
				t.Fatal(report.Status)
			}
			if strings.Contains(out.String(), "INFO Checking") {
				t.Fatal("progress text polluted JSON")
			}
		})
	}
}
func TestProbePresentationAndOutputErrors(t *testing.T) {
	report := probe.Report{Timestamp: time.Now().UTC(), Host: "host\x1b[31m\nname", Status: "completed", Addresses: []probe.Address{{IP: "::1", Family: "ipv6"}}, Error: &probe.Failure{Message: "failure\tmessage"}}
	for _, status := range []string{"connected", "refused", "timed-out", "network-error", "cancelled", "not-attempted"} {
		report.Results = append(report.Results, probe.Result{Endpoint: "[::1]:8080", Status: status, Attempted: status != "not-attempted", Message: "reason\x1b[31m\rtext", LocalEndpoint: "[::1]:30000", DurationMS: .25})
	}
	for _, width := range []int{0, 1, 8, 15, 16, 24, 80, 120} {
		var out bytes.Buffer
		if err := writeProbeReport(&out, report, width); err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(out.String(), "\x1b\t\r") {
			t.Fatal("untrusted control bytes reached report")
		}
		if width > 0 {
			for _, line := range strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n") {
				if len(line) > width {
					t.Fatalf("width=%d line=%q", width, line)
				}
			}
		}
	}
	var plain, colored bytes.Buffer
	if err := writeProbeReport(&plain, report, 80); err != nil {
		t.Fatal(err)
	}
	if err := writeProbeReport(terminal.Wrap(&colored, "always"), report, 80); err != nil {
		t.Fatal(err)
	}
	if ansi.ReplaceAllString(colored.String(), "") != plain.String() {
		t.Fatal("color changed report content")
	}
	if err := writeProbeReport(brokenWriter{}, report, 80); err == nil {
		t.Fatal("report output error ignored")
	}
	calls := 0
	engine := probe.Engine{Dial: func(context.Context, string, string) (net.Conn, error) { calls++; return nil, syscall.ECONNREFUSED }}
	if code := probeCommand(context.Background(), []string{"--host", "127.0.0.1", "--ports", "8080"}, brokenWriter{}, &plain, engine); code != 1 || calls != 0 {
		t.Fatal("network work started after initial output failure", code, calls)
	}
	if code := probeCommand(context.Background(), []string{"--host", "127.0.0.1", "--ports", "8080", "--json"}, brokenWriter{}, &plain, engine); code != 1 {
		t.Fatal("JSON output failure ignored", code)
	}
}
func TestProbeNativeCLIJSONAndColors(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan error, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			err = connection.Close()
		}
		accepted <- err
	}()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	var out, stderr bytes.Buffer
	code := RunContext(context.Background(), []string{"probe", "--host", "127.0.0.1", "--ports", port, "--color", "always", "--json"}, strings.NewReader(""), &out, &stderr, "test")
	var report probe.Report
	if code != 0 || json.Unmarshal(out.Bytes(), &report) != nil || report.Summary.Connected != 1 || strings.Contains(out.String()+stderr.String(), "\x1b") {
		t.Fatal(code, &out, &stderr)
	}
	if err := <-accepted; err != nil {
		t.Fatal(err)
	}
	out.Reset()
	stderr.Reset()
	code = RunContext(context.Background(), []string{"probe", "--host", "127.0.0.1", "--ports", "0", "--color", "always", "--json"}, strings.NewReader(""), &out, &stderr, "test")
	if code != 1 || strings.Contains(out.String()+stderr.String(), "\x1b") {
		t.Fatal("JSON diagnostics gained colors", code, &out, &stderr)
	}
}
