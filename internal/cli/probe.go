package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Thuya-Myint/socklens/internal/probe"
	"github.com/Thuya-Myint/socklens/internal/terminal"
)

func probeCommand(ctx context.Context, args []string, out, errOut io.Writer, engine probe.Engine) int {
	fs := newCommandFlags("probe", errOut)
	options := probe.Options{}
	fs.StringVar(&options.Host, "host", "", "one numeric IP or ASCII hostname (required; no URL/CIDR/port)")
	ports := fs.String("ports", "", "comma-separated TCP ports, 1-65535; at most 64 distinct ports (required)")
	fs.StringVar(&options.Family, "family", "any", "selected address family: any, ipv4, or ipv6")
	fs.DurationVar(&options.ConnectTimeout, "connect-timeout", 3*time.Second, "deadline per TCP attempt (100ms to 30s)")
	fs.DurationVar(&options.Timeout, "timeout", 15*time.Second, "total deadline including DNS and queued attempts (1s to 5m)")
	fs.IntVar(&options.Concurrency, "concurrency", 4, "maximum simultaneous TCP attempts (1-32)")
	asJSON := fs.Bool("json", false, "one complete or partial JSON TCP probe report")
	if err := parseCommandFlags(fs, args, errOut); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 {
		return fail(errOut, fmt.Errorf("probe takes --host HOST --ports PORTS, with no positional arguments"))
	}
	var err error
	options.Ports, err = probe.ParsePorts(*ports)
	if err != nil {
		return fail(errOut, err)
	}
	options, err = options.Normalize()
	if err != nil {
		return fail(errOut, err)
	}
	if !*asJSON {
		if err := writeFolded(out, terminal.Info, "INFO Checking TCP reachability...", terminal.Width(out)); err != nil {
			return fail(errOut, err)
		}
	}
	report, err := engine.Run(ctx, options)
	if err != nil {
		return fail(errOut, err)
	}
	if *asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(report)
	} else {
		err = writeProbeReport(out, report, terminal.Width(out))
	}
	if err != nil {
		return fail(errOut, err)
	}
	return report.ExitCode()
}

func writeProbeReport(out io.Writer, report probe.Report, columns int) error {
	line := func(role terminal.Role, value string) error { return writeFolded(out, role, auditText(value), columns) }
	field := func(label, value string) error {
		if columns > 0 {
			return writeSocketField(out, label, auditText(value), columns)
		}
		_, err := fmt.Fprintf(out, "  %s: %s\n", label, auditText(value))
		return err
	}
	if err := line(terminal.Heading, "SockLens probe | TCP | "+report.Host+" | "+report.Timestamp.Format(time.RFC3339)); err != nil {
		return err
	}
	addresses := []string{}
	for _, address := range report.Addresses {
		addresses = append(addresses, address.IP+" ("+address.Family+")")
	}
	if len(addresses) > 0 {
		if err := field("Resolved", strings.Join(addresses, ", ")); err != nil {
			return err
		}
	}
	summary := report.Summary
	if err := line(terminal.Info, fmt.Sprintf("Results: %d connected | %d refused | %d timed out | %d network errors | %d cancelled | %d not attempted", summary.Connected, summary.Refused, summary.TimedOut, summary.NetworkErrors, summary.Cancelled, summary.NotAttempted)); err != nil {
		return err
	}
	if report.Error != nil {
		if err := field("Reason", report.Error.Message); err != nil {
			return err
		}
	}
	for _, result := range report.Results {
		role := terminal.Warning
		switch result.Status {
		case "connected":
			role = terminal.Success
		case "network-error":
			role = terminal.Error
		case "not-attempted", "cancelled":
			role = terminal.Muted
		}
		heading := strings.ToUpper(result.Status) + " " + result.Endpoint
		if result.Attempted {
			heading += fmt.Sprintf(" | %.3f ms", result.DurationMS)
		}
		if err := line(role, heading); err != nil {
			return err
		}
		if result.LocalEndpoint != "" {
			if err := field("From", result.LocalEndpoint); err != nil {
				return err
			}
		}
		if result.RemoteEndpoint != "" && result.RemoteEndpoint != result.Endpoint {
			if err := field("Actual peer", result.RemoteEndpoint); err != nil {
				return err
			}
		}
		if err := field("Result", result.Message); err != nil {
			return err
		}
	}
	if err := line(terminal.Info, fmt.Sprintf("Completion: %s | attempted %d/%d | %.3f ms total", report.Status, summary.Attempted, summary.Planned, report.DurationMS)); err != nil {
		return err
	}
	return line(terminal.Muted, "Scope: TCP reachability from this machine. No application data sent; service health and security were not tested. Timeout causes remain unknown.")
}
