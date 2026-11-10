package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Thuya-Myint/socklens/internal/tlscheck"
)

func TestTLSInputValidation(t *testing.T) {
	for _, args := range [][]string{nil, {"--host", "https://example.com"}, {"--host", "127.0.0.1", "--port", "0"}, {"--host", "127.0.0.1", "--port", "65536"}, {"--host", "127.0.0.1", "--family", "ipv6"}, {"--host", "localhost", "--handshake-timeout", "99ms"}, {"--host", "localhost", "--timeout", "0s"}, {"--host", "localhost", "extra"}} {
		var out, stderr bytes.Buffer
		engine := tlscheck.Engine{Dial: func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("network used for invalid options")
			return nil, nil
		}}
		if code := tlsCommand(context.Background(), args, &out, &stderr, engine); code != 1 || out.Len() != 0 {
			t.Fatalf("args=%v code=%d out=%s error=%s", args, code, &out, &stderr)
		}
	}
}

func TestTLSCommandPrivateCAJSON(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	defer server.Close()
	ca := filepath.Join(t.TempDir(), "root.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	host, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	var out, stderr bytes.Buffer
	code := Run([]string{"tls", "--host", host, "--port", port, "--ca-file", ca, "--json", "--color", "always"}, strings.NewReader(""), &out, &stderr, "test")
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d out=%s error=%s", code, &out, &stderr)
	}
	var report tlscheck.Report
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Results) != 1 || report.Results[0].Status != "verified" || bytes.Contains(out.Bytes(), []byte("\x1b")) {
		t.Fatalf("report=%+v", report)
	}
}

func TestTLSOutputErrorsAndEscaping(t *testing.T) {
	report := tlscheck.Report{Host: "example.test", Port: 443, ServerName: "example.test", Status: "completed", Results: []tlscheck.Result{{Endpoint: "127.0.0.1:443", Status: "certificate-error", Error: "bad\x1b[31m\nhost"}}}
	if err := writeTLSReport(brokenWriter{}, report); err == nil {
		t.Fatal("lost output failure")
	}
	var out bytes.Buffer
	if err := writeTLSReport(&out, report); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") || strings.Contains(out.String(), "bad\n") {
		t.Fatalf("unescaped remote content: %q", out.String())
	}
}

func TestTLSHelpAndCancellationJSON(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := Run([]string{"tls", "--help"}, strings.NewReader(""), &out, &stderr, "test"); code != 0 || !strings.Contains(stderr.String(), "server-name") {
		t.Fatalf("help code=%d error=%s", code, &stderr)
	}
	out.Reset()
	stderr.Reset()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code := RunContext(ctx, []string{"tls", "--host", "example.test", "--json"}, strings.NewReader(""), &out, &stderr, "test")
	if code != 130 || !strings.Contains(out.String(), `"status": "cancelled"`) {
		t.Fatalf("code=%d out=%s err=%s", code, &out, &stderr)
	}
}
