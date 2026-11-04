package cli

import (
	"github.com/Thuya-Myint/socklens/internal/audit"
	"net/netip"
	"strconv"
	"strings"
)

// These are socket/binding facts, not simulated firewall verdicts or probes.
func auditCommunication(listener audit.Listener) (incoming, outgoing string) {
	protocol := strings.ToUpper(listener.Protocol)
	scope := "bound to this local address; remote access not established"
	switch listener.BindingScope {
	case "loopback":
		scope = "local applications via loopback"
	case "wildcard":
		scope = "all local interfaces in this address family; remote access not established"
	}
	incoming = protocol + " to " + listener.Local + " | " + scope
	if listener.Protocol == "udp" {
		outgoing = "UDP from " + listener.Local + " | remote peer not reported by this audit snapshot"
	} else {
		command := "socklens inspect --protocol tcp PORT"
		if endpoint, err := netip.ParseAddrPort(listener.Local); err == nil {
			command = "socklens inspect --protocol tcp " + strconv.Itoa(int(endpoint.Port()))
		}
		outgoing = "TCP replies use accepted connections; view connection rows with " + command
	}
	return incoming, outgoing
}
