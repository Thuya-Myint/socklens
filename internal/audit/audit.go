// Package audit explains local listener observations without network probes.
package audit

import (
	"fmt"
	"net/netip"
	"slices"
	"sort"
	"time"

	"github.com/Thuya-Myint/socklens/internal/firewall"
	"github.com/Thuya-Myint/socklens/internal/platform"
)

const SchemaVersion = 1

type Finding struct {
	RuleID         string   `json:"rule_id"`
	Level          string   `json:"level"`
	Confidence     string   `json:"confidence"`
	Title          string   `json:"title"`
	Evidence       []string `json:"evidence"`
	Recommendation string   `json:"recommendation"`
}

type Process struct {
	PID  int    `json:"pid"`
	Name string `json:"name"`
}

type ServiceHint struct {
	Name     string `json:"name"`
	Source   string `json:"source"`
	Verified bool   `json:"verified"`
}

type Listener struct {
	Services          []platform.Service `json:"services,omitempty"`
	Firewall          *firewall.Context  `json:"firewall,omitempty"`
	Protocol          string             `json:"protocol"`
	Local             string             `json:"local"`
	State             string             `json:"state"`
	SocketID          string             `json:"socket_id,omitempty"`
	AddressFamily     string             `json:"address_family"`
	BindingScope      string             `json:"binding_scope"`
	PIDs              []int              `json:"pids"`
	Processes         []Process          `json:"processes"`
	ProcessVisibility string             `json:"process_visibility"`
	ServiceHint       *ServiceHint       `json:"service_hint,omitempty"`
	Findings          []Finding          `json:"findings"`
}

type Summary struct {
	Listeners             int `json:"listeners"`
	ReviewListeners       int `json:"review_listeners"`
	InformationListeners  int `json:"information_listeners"`
	Findings              int `json:"findings"`
	MissingProcessDetails int `json:"missing_process_details"`
}

type Report struct {
	ServiceLookup        *platform.ServiceSnapshot `json:"service_lookup,omitempty"`
	Firewall             *firewall.Snapshot        `json:"firewall,omitempty"`
	SchemaVersion        int                       `json:"schema_version"`
	Timestamp            time.Time                 `json:"timestamp"`
	Scope                string                    `json:"scope"`
	ExternalReachability string                    `json:"external_reachability"`
	Summary              Summary                   `json:"summary"`
	Listeners            []Listener                `json:"listeners"`
	Notes                []string                  `json:"notes"`
}

// Analyze is a pure snapshot transformation. It neither probes sockets nor
// looks up hostnames, configurations, service versions, or vulnerability data.
func Analyze(rows []platform.Socket, observedAt time.Time) (Report, error) {
	report := Report{SchemaVersion: SchemaVersion, Timestamp: observedAt.UTC(), Scope: "local", ExternalReachability: "not-tested", Listeners: []Listener{}, Notes: []string{
		"Local socket-table snapshot; external reachability was not tested.",
		"Review items are configuration questions, not confirmed vulnerabilities. Information items are not safety verdicts.",
		"Service hints use port conventions and are unverified; authentication, TLS, and vulnerabilities were not tested.",
		"Process visibility depends on OS permissions. Firewalls, routing, NAT, and network namespaces affect reachability.",
	}}
	for _, row := range rows {
		if !row.IsListener() {
			continue
		}
		endpoint, err := netip.ParseAddrPort(row.Local)
		if err != nil {
			return Report{}, fmt.Errorf("invalid local listener endpoint %q: %w", row.Local, err)
		}
		address := endpoint.Addr()
		family := "ipv6"
		if address.Is4() {
			family = "ipv4"
		} else if address.Is4In6() {
			family = "ipv4-mapped-ipv6"
		}
		scope := bindingScope(address.Unmap())
		listener := Listener{Protocol: row.Protocol, Local: row.Local, State: row.State, SocketID: row.Inode, AddressFamily: family, BindingScope: scope, PIDs: append([]int{}, row.PIDs...), Processes: []Process{}, Findings: []Finding{}}
		sort.Ints(listener.PIDs)
		listener.PIDs = slices.Compact(listener.PIDs)
		names := map[int]bool{}
		for _, process := range row.Processes {
			listener.Processes = append(listener.Processes, Process{PID: process.PID, Name: process.Name})
			if process.Name != "" {
				names[process.PID] = true
			}
		}
		sort.Slice(listener.Processes, func(i, j int) bool {
			a, b := listener.Processes[i], listener.Processes[j]
			if a.PID != b.PID {
				return a.PID < b.PID
			}
			return a.Name < b.Name
		})
		listener.Processes = slices.Compact(listener.Processes)
		listener.ProcessVisibility = "observed"
		if len(listener.PIDs) == 0 {
			listener.ProcessVisibility = "unknown"
		} else {
			for _, pid := range listener.PIDs {
				if !names[pid] {
					listener.ProcessVisibility = "partial"
					break
				}
			}
		}
		listener.Findings = append(listener.Findings, bindingFinding(listener, address))
		if name := serviceName(row.Protocol, endpoint.Port()); name != "" {
			listener.ServiceHint = &ServiceHint{Name: name, Source: "port-convention", Verified: false}
			if scope != "loopback" {
				listener.Findings = append(listener.Findings, Finding{
					RuleID: "sensitive-service-port", Level: "review", Confidence: "heuristic",
					Title:          "Non-loopback listener on a database or administration service port",
					Evidence:       []string{fmt.Sprintf("%s local endpoint %s has binding scope %s.", row.Protocol, row.Local, scope), fmt.Sprintf("Port %d is commonly associated with %s; the actual service was not identified.", endpoint.Port(), name)},
					Recommendation: "Identify the actual service. If the hint applies, review intended clients, network access controls, authentication, and encryption in its configuration.",
				})
			}
		}
		// Within each record, put the review question before background information.
		sort.SliceStable(listener.Findings, func(i, j int) bool {
			return listener.Findings[i].Level == "review" && listener.Findings[j].Level != "review"
		})
		review := false
		for _, finding := range listener.Findings {
			if finding.Level == "review" {
				review = true
			}
		}
		report.Summary.Listeners++
		report.Summary.Findings += len(listener.Findings)
		if review {
			report.Summary.ReviewListeners++
		} else {
			report.Summary.InformationListeners++
		}
		if listener.ProcessVisibility != "observed" {
			report.Summary.MissingProcessDetails++
		}
		report.Listeners = append(report.Listeners, listener)
	}
	sort.SliceStable(report.Listeners, func(i, j int) bool {
		a, b := report.Listeners[i], report.Listeners[j]
		if ap, bp := readingPriority(a), readingPriority(b); ap != bp {
			return ap < bp
		}
		// Within a reading group, broader bindings come first. This is an
		// explanation order, not a claim about severity or reachability.
		if (a.BindingScope == "wildcard") != (b.BindingScope == "wildcard") {
			return a.BindingScope == "wildcard"
		}
		// Analyze has already validated numeric endpoints. Compare numeric ports
		// and addresses so port 80 precedes 443 rather than sorting as strings.
		ae, _ := netip.ParseAddrPort(a.Local)
		be, _ := netip.ParseAddrPort(b.Local)
		if ae.Port() != be.Port() {
			return ae.Port() < be.Port()
		}
		if a.Protocol != b.Protocol {
			return a.Protocol < b.Protocol
		}
		if order := ae.Addr().Compare(be.Addr()); order != 0 {
			return order < 0
		}
		if a.Local != b.Local {
			return a.Local < b.Local
		}
		if a.SocketID != b.SocketID {
			return a.SocketID < b.SocketID
		}
		if order := slices.Compare(a.PIDs, b.PIDs); order != 0 {
			return order < 0
		}
		if len(a.Processes) != len(b.Processes) {
			return len(a.Processes) < len(b.Processes)
		}
		for k := range a.Processes {
			if a.Processes[k].PID != b.Processes[k].PID {
				return a.Processes[k].PID < b.Processes[k].PID
			}
			if a.Processes[k].Name != b.Processes[k].Name {
				return a.Processes[k].Name < b.Processes[k].Name
			}
		}
		return a.State < b.State
	})
	return report, nil
}

// readingPriority helps people find configuration questions before background
// information. Port hints remain heuristic; this is not a vulnerability score.
func readingPriority(listener Listener) int {
	for _, finding := range listener.Findings {
		if finding.RuleID == "sensitive-service-port" {
			return 0
		}
	}
	for _, finding := range listener.Findings {
		if finding.Level == "review" {
			return 1
		}
	}
	if listener.BindingScope != "loopback" {
		return 2
	}
	return 3
}

func bindingScope(address netip.Addr) string {
	switch {
	case address.IsUnspecified():
		return "wildcard"
	case address.IsLoopback():
		return "loopback"
	case address.IsPrivate():
		return "private"
	case address.IsLinkLocalUnicast():
		return "link-local"
	case address.IsMulticast():
		return "multicast"
	default:
		return "specific-address"
	}
}

func bindingFinding(listener Listener, address netip.Addr) Finding {
	evidence := []string{fmt.Sprintf("%s local endpoint %s was observed in state %s.", listener.Protocol, listener.Local, listener.State)}
	finding := Finding{RuleID: "specific-binding", Level: "information", Confidence: "observed", Title: "Listener bound to a specific address", Evidence: evidence, Recommendation: "Confirm this address and its intended clients match the service's purpose. Verify network controls separately."}
	switch listener.BindingScope {
	case "wildcard":
		finding.RuleID = "wildcard-binding"
		finding.Level = "review"
		family := "IPv6"
		if address.Is4() || address.Is4In6() {
			family = "IPv4"
		}
		finding.Title = "Listener bound to all local " + family + " interfaces"
		finding.Evidence = append(finding.Evidence, "The address is a wildcard binding; external reachability was not tested.")
		if !address.Is4() && !address.Is4In6() {
			finding.Evidence = append(finding.Evidence, "IPv4 acceptance by this IPv6 socket was not tested.")
		}
		finding.Recommendation = "Confirm whether access beyond localhost is intended. If access is only local, consider a loopback binding; otherwise review intended interfaces and network access controls."
	case "loopback":
		finding.RuleID = "loopback-binding"
		finding.Title = "Listener bound to a loopback address"
		finding.Recommendation = "Keep this binding if access is intended only from the local environment. Local processes can still connect; application security requires separate checks."
	case "private":
		finding.Title = "Listener bound to a private-network address"
	case "link-local":
		finding.Title = "Listener bound to a link-local address"
	case "multicast":
		finding.Title = "Socket bound to a multicast address"
		finding.Recommendation = "Confirm the multicast group, intended interfaces, and network access controls match the application's purpose."
	}
	return finding
}

// A small, explicit hint set, not a service fingerprint or a vulnerability list.
// Sources and limitations are documented in docs/audit.md.
func serviceName(protocol string, port uint16) string {
	if protocol == "udp" && port == 161 {
		return "SNMP"
	}
	if protocol != "tcp" {
		return ""
	}
	switch port {
	case 22:
		return "SSH"
	case 3306:
		return "MySQL"
	case 3389:
		return "Remote Desktop"
	case 5432:
		return "PostgreSQL"
	case 6379:
		return "Redis"
	}
	return ""
}
