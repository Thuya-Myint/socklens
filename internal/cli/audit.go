package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/Thuya-Myint/socklens/internal/audit"
	"github.com/Thuya-Myint/socklens/internal/platform"
	"github.com/Thuya-Myint/socklens/internal/terminal"
)

func auditCommand(ctx context.Context, args []string, out, errOut io.Writer, list func() ([]platform.Socket, error)) int {
	fs := newCommandFlags("audit", errOut)
	opts := addSocketFlags(fs)
	if err := parseCommandFlags(fs, args, errOut); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if err := opts.validate(fs); err != nil {
		return fail(errOut, err)
	}
	if ctx.Err() != nil {
		return auditStopped(ctx, errOut)
	}
	rows, err := list()
	if ctx.Err() != nil {
		return auditStopped(ctx, errOut)
	}
	if err != nil {
		return fail(errOut, fmt.Errorf("collect audit snapshot: %w", err))
	}
	rows, err = filterSockets(rows, opts.filter)
	if err != nil {
		return fail(errOut, err)
	}
	report, err := audit.Analyze(rows, time.Now())
	if ctx.Err() != nil {
		return auditStopped(ctx, errOut)
	}
	if err != nil {
		return fail(errOut, err)
	}
	if opts.json {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return fail(errOut, err)
		}
	} else if err := writeAuditReport(out, report, terminal.Width(out)); err != nil {
		return fail(errOut, err)
	}
	return 0
}

func auditStopped(ctx context.Context, out io.Writer) int {
	if errors.Is(ctx.Err(), context.Canceled) {
		_ = status(out, terminal.Info, "CANCELLED", "Local audit interrupted.")
		return 130
	}
	return fail(out, ctx.Err())
}

// Escape untrusted strings before measuring or wrapping terminal cells.
func auditText(value string) string {
	if strings.IndexFunc(value, func(r rune) bool { return r < 32 || r > 126 }) >= 0 {
		return strconv.QuoteToASCII(value)
	}
	return value
}

func writeAuditReport(out io.Writer, report audit.Report, columns int) error {
	if err := writeFolded(out, terminal.Heading, "Local listener audit | "+report.Timestamp.Format(time.RFC3339), columns); err != nil {
		return err
	}
	summary := fmt.Sprintf("Listeners: %d | Review: %d | Information: %d | Findings: %d", report.Summary.Listeners, report.Summary.ReviewListeners, report.Summary.InformationListeners, report.Summary.Findings)
	if err := writeFolded(out, terminal.Info, summary, columns); err != nil {
		return err
	}
	if err := writeFolded(out, terminal.Info, "External reachability: not tested. Review items are not confirmed vulnerabilities.", columns); err != nil {
		return err
	}
	if len(report.Listeners) == 0 {
		if err := writeFolded(out, terminal.Info, "INFO No matching local listeners were observed.", columns); err != nil {
			return err
		}
	}
	field := func(label, value string) error {
		value = auditText(value)
		if columns > 0 {
			return writeSocketField(out, label, value, columns)
		}
		_, err := fmt.Fprintf(out, "  %s: %s\n", label, value)
		return err
	}
	for _, listener := range report.Listeners {
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
		role, label := terminal.Info, "INFO"
		for _, finding := range listener.Findings {
			if finding.Level == "review" {
				role, label = terminal.Warning, "REVIEW"
				break
			}
		}
		if err := writeFolded(out, role, label+" "+strings.ToUpper(listener.Protocol)+" "+auditText(listener.Local), columns); err != nil {
			return err
		}
		pids := make([]string, len(listener.PIDs))
		for i, pid := range listener.PIDs {
			pids[i] = strconv.Itoa(pid)
		}
		owners := strings.Join(pids, ",")
		if owners == "" {
			owners = "unknown"
		}
		processes := make([]string, 0, len(listener.Processes))
		for _, process := range listener.Processes {
			if process.Name != "" {
				processes = append(processes, fmt.Sprintf("%d:%s", process.PID, processName(process.Name)))
			}
		}
		names := strings.Join(processes, ", ")
		if names == "" {
			names = "unknown"
		}
		for _, entry := range []struct{ label, value string }{{"Binding", listener.BindingScope}, {"Family", listener.AddressFamily}, {"PIDs", owners}, {"Processes", names}, {"Visibility", listener.ProcessVisibility}} {
			if err := field(entry.label, entry.value); err != nil {
				return err
			}
		}
		if listener.ServiceHint != nil {
			if err := field("Hint", listener.ServiceHint.Name+" (port convention; unverified)"); err != nil {
				return err
			}
		}
		for _, finding := range listener.Findings {
			findingRole := terminal.Info
			if finding.Level == "review" {
				findingRole = terminal.Warning
			}
			if err := writeFolded(out, findingRole, "  "+strings.ToUpper(finding.Level)+" ["+finding.RuleID+"] "+auditText(finding.Title), columns); err != nil {
				return err
			}
			if err := field("Confidence", finding.Confidence); err != nil {
				return err
			}
			for _, evidence := range finding.Evidence {
				if err := field("Evidence", evidence); err != nil {
					return err
				}
			}
			if err := field("Next step", finding.Recommendation); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}
	if report.Summary.MissingProcessDetails > 0 {
		if err := writeFolded(out, terminal.Info, fmt.Sprintf("INFO Listeners with incomplete process details: %d. Permissions may limit visibility.", report.Summary.MissingProcessDetails), columns); err != nil {
			return err
		}
	}
	for _, note := range report.Notes {
		if err := writeFolded(out, terminal.Muted, auditText(note), columns); err != nil {
			return err
		}
	}
	return nil
}
