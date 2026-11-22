package cli

import (
	"net/netip"
	"strconv"
	"strings"

	"github.com/Thuya-Myint/socklens/internal/audit"
)

// These summaries explain observations and evidence gaps, never a security or
// reachability verdict. JSON and the detailed evidence are not changed.
func auditExposureSummary(listener audit.Listener) string {
	switch listener.BindingScope {
	case "loopback":
		return "Loopback-bound; direct socket access is local. Proxies or forwarding are not evaluated."
	case "wildcard":
		return "All interfaces in this address family; remote reachability is not established."
	case "private":
		return "Private-address binding; private does not mean trusted or unreachable."
	case "link-local":
		return "Link-local binding; interface scope and network controls still apply."
	case "multicast":
		return "Multicast-address binding; group membership and delivery are not evaluated."
	default:
		return "Specific-address binding; remote reachability is not established."
	}
}

func auditUnknownSummary(listener audit.Listener) string {
	unknown := []string{}
	if listener.ProcessVisibility != "observed" {
		unknown = append(unknown, "owner metadata incomplete")
	}
	if listener.Firewall == nil {
		unknown = append(unknown, "firewall not collected")
	} else {
		coverage := listener.Firewall.Coverage
		if coverage == "" {
			coverage = "unknown"
		}
		unknown = append(unknown, "traffic decision not established ("+coverage+" firewall coverage)")
	}
	return strings.Join(unknown, "; ")
}

// auditFirstCheck offers at most one passive, port-scoped command. It contains
// only a validated protocol and numeric port, never process names, addresses,
// peer values, paths or other untrusted strings that might become shell code.
// The user chooses whether to run it; rendering never executes commands.
func auditFirstCheck(listener audit.Listener) (action, command string) {
	endpoint, err := netip.ParseAddrPort(listener.Local)
	protocol := strings.ToLower(listener.Protocol)
	if err != nil || endpoint.Port() == 0 || (protocol != "tcp" && protocol != "udp") {
		return "", ""
	}
	port := strconv.Itoa(int(endpoint.Port()))
	inspect := "socklens inspect --protocol " + protocol + " " + port
	if listener.ProcessVisibility != "observed" {
		return "Identify the owner first; missing names may require additional OS permissions.", inspect
	}
	if listener.Firewall != nil && listener.Firewall.Coverage != "complete" {
		return "Review firewall evidence and source restrictions before judging access.", "socklens audit --protocol " + protocol + " --port " + port + " --details"
	}
	return "", inspect
}
