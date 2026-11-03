package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/Thuya-Myint/socklens/internal/audit"
	"github.com/Thuya-Myint/socklens/internal/firewall"
	"github.com/Thuya-Myint/socklens/internal/terminal"
)

// The concise view emphasizes decisions a person needs to review. Detailed
// evidence and policy restrictions remain available via --details and JSON.
func writeAuditReportCompact(out io.Writer, report audit.Report, columns, ruleLimit int) error {
	line := func(role terminal.Role, value string) error { return writeFolded(out, role, auditText(value), columns) }
	field := func(label, value string) error {
		if columns > 0 {
			return writeSocketField(out, label, auditText(value), columns)
		}
		_, err := fmt.Fprintf(out, "  %s: %s\n", label, auditText(value))
		return err
	}
	if err := line(terminal.Heading, "SockLens audit | "+report.Timestamp.Format(time.RFC3339)); err != nil {
		return err
	}
	if err := line(terminal.Info, fmt.Sprintf("Listeners: %d | Review: %d | Information: %d | Findings: %d", report.Summary.Listeners, report.Summary.ReviewListeners, report.Summary.InformationListeners, report.Summary.Findings)); err != nil {
		return err
	}
	if err := line(terminal.Muted, "External reachability: not tested. Review items are not confirmed vulnerabilities."); err != nil {
		return err
	}
	if err := line(terminal.Muted, "Review entries first; order is not a vulnerability score. Service hints are unverified."); err != nil {
		return err
	}
	if report.Firewall != nil {
		policy := report.Firewall
		read := 0
		for _, source := range policy.Sources {
			if source.Status == "read" {
				read++
			}
		}
		if err := field("Firewall", fmt.Sprintf("%s coverage | %d/%d sources read | decisions unknown", policy.Coverage, read, len(policy.Sources))); err != nil {
			return err
		}
		// Successful fallback and failed sources are reported once per snapshot,
		// rather than repeating collector metadata inside every listener.
		for _, source := range policy.Sources {
			value := auditSourceName(source.Name) + ": " + source.Status
			if source.Status != "read" && source.Detail != "" {
				value += "; " + source.Detail
			}
			if err := field("Source", value); err != nil {
				return err
			}
		}
		networks := []string{}
		for _, iface := range policy.Interfaces {
			if iface.Profile != "" {
				networks = append(networks, iface.Name+" ("+iface.Profile+")")
			}
		}
		if len(networks) > 0 {
			if err := field("Network", strings.Join(networks, ", ")); err != nil {
				return err
			}
		}
		for _, profile := range policy.Profiles {
			name := profile.Name
			if strings.Contains(profile.Detail, "profile active=True") {
				name += " (active)"
			}
			switch profile.Name {
			case "Application Firewall --getglobalstate":
				name = "Application Firewall"
			case "Application Firewall --getblockall":
				name = "Block-all setting"
			case "Application Firewall --getallowsigned":
				name = "Signed-app setting"
			}
			state := profile.Enabled
			switch strings.ToLower(state) {
			case "true", "1":
				state = "configured on"
			case "false", "0":
				state = "configured off"
			}
			value := name + " | " + state + " | defaults: in=" + profile.Inbound + ", out=" + profile.Outbound
			if profile.Enabled == "see detail" {
				value = name + " | " + strings.SplitN(strings.TrimSpace(profile.Detail), "\n", 2)[0]
			}
			if err := field("Profile", value); err != nil {
				return err
			}
		}
	}
	if len(report.Listeners) == 0 {
		if err := line(terminal.Info, "INFO No matching local listeners were observed."); err != nil {
			return err
		}
	}
	hiddenRules := false
	for _, listener := range report.Listeners {
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
		label, role := "INFO", terminal.Info
		for _, finding := range listener.Findings {
			if finding.Level == "review" {
				label, role = "REVIEW", terminal.Warning
				break
			}
		}
		if err := line(role, label+" "+strings.ToUpper(listener.Protocol)+" "+listener.Local); err != nil {
			return err
		}
		if err := field("Process", auditOwnerSummary(listener)); err != nil {
			return err
		}
		if err := field("Binding", auditBindingSummary(listener)); err != nil {
			return err
		}
		if listener.ProcessVisibility != "observed" {
			if err := field("Ownership", listener.ProcessVisibility+"; process details are incomplete"); err != nil {
				return err
			}
		}
		if listener.ServiceHint != nil {
			if err := field("Hint", listener.ServiceHint.Name+" (port convention; unverified)"); err != nil {
				return err
			}
		}
		actions := []string{}
		for _, finding := range listener.Findings {
			label := "Info"
			if finding.Level == "review" {
				label = "Review"
			}
			if err := field(label, finding.Title+" ["+finding.Confidence+"]"); err != nil {
				return err
			}
			action := auditShortAction(finding)
			if action != "" {
				found := false
				for _, existing := range actions {
					found = found || existing == action
				}
				if !found {
					actions = append(actions, action)
				}
			}
		}
		if len(actions) > 0 {
			if err := field("Next", strings.Join(actions, " ")); err != nil {
				return err
			}
		}
		if listener.Firewall != nil {
			policy := listener.Firewall
			if listener.BindingScope == "wildcard" && len(policy.LocalAddresses) > 0 {
				if err := field("Local IPs", strings.Join(policy.LocalAddresses, ", ")); err != nil {
					return err
				}
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
				allow, block, other := 0, 0, 0
				for _, candidate := range direction.data.Rules {
					switch candidate.Rule.Action {
					case "allow":
						allow++
					case "block":
						block++
					default:
						other++
					}
				}
				summary := fmt.Sprintf("%s | %d conditional candidates (%d allow, %d block", direction.data.Assessment, len(direction.data.Rules), allow, block)
				if other > 0 {
					summary += fmt.Sprintf(", %d other", other)
				}
				summary += ")"
				if err := field(direction.name, summary); err != nil {
					return err
				}
				if strings.HasPrefix(policy.PeerCompatibility, "different-family") || strings.HasPrefix(policy.PeerCompatibility, "local-only") {
					if err := field("Scope", direction.data.Summary); err != nil {
						return err
					}
				}
				for i, candidate := range direction.data.Rules {
					if ruleLimit > 0 && i >= ruleLimit {
						hiddenRules = true
						if err := field("More rules", fmt.Sprintf("%d additional %s candidates", len(direction.data.Rules)-i, strings.ToLower(direction.name))); err != nil {
							return err
						}
						break
					}
					rule := candidate.Rule
					name := rule.Name
					if name == "" {
						name = rule.ID
					}
					if name == "" {
						name = "unnamed rule"
					}
					if err := field("Rule", strings.ToUpper(rule.Action)+" "+name+" ["+candidate.Match+"]"); err != nil {
						return err
					}
					if err := field("Traffic", auditRuleValues(rule.Protocols, true)+" | local port="+auditRuleValues(rule.LocalPorts, false)+" | remote port="+auditRuleValues(rule.RemotePorts, false)); err != nil {
						return err
					}
					if err := field("IPs", "local="+auditRuleValues(rule.LocalAddresses, false)+" | remote="+auditRuleValues(rule.RemoteAddresses, false)); err != nil {
						return err
					}
					scope := []string{}
					if rule.Profiles != "" {
						scope = append(scope, "profile="+rule.Profiles)
					}
					if rule.Interfaces != "" {
						scope = append(scope, "interfaces="+rule.Interfaces)
					}
					if rule.Program != "" {
						scope = append(scope, "program="+auditProgramName(rule.Program))
					}
					if len(scope) > 0 {
						if err := field("Scope", strings.Join(scope, " | ")); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}
	if report.Firewall != nil {
		if err := line(terminal.Muted, "Policy candidates are conditional. Other restrictions, profile/interface applicability, state and precedence remain unresolved."); err != nil {
			return err
		}
		if err := line(terminal.Muted, "No candidates does not mean allowed or blocked. Partial/unknown coverage can miss filtering layers."); err != nil {
			return err
		}
	}
	if report.Summary.MissingProcessDetails > 0 {
		if err := line(terminal.Info, fmt.Sprintf("INFO %d listeners have incomplete process details.", report.Summary.MissingProcessDetails)); err != nil {
			return err
		}
	}
	tips := "Use --details for full evidence, metadata, restrictions, raw policy and program paths; --json for structured data."
	if hiddenRules {
		tips += " Use --rule-limit 0 to show all candidate rules."
	}
	return line(terminal.Muted, tips)
}

func auditOwnerSummary(listener audit.Listener) string {
	values := []string{}
	seen := map[int]bool{}
	for _, process := range listener.Processes {
		if process.Name != "" {
			values = append(values, processName(process.Name)+" (PID "+strconv.Itoa(process.PID)+")")
			seen[process.PID] = true
		}
	}
	for _, pid := range listener.PIDs {
		if !seen[pid] {
			values = append(values, "PID "+strconv.Itoa(pid)+" (name unavailable)")
		}
	}
	if len(values) == 0 {
		return "unknown"
	}
	return strings.Join(values, ", ")
}
func auditBindingSummary(listener audit.Listener) string {
	family := listener.AddressFamily
	switch family {
	case "ipv4":
		family = "IPv4"
	case "ipv6":
		family = "IPv6"
	case "ipv4-mapped-ipv6":
		family = "mapped IPv4"
	}
	binding := listener.BindingScope + " / " + family
	if listener.BindingScope == "wildcard" {
		binding = "all local " + family + " interfaces"
	}
	return binding + " | " + listener.State
}
func auditShortAction(finding audit.Finding) string {
	switch finding.RuleID {
	case "wildcard-binding":
		return "Confirm access beyond localhost is intended; review binding and network controls."
	case "sensitive-service-port":
		return "Verify service identity, intended clients, authentication and encryption."
	case "loopback-binding":
		return "Keep loopback for local clients; local processes can still connect."
	case "specific-binding":
		return "Confirm the address, intended clients and network controls."
	default:
		return finding.Recommendation
	}
}
func auditRuleValues(values []string, protocol bool) string {
	if len(values) == 0 {
		return "unresolved"
	}
	display := make([]string, len(values))
	for i, value := range values {
		switch strings.ToLower(value) {
		case "any", "*":
			value = "any"
		default:
			if protocol {
				switch value {
				case "6":
					value = "TCP"
				case "17":
					value = "UDP"
				case "256":
					value = "any"
				default:
					value = strings.ToUpper(value)
				}
			}
		}
		display[i] = value
	}
	return strings.Join(display, ",")
}
func auditProgramName(program string) string {
	if strings.EqualFold(program, "any") || program == "*" {
		return "any"
	}
	if separator := strings.LastIndexAny(program, "/\\"); separator >= 0 && separator < len(program)-1 {
		return program[separator+1:] + " (path in --details)"
	}
	return program
}
func auditSourceName(name string) string {
	switch name {
	case "windows-active-store":
		return "Windows NetSecurity"
	case "windows-policy-com":
		return "Windows COM fallback"
	case "nftables":
		return "Linux nftables"
	case "iptables-save":
		return "Linux iptables (IPv4)"
	case "ip6tables-save":
		return "Linux iptables (IPv6)"
	case "macos-pf-status":
		return "macOS PF status"
	case "macos-pf-rules":
		return "macOS PF rules"
	default:
		return name
	}
}
