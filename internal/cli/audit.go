package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/Thuya-Myint/socklens/internal/audit"
	"github.com/Thuya-Myint/socklens/internal/firewall"
	"github.com/Thuya-Myint/socklens/internal/platform"
	"github.com/Thuya-Myint/socklens/internal/terminal"
)

func auditCommand(ctx context.Context, args []string, out, errOut io.Writer, list func() ([]platform.Socket, error)) int {
	return auditWithFirewall(ctx, args, out, errOut, list, platform.CollectFirewall)
}

type firewallCollector func(context.Context, []platform.Socket) firewall.Snapshot

func auditWithFirewall(ctx context.Context, args []string, out, errOut io.Writer, list func() ([]platform.Socket, error), collect firewallCollector) int {
	return auditWithMetadata(ctx, args, out, errOut, list, collect, platform.CollectServices)
}

type serviceCollector func(context.Context, []platform.Socket) *platform.ServiceSnapshot

func auditWithMetadata(ctx context.Context, args []string, out, errOut io.Writer, list func() ([]platform.Socket, error), collect firewallCollector, services serviceCollector) int {
	fs := newCommandFlags("audit", errOut)
	opts := addSocketFlags(fs)
	includeFirewall := fs.Bool("firewall", true, "collect read-only local firewall context; false keeps socket/process observations")
	peerValue := fs.String("peer", "", "numeric peer IP for candidate policy filtering; no connection is made")
	policyTimeout := fs.Duration("firewall-timeout", 20*time.Second, "total firewall query deadline (1s to 2m)")
	ruleLimit := fs.Int("rule-limit", 3, "maximum rules per direction in the selected view; 0 removes the limit (--details/JSON include unverified candidates)")
	details := fs.Bool("details", false, "show full evidence, firewall metadata, rule restrictions and raw policy (JSON always includes all)")
	if err := parseCommandFlags(fs, args, errOut); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if err := opts.validate(fs); err != nil {
		return fail(errOut, err)
	}
	var peer netip.Addr
	peerSupplied := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "peer" {
			peerSupplied = true
		}
	})
	if peerSupplied {
		var err error
		peer, err = firewall.ParsePeer(*peerValue)
		if err != nil {
			return fail(errOut, err)
		}
		if !*includeFirewall {
			return fail(errOut, fmt.Errorf("--peer requires firewall collection; remove --firewall=false"))
		}
	}
	if *policyTimeout < time.Second || *policyTimeout > 2*time.Minute {
		return fail(errOut, fmt.Errorf("firewall-timeout must be between 1s and 2m"))
	}
	if *ruleLimit < 0 || *ruleLimit > 10000 {
		return fail(errOut, fmt.Errorf("rule-limit must be between 0 and 10000"))
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
	lookup := services(ctx, rows)
	if ctx.Err() != nil {
		return auditStopped(ctx, errOut)
	}
	audit.AddServices(&report, lookup)
	if *includeFirewall {
		policyCtx, cancel := context.WithTimeout(ctx, *policyTimeout)
		snapshot := collect(policyCtx, rows)
		if policyCtx.Err() != nil {
			snapshot.Notes = append(snapshot.Notes, "Firewall collection deadline/cancellation: "+policyCtx.Err().Error())
			snapshot.Coverage = "partial"
			readSource := false
			for _, source := range snapshot.Sources {
				if source.Status == "read" {
					readSource = true
				}
			}
			if !readSource {
				snapshot.Coverage = "unknown"
			}
		}
		cancel()
		if ctx.Err() != nil {
			return auditStopped(ctx, errOut)
		}
		report.Firewall = &snapshot
		for i := range report.Listeners {
			listener := &report.Listeners[i]
			context := firewall.Explain(snapshot, listener.Protocol, listener.Local, listener.PIDs, peer)
			listener.Firewall = &context
		}
	}
	if opts.json {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return fail(errOut, err)
		}
	} else if err := writeAuditReportMode(out, report, terminal.Width(out), *ruleLimit, *details); err != nil {
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
	return writeAuditReportLimited(out, report, columns, 3)
}
func writeAuditReportLimited(out io.Writer, report audit.Report, columns, ruleLimit int) error {
	return writeAuditReportMode(out, report, columns, ruleLimit, false)
}
func writeAuditReportMode(out io.Writer, report audit.Report, columns, ruleLimit int, details bool) error {
	if details {
		return writeAuditReportDetailed(out, report, columns, ruleLimit)
	}
	return writeAuditReportCompact(out, report, columns, ruleLimit)
}
func writeAuditReportDetailed(out io.Writer, report audit.Report, columns, ruleLimit int) error {
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
	if err := writeFolded(out, terminal.Muted, "Read first: non-loopback service-port hints, other review items, other specific bindings, then loopback. Hints are unverified; order is not a vulnerability score.", columns); err != nil {
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
	if report.Firewall != nil {
		policy := report.Firewall
		if err := field("Firewall", policy.Coverage+" coverage; candidate rules only; effective decision unknown"); err != nil {
			return err
		}
		for _, source := range policy.Sources {
			if err := field("Source", source.Name+": "+source.Status+"; "+source.Detail); err != nil {
				return err
			}
		}
		for _, profile := range policy.Profiles {
			if err := field("Profile", profile.Name+" | enabled="+profile.Enabled+" | default in="+profile.Inbound+" out="+profile.Outbound+" | "+profile.Detail); err != nil {
				return err
			}
		}
		for _, iface := range policy.Interfaces {
			if err := field("Interface", iface.Name+" | "+strings.Join(iface.Addresses, ", ")+" | profile="+iface.Profile); err != nil {
				return err
			}
		}
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
		for _, service := range listener.Services {
			if err := field("Service", fmt.Sprintf("%s | display=%s | PID %d | %s | %s | %s", service.Name, service.DisplayName, service.PID, service.State, service.ProcessType, service.Source)); err != nil {
				return err
			}
		}
		incoming, outgoing := auditCommunication(listener)
		if err := field("Inbound", incoming); err != nil {
			return err
		}
		if err := field("Outbound", outgoing); err != nil {
			return err
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
		if listener.Firewall != nil {
			policy := listener.Firewall
			if err := field("Local IPs", strings.Join(policy.LocalAddresses, ", ")); err != nil {
				return err
			}
			if policy.Peer != "" {
				if err := field("Peer", policy.Peer+" | "+policy.PeerCompatibility); err != nil {
					return err
				}
			}
			for _, direction := range []struct {
				name string
				data firewall.Direction
			}{{"Inbound", policy.Inbound}, {"Outbound", policy.Outbound}} {
				if err := field(direction.name, direction.data.Assessment+" | "+direction.data.Summary); err != nil {
					return err
				}
				for i, candidate := range direction.data.Rules {
					if ruleLimit > 0 && i >= ruleLimit {
						if err := field("More rules", fmt.Sprintf("%d additional %s candidates; use --rule-limit 0 or --json to view all.", len(direction.data.Rules)-i, strings.ToLower(direction.name))); err != nil {
							return err
						}
						break
					}
					if err := field("Relevance", candidate.Relevance); err != nil {
						return err
					}
					rule := candidate.Rule
					if err := field("Rule", strings.ToUpper(rule.Action)+" "+rule.Name+" | "+candidate.Match+" | "+rule.Source+" #"+rule.ID); err != nil {
						return err
					}
					if err := field("Ports", fmt.Sprintf("protocol=%v local=%v remote=%v", rule.Protocols, rule.LocalPorts, rule.RemotePorts)); err != nil {
						return err
					}
					if err := field("Addresses", fmt.Sprintf("local=%v remote=%v", rule.LocalAddresses, rule.RemoteAddresses)); err != nil {
						return err
					}
					if rule.Program != "" {
						if err := field("Program", rule.Program); err != nil {
							return err
						}
					}
					if err := field("Applies if", "profile="+rule.Profiles+" | interfaces="+rule.Interfaces+" | "+strings.Join(rule.Conditions, "; ")); err != nil {
						return err
					}
					if rule.Raw != "" {
						if err := field("Raw rule", rule.Raw); err != nil {
							return err
						}
					}
					for _, reason := range candidate.Reasons {
						if err := field("Limit", reason); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	if report.Firewall != nil {
		for _, note := range report.Firewall.Notes {
			if err := writeFolded(out, terminal.Muted, auditText(note), columns); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}
	if report.ServiceLookup != nil {
		if err := field("Services", report.ServiceLookup.Status+" | "+report.ServiceLookup.Detail); err != nil {
			return err
		}
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
