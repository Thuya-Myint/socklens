// Package tlscheck performs explicit, verified TLS handshakes without sending
// application requests or weakening certificate verification.
package tlscheck

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"time"

	"github.com/Thuya-Myint/socklens/internal/probe"
)

type Options struct {
	Host             string
	Port             int
	ServerName       string
	Family           string
	Timeout          time.Duration
	HandshakeTimeout time.Duration
}

func (o Options) Normalize() (Options, error) {
	var err error
	o.Host, err = probe.NormalizeHost(o.Host)
	if err != nil {
		return o, err
	}
	if o.ServerName == "" {
		o.ServerName = o.Host
	}
	o.ServerName, err = probe.NormalizeHost(o.ServerName)
	if err != nil {
		return o, fmt.Errorf("server-name: %w", err)
	}
	if ip, err := netip.ParseAddr(o.ServerName); err == nil && ip.Zone() != "" {
		return o, fmt.Errorf("server-name must not contain an IPv6 zone")
	}
	if o.Port < 1 || o.Port > 65535 {
		return o, fmt.Errorf("port must be 1-65535")
	}
	if o.Family != "any" && o.Family != "ipv4" && o.Family != "ipv6" {
		return o, fmt.Errorf("family must be any, ipv4, or ipv6")
	}
	if ip, err := netip.ParseAddr(o.Host); err == nil && (o.Family == "ipv4" && !ip.Is4() || o.Family == "ipv6" && !ip.Is6()) {
		return o, fmt.Errorf("host address does not match requested family")
	}
	if o.Timeout < time.Second || o.Timeout > 5*time.Minute {
		return o, fmt.Errorf("timeout must be between 1s and 5m")
	}
	if o.HandshakeTimeout < 100*time.Millisecond || o.HandshakeTimeout > 30*time.Second {
		return o, fmt.Errorf("handshake-timeout must be between 100ms and 30s")
	}
	return o, nil
}

type Certificate struct {
	Subject     string    `json:"subject"`
	Issuer      string    `json:"issuer"`
	DNSNames    []string  `json:"dns_names"`
	IPAddresses []string  `json:"ip_addresses"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	SHA256      string    `json:"sha256"`
	Verified    bool      `json:"verified"`
}

type Result struct {
	Endpoint    string       `json:"endpoint"`
	Status      string       `json:"status"`
	DurationMS  float64      `json:"duration_ms"`
	TLSVersion  string       `json:"tls_version,omitempty"`
	CipherSuite string       `json:"cipher_suite,omitempty"`
	Certificate *Certificate `json:"certificate,omitempty"`
	Warning     string       `json:"warning,omitempty"`
	Error       string       `json:"error,omitempty"`
	Code        string       `json:"code,omitempty"`
	NextStep    string       `json:"next_step,omitempty"`
}

type Report struct {
	SchemaVersion      int       `json:"schema_version"`
	Timestamp          time.Time `json:"timestamp"`
	Host               string    `json:"host"`
	Port               int       `json:"port"`
	ServerName         string    `json:"server_name"`
	Family             string    `json:"family"`
	TimeoutMS          int64     `json:"timeout_ms"`
	HandshakeTimeoutMS int64     `json:"handshake_timeout_ms"`
	DurationMS         float64   `json:"duration_ms"`
	Status             string    `json:"status"`
	Error              string    `json:"error,omitempty"`
	Results            []Result  `json:"results"`
	Notes              []string  `json:"notes"`
}

func (r Report) ExitCode() int {
	if r.Status == "cancelled" {
		return 130
	}
	if r.Status == "resolution-error" {
		return 1
	}
	if r.Status != "completed" || len(r.Results) == 0 {
		return 2
	}
	for _, result := range r.Results {
		if result.Status != "verified" || result.Warning != "" {
			return 2
		}
	}
	return 0
}

type Engine struct {
	LookupIP func(context.Context, string) ([]net.IPAddr, error)
	Dial     func(context.Context, string, string) (net.Conn, error)
	Roots    *x509.CertPool
	Now      func() time.Time
}

func (e Engine) Run(parent context.Context, options Options) (report Report, err error) {
	o, err := options.Normalize()
	if err != nil {
		return report, err
	}
	start := time.Now()
	defer func() { report.DurationMS = float64(time.Since(start)) / float64(time.Millisecond) }()
	now := e.Now
	if now == nil {
		now = time.Now
	}
	report = Report{SchemaVersion: 1, Timestamp: now().UTC(), Host: o.Host, Port: o.Port, ServerName: o.ServerName, Family: o.Family, TimeoutMS: o.Timeout.Milliseconds(), HandshakeTimeoutMS: o.HandshakeTimeout.Milliseconds(), Status: "completed", Results: []Result{}, Notes: []string{
		"Explicit TLS handshakes only; no HTTP/application requests, STARTTLS, client certificates, redirects, or background update checks.",
		"Each selected address is resolved once and checked separately, using TLS 1.2 or newer and normal hostname/chain verification.",
		"One negotiated version/cipher does not enumerate server support. No authentication, application health, revocation, or CVE verdict is established.",
		"Certificates attached to failed verification are untrusted evidence. Expiry within 30 days is a warning, not a verification failure.",
	}}
	ctx, cancel := context.WithTimeout(parent, o.Timeout)
	defer cancel()
	stop := func() {
		report.Status = "timed-out"
		if errors.Is(ctx.Err(), context.Canceled) {
			report.Status = "cancelled"
		}
	}
	if ctx.Err() != nil {
		stop()
		return report, nil
	}
	addresses, lookupErr := probe.ResolveAddresses(ctx, o.Host, o.Family, e.LookupIP)
	if ctx.Err() != nil {
		stop()
		return report, nil
	}
	if lookupErr != nil {
		report.Status = "resolution-error"
		report.Error = lookupErr.Error()
		return report, nil
	}
	for _, ip := range addresses {
		report.Results = append(report.Results, Result{Endpoint: net.JoinHostPort(ip.String(), strconv.Itoa(o.Port)), Status: "not-attempted"})
	}
	dial := e.Dial
	if dial == nil {
		dial = (&net.Dialer{KeepAlive: -1}).DialContext
	}
	for i := range report.Results {
		if ctx.Err() != nil {
			stop()
			break
		}
		report.Results[i] = e.check(ctx, o, report.Results[i].Endpoint, dial, now)
	}
	if ctx.Err() != nil {
		stop()
	}
	return report, nil
}

func (e Engine) check(parent context.Context, o Options, endpoint string, dial func(context.Context, string, string) (net.Conn, error), now func() time.Time) (result Result) {
	result = Result{Endpoint: endpoint}
	start := time.Now()
	defer func() { result.DurationMS = float64(time.Since(start)) / float64(time.Millisecond) }()
	ctx, cancel := context.WithTimeout(parent, o.HandshakeTimeout)
	defer cancel()
	connection, err := dial(ctx, "tcp", endpoint) // numeric endpoint: never re-resolve DNS
	if connection != nil {
		defer connection.Close()
	}
	if err != nil {
		result.Status = "network-error"
		result.Error = err.Error()
	} else if connection == nil {
		result.Status = "network-error"
		result.Error = "dial returned no connection"
	} else {
		client := tls.Client(connection, &tls.Config{ServerName: o.ServerName, RootCAs: e.Roots, MinVersion: tls.VersionTLS12, Time: now})
		err = client.HandshakeContext(ctx)
		if err == nil {
			state := client.ConnectionState()
			result.Status = "verified"
			result.TLSVersion = tls.VersionName(state.Version)
			result.CipherSuite = tls.CipherSuiteName(state.CipherSuite)
			if len(state.PeerCertificates) > 0 {
				result.Certificate = certificate(state.PeerCertificates[0], true)
			}
			if result.Certificate != nil && result.Certificate.NotAfter.Sub(now()) <= 30*24*time.Hour {
				result.Warning = "certificate expires within 30 days"
				result.Code = "expires-soon"
				result.NextStep = "Renew the certificate before its expiry date."
			}
		} else {
			result.Status = "tls-error"
			result.Error = err.Error()
			result.NextStep = "Confirm this port serves TLS directly and supports TLS 1.2 or newer."
			var verification *tls.CertificateVerificationError
			if errors.As(err, &verification) {
				result.Status = "certificate-error"
				result.Code, result.NextStep = verificationFinding(verification.Err, now())
				if len(verification.UnverifiedCertificates) > 0 {
					result.Certificate = certificate(verification.UnverifiedCertificates[0], false)
				}
			}
		}
	}
	if ctx.Err() != nil {
		result.Status = "timed-out"
		if errors.Is(ctx.Err(), context.Canceled) {
			result.Status = "cancelled"
		}
		result.Error = ctx.Err().Error()
	} else {
		var timeout net.Error
		if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &timeout) && timeout.Timeout() {
			result.Status = "timed-out"
		}
	}
	return result
}

func verificationFinding(err error, now time.Time) (string, string) {
	var hostname x509.HostnameError
	if errors.As(err, &hostname) {
		return "hostname-mismatch", "Check the verification name and deploy a certificate covering the intended hostname or IP."
	}
	var authority x509.UnknownAuthorityError
	if errors.As(err, &authority) {
		return "unknown-authority", "Serve the correct certificate chain; for private PKI, explicitly supply its trusted CA with --ca-file."
	}
	var invalid x509.CertificateInvalidError
	if errors.As(err, &invalid) && invalid.Reason == x509.Expired {
		if now.Before(invalid.Cert.NotBefore) {
			return "not-yet-valid", "Check the system clock and the certificate activation date."
		}
		return "expired", "Renew the expired certificate or intermediate, and check the system clock."
	}
	return "certificate-invalid", "Inspect the verification error and correct the server certificate chain or intended verification name."
}

func certificate(cert *x509.Certificate, verified bool) *Certificate {
	fingerprint := sha256.Sum256(cert.Raw)
	ips := []string{}
	for _, ip := range cert.IPAddresses {
		ips = append(ips, ip.String())
	}
	return &Certificate{Subject: cert.Subject.String(), Issuer: cert.Issuer.String(), DNSNames: append([]string{}, cert.DNSNames...), IPAddresses: ips, NotBefore: cert.NotBefore.UTC(), NotAfter: cert.NotAfter.UTC(), SHA256: hex.EncodeToString(fingerprint[:]), Verified: verified}
}
