package platform

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/Thuya-Myint/socklens/internal/firewall"
)

const maxPolicyOutput = 8 << 20

type boundedPolicyBuffer struct {
	bytes.Buffer
	overflow bool
}

func (b *boundedPolicyBuffer) Write(data []byte) (int, error) {
	if len(data) > maxPolicyOutput-b.Len() {
		b.overflow = true
		return 0, fmt.Errorf("firewall query output exceeds 8 MiB")
	}
	return b.Buffer.Write(data)
}
func runPolicy(ctx context.Context, path string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, path, args...)
	command.Env = policyEnvironment()
	preparePolicy(command)
	command.WaitDelay = time.Second
	var out, stderr boundedPolicyBuffer
	command.Stdout = &out
	command.Stderr = &stderr
	err := command.Run()
	if out.overflow || stderr.overflow {
		return nil, fmt.Errorf("firewall query output exceeds 8 MiB")
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("local firewall query stopped: %w", ctx.Err())
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		lower := strings.ToLower(detail)
		if strings.Contains(lower, "access is denied") || strings.Contains(lower, "permission denied") || strings.Contains(lower, "operation not permitted") {
			return nil, fmt.Errorf("permission denied while reading local firewall policy; no elevation attempted")
		}
		if len(detail) > 512 {
			detail = detail[:512]
		}
		return nil, fmt.Errorf("%v: %s", err, detail)
	}
	return out.Bytes(), nil
}
func localInterfaces() ([]firewall.Interface, error) {
	result := []firewall.Interface{}
	interfaces, err := net.Interfaces()
	if err != nil {
		return result, err
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			return result, err
		}
		row := firewall.Interface{Name: iface.Name, Addresses: []string{}}
		for _, address := range addresses {
			row.Addresses = append(row.Addresses, address.String())
		}
		result = append(result, row)
	}
	return result, nil
}
func policyBase() firewall.Snapshot {
	s := firewall.Empty()
	var err error
	s.Interfaces, err = localInterfaces()
	if err != nil {
		s.Notes = append(s.Notes, "Interface address query incomplete: "+err.Error())
	}
	s.Notes = append(s.Notes, "Read-only candidate policy context, not a packet-filter simulation. No remote reachability was tested.", "Third-party filters, stateful replies, routing, NAT, VPNs, namespaces, and other firewall layers are not fully evaluated.", "Outbound candidates concern traffic from this local endpoint, not every outbound connection owned by the process.")
	return s
}
func policySource(s *firewall.Snapshot, name string, data []byte, err error) bool {
	if err != nil {
		s.Sources = append(s.Sources, firewall.Source{Name: name, Status: "unavailable", Detail: err.Error()})
		return false
	}
	s.Sources = append(s.Sources, firewall.Source{Name: name, Status: "read", Detail: "Local read-only query succeeded; coverage remains partial."})
	s.Coverage = "partial"
	return true
}
