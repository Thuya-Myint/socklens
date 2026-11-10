package cli

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Thuya-Myint/socklens/internal/terminal"
	"github.com/Thuya-Myint/socklens/internal/tlscheck"
)

func tlsCommand(ctx context.Context, args []string, out, errOut io.Writer, engine tlscheck.Engine) int {
	fs := newCommandFlags("tls", errOut)
	o := tlscheck.Options{}
	fs.StringVar(&o.Host, "host", "", "one IP or ASCII hostname to check (required)")
	fs.IntVar(&o.Port, "port", 443, "TCP port (1-65535)")
	fs.StringVar(&o.ServerName, "server-name", "", "certificate verification name and DNS SNI (defaults to host)")
	fs.StringVar(&o.Family, "family", "any", "address family: any, ipv4, or ipv6")
	fs.DurationVar(&o.Timeout, "timeout", 15*time.Second, "total deadline including DNS (1s to 5m)")
	fs.DurationVar(&o.HandshakeTimeout, "handshake-timeout", 3*time.Second, "per-address TCP and TLS deadline (100ms to 30s)")
	caFile := fs.String("ca-file", "", "PEM CA certificates to add to system trust for this check only")
	asJSON := fs.Bool("json", false, "one JSON TLS report")
	if err := parseCommandFlags(fs, args, errOut); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 {
		return fail(errOut, fmt.Errorf("tls takes --host HOST and options, with no positional arguments"))
	}
	var err error
	o, err = o.Normalize()
	if err != nil {
		return fail(errOut, err)
	}
	if *caFile != "" {
		info, err := os.Stat(*caFile)
		if err != nil {
			return fail(errOut, err)
		}
		if !info.Mode().IsRegular() {
			return fail(errOut, fmt.Errorf("ca-file must be a regular PEM file"))
		}
		file, err := os.Open(*caFile)
		if err != nil {
			return fail(errOut, err)
		}
		data, readErr := io.ReadAll(io.LimitReader(file, 4<<20+1))
		closeErr := file.Close()
		if readErr != nil {
			return fail(errOut, readErr)
		}
		if closeErr != nil {
			return fail(errOut, closeErr)
		}
		if len(data) > 4<<20 {
			return fail(errOut, fmt.Errorf("ca-file exceeds 4 MiB"))
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			return fail(errOut, fmt.Errorf("load system trust: %w", err))
		}
		if !roots.AppendCertsFromPEM(data) {
			return fail(errOut, fmt.Errorf("ca-file contains no usable PEM CA certificates"))
		}
		engine.Roots = roots
	}
	report, err := engine.Run(ctx, o)
	if err != nil {
		return fail(errOut, err)
	}
	if *asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(report)
	} else {
		err = writeTLSReport(out, report)
	}
	if err != nil {
		return fail(errOut, err)
	}
	return report.ExitCode()
}

func writeTLSReport(out io.Writer, report tlscheck.Report) error {
	line := func(role terminal.Role, value string) error {
		return writeFolded(out, role, auditText(value), terminal.Width(out))
	}
	if err := line(terminal.Heading, fmt.Sprintf("SockLens TLS | %s:%d | verify %s", report.Host, report.Port, report.ServerName)); err != nil {
		return err
	}
	if report.Error != "" {
		if err := line(terminal.Error, report.Error); err != nil {
			return err
		}
	}
	for _, r := range report.Results {
		role := terminal.Warning
		if r.Status == "verified" && r.Warning == "" {
			role = terminal.Success
		}
		if err := line(role, r.Status+" | "+r.Endpoint); err != nil {
			return err
		}
		if r.TLSVersion != "" {
			if err := line(terminal.Info, r.TLSVersion+" | "+r.CipherSuite); err != nil {
				return err
			}
		}
		if c := r.Certificate; c != nil {
			for _, value := range []string{"Subject: " + c.Subject, "Issuer: " + c.Issuer, "Valid from: " + c.NotBefore.Format(time.RFC3339), "Expires: " + c.NotAfter.Format(time.RFC3339), "Certificate SHA-256: " + c.SHA256} {
				if err := line(terminal.Info, value); err != nil {
					return err
				}
			}
		}
		if r.Error != "" {
			if err := line(terminal.Error, r.Error); err != nil {
				return err
			}
		}
		if r.Warning != "" {
			if err := line(terminal.Warning, r.Warning); err != nil {
				return err
			}
		}
		if r.Certificate != nil && !r.Certificate.Verified {
			if err := line(terminal.Warning, "Certificate evidence is unverified."); err != nil {
				return err
			}
		}
		if r.NextStep != "" {
			if err := line(terminal.Info, "Next step: "+r.NextStep); err != nil {
				return err
			}
		}
	}
	if err := line(terminal.Info, "Completion: "+report.Status); err != nil {
		return err
	}
	return line(terminal.Muted, "Scope: one verified TLS handshake per selected address; no application request, cipher enumeration, revocation check, or vulnerability verdict.")
}
