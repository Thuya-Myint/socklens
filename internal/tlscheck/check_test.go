package tlscheck

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T, expires time.Time, version uint16) (Options, *x509.CertPool, *atomic.Int32) {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test root"}, NotBefore: time.Now().Add(-7 * 24 * time.Hour), NotAfter: time.Now().Add(365 * 24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	rootDER, err := x509.CreateCertificate(rand.Reader, root, root, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	root, err = x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "service.test"}, DNSNames: []string{"service.test"}, NotBefore: time.Now().Add(-3 * 24 * time.Hour), NotAfter: expires, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, root, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	if err != nil {
		t.Fatal(err)
	}
	requests := &atomic.Int32{}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: version, MaxVersion: version}
	server.StartTLS()
	t.Cleanup(server.Close)
	host, portText, _ := net.SplitHostPort(server.Listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	roots := x509.NewCertPool()
	roots.AddCert(root)
	return Options{Host: host, Port: port, ServerName: "service.test", Family: "any", Timeout: time.Second, HandshakeTimeout: 500 * time.Millisecond}, roots, requests
}

func TestVerifiedVersionsAndNoApplicationRequests(t *testing.T) {
	for _, version := range []uint16{tls.VersionTLS12, tls.VersionTLS13} {
		t.Run(tls.VersionName(version), func(t *testing.T) {
			o, roots, requests := fixture(t, time.Now().Add(90*24*time.Hour), version)
			r, err := (Engine{Roots: roots}).Run(context.Background(), o)
			if err != nil || r.ExitCode() != 0 || len(r.Results) != 1 {
				t.Fatalf("report=%+v err=%v", r, err)
			}
			result := r.Results[0]
			if result.TLSVersion != tls.VersionName(version) || result.Certificate == nil || !result.Certificate.Verified || len(result.Certificate.SHA256) != 64 {
				t.Fatalf("result=%+v", result)
			}
			if requests.Load() != 0 {
				t.Fatal("sent an application request")
			}
		})
	}
}

func TestCertificateFailuresRemainUnverified(t *testing.T) {
	for _, kind := range []string{"expired", "wrong-host", "untrusted"} {
		t.Run(kind, func(t *testing.T) {
			expires := time.Now().Add(90 * 24 * time.Hour)
			if kind == "expired" {
				expires = time.Now().Add(-24 * time.Hour)
			}
			o, roots, _ := fixture(t, expires, tls.VersionTLS13)
			if kind == "wrong-host" {
				o.ServerName = "wrong.test"
			}
			if kind == "untrusted" {
				roots = x509.NewCertPool()
			}
			r, err := (Engine{Roots: roots}).Run(context.Background(), o)
			if err != nil || r.ExitCode() != 2 || r.Results[0].Status != "certificate-error" {
				t.Fatalf("report=%+v err=%v", r, err)
			}
			result := r.Results[0]
			if result.Certificate == nil || result.Certificate.Verified || result.TLSVersion != "" || result.Error == "" {
				t.Fatalf("unverified evidence mislabelled: %+v", result)
			}
			want := map[string]string{"expired": "expired", "wrong-host": "hostname-mismatch", "untrusted": "unknown-authority"}[kind]
			if result.Code != want || result.NextStep == "" {
				t.Fatalf("finding=%+v want=%s", result, want)
			}
		})
	}
}

func TestExpiringCertificateIsWarning(t *testing.T) {
	o, roots, _ := fixture(t, time.Now().Add(15*24*time.Hour), tls.VersionTLS13)
	r, err := (Engine{Roots: roots}).Run(context.Background(), o)
	if err != nil || r.ExitCode() != 2 || r.Results[0].Status != "verified" || r.Results[0].Warning == "" {
		t.Fatalf("report=%+v err=%v", r, err)
	}
}

type observedConnection struct {
	net.Conn
	closed *atomic.Bool
}

func (c observedConnection) Close() error { c.closed.Store(true); return c.Conn.Close() }

func TestResolveOncePinNumericEndpointAndClose(t *testing.T) {
	o, roots, _ := fixture(t, time.Now().Add(90*24*time.Hour), tls.VersionTLS13)
	ip := o.Host
	o.Host = "service.test"
	o.ServerName = ""
	lookups := 0
	closed := &atomic.Bool{}
	engine := Engine{Roots: roots, LookupIP: func(context.Context, string) ([]net.IPAddr, error) {
		lookups++
		return []net.IPAddr{{IP: net.ParseIP(ip)}}, nil
	}, Dial: func(ctx context.Context, network, endpoint string) (net.Conn, error) {
		if endpoint != net.JoinHostPort(ip, strconv.Itoa(o.Port)) {
			t.Fatalf("endpoint=%s", endpoint)
		}
		connection, err := (&net.Dialer{}).DialContext(ctx, network, endpoint)
		if err != nil {
			return nil, err
		}
		return observedConnection{connection, closed}, nil
	}}
	r, err := engine.Run(context.Background(), o)
	if err != nil || r.ExitCode() != 0 || lookups != 1 || !closed.Load() {
		t.Fatalf("report=%+v err=%v lookups=%d closed=%v", r, err, lookups, closed.Load())
	}
}

func TestHandshakeTimeoutClosesSocket(t *testing.T) {
	o := Options{Host: "127.0.0.1", Port: 443, Family: "any", Timeout: time.Second, HandshakeTimeout: 100 * time.Millisecond}
	client, server := net.Pipe()
	defer server.Close()
	closed := &atomic.Bool{}
	r, err := (Engine{Dial: func(context.Context, string, string) (net.Conn, error) {
		return observedConnection{client, closed}, nil
	}}).Run(context.Background(), o)
	if err != nil || r.ExitCode() != 2 || r.Results[0].Status != "timed-out" || !closed.Load() {
		t.Fatalf("report=%+v err=%v closed=%v", r, err, closed.Load())
	}
}

func TestCancellationBeforeDNS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	o := Options{Host: "service.test", Port: 443, Family: "any", Timeout: time.Second, HandshakeTimeout: time.Second}
	r, err := (Engine{LookupIP: func(context.Context, string) ([]net.IPAddr, error) {
		t.Fatal("resolved after cancellation")
		return nil, nil
	}}).Run(ctx, o)
	if err != nil || r.ExitCode() != 130 || r.Status != "cancelled" {
		t.Fatalf("report=%+v err=%v", r, err)
	}
}

func TestInvalidTargetsNeverDial(t *testing.T) {
	for _, host := range []string{"", "https://service.test", "example.com:443", "224.0.0.1", "0.0.0.0", "bad\nhost"} {
		o := Options{Host: host, Port: 443, Family: "any", Timeout: time.Second, HandshakeTimeout: time.Second}
		_, err := (Engine{Dial: func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("dialled invalid target")
			return nil, nil
		}}).Run(context.Background(), o)
		if err == nil {
			t.Fatalf("accepted %q", host)
		}
	}
}

func TestCancellationDuringHandshake(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, server := net.Pipe()
	defer server.Close()
	closed := &atomic.Bool{}
	go func() { var header [5]byte; _, _ = io.ReadFull(server, header[:]); cancel() }()
	o := Options{Host: "127.0.0.1", Port: 443, Family: "any", Timeout: time.Second, HandshakeTimeout: 500 * time.Millisecond}
	r, err := (Engine{Dial: func(context.Context, string, string) (net.Conn, error) {
		return observedConnection{client, closed}, nil
	}}).Run(ctx, o)
	if err != nil || r.ExitCode() != 130 || r.Results[0].Status != "cancelled" || !closed.Load() {
		t.Fatalf("report=%+v err=%v closed=%v", r, err, closed.Load())
	}
}

func TestDNSDeadlineAndTargetLimit(t *testing.T) {
	o := Options{Host: "service.test", Port: 443, Family: "any", Timeout: time.Second, HandshakeTimeout: time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	r, err := (Engine{LookupIP: func(ctx context.Context, _ string) ([]net.IPAddr, error) { <-ctx.Done(); return nil, ctx.Err() }}).Run(ctx, o)
	if err != nil || r.Status != "timed-out" || r.ExitCode() != 2 {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	engine := Engine{LookupIP: func(context.Context, string) ([]net.IPAddr, error) {
		ips := []net.IPAddr{}
		for n := 1; n <= 17; n++ {
			ips = append(ips, net.IPAddr{IP: net.IPv4(127, 0, 0, byte(n))})
		}
		return ips, nil
	}, Dial: func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("dialled oversized plan")
		return nil, nil
	}}
	r, err = engine.Run(context.Background(), o)
	if err != nil || r.Status != "resolution-error" || r.ExitCode() != 1 || len(r.Results) != 0 {
		t.Fatalf("report=%+v err=%v", r, err)
	}
}

func TestLegacyTLSIsNotSilentlyEnabled(t *testing.T) {
	o, roots, _ := fixture(t, time.Now().Add(90*24*time.Hour), tls.VersionTLS11)
	r, err := (Engine{Roots: roots}).Run(context.Background(), o)
	if err != nil || r.ExitCode() != 2 || r.Results[0].Status != "tls-error" || r.Results[0].TLSVersion != "" {
		t.Fatalf("report=%+v err=%v", r, err)
	}
}

func TestNotYetValidCertificateFinding(t *testing.T) {
	o, roots, _ := fixture(t, time.Now().Add(90*24*time.Hour), tls.VersionTLS13)
	r, err := (Engine{Roots: roots, Now: func() time.Time { return time.Now().Add(-4 * 24 * time.Hour) }}).Run(context.Background(), o)
	if err != nil || r.ExitCode() != 2 || r.Results[0].Code != "not-yet-valid" {
		t.Fatalf("report=%+v err=%v", r, err)
	}
}

func TestPlaintextIsTLSFailure(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	go func() {
		defer server.Close()
		var header [5]byte
		if _, err := io.ReadFull(server, header[:]); err != nil {
			return
		}
		if _, err := io.CopyN(io.Discard, server, int64(binary.BigEndian.Uint16(header[3:]))); err != nil {
			return
		}
		_, _ = io.WriteString(server, "HTTP/1.1 400 Bad Request\r\n\r\n")
	}()
	o := Options{Host: "127.0.0.1", Port: 443, Family: "any", Timeout: time.Second, HandshakeTimeout: 500 * time.Millisecond}
	r, err := (Engine{Dial: func(context.Context, string, string) (net.Conn, error) { return client, nil }}).Run(context.Background(), o)
	if err != nil || r.ExitCode() != 2 || r.Results[0].Status != "tls-error" || r.Results[0].Certificate != nil {
		t.Fatalf("report=%+v err=%v", r, err)
	}
}
