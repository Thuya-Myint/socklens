//go:build linux

package platform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Thuya-Myint/socklens/internal/firewall"
)

func systemPolicyTool(name string) string {
	for _, dir := range []string{"/usr/sbin", "/sbin", "/usr/bin", "/bin"} {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return path
		}
	}
	return ""
}
func CollectFirewall(ctx context.Context, rows []Socket) firewall.Snapshot {
	s := policyBase()
	path := systemPolicyTool("nft")
	nftRead := false
	if path != "" {
		data, err := runPolicy(ctx, path, "-j", "list", "ruleset")
		if policySource(&s, "nftables", data, err) {
			if err := decodeNFT(data, &s); err != nil {
				s.Notes = append(s.Notes, "nftables decoding failed: "+err.Error())
				s.Sources[len(s.Sources)-1].Status = "unavailable"
			} else {
				nftRead = true
			}
		}
	} else {
		policySource(&s, "nftables", nil, fmt.Errorf("nft is not installed in system directories"))
	}
	if !nftRead {
		for _, tool := range []string{"iptables-save", "ip6tables-save"} {
			path := systemPolicyTool(tool)
			if path == "" {
				policySource(&s, tool, nil, fmt.Errorf("%s is not installed in system directories", tool))
				continue
			}
			data, err := runPolicy(ctx, path, "-t", "filter")
			if policySource(&s, tool, data, err) {
				decodeIPTables(data, tool, &s)
			}
		}
	}
	if !hasPolicyRead(s.Sources) {
		s.Coverage = "unknown"
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return s
		}
		for _, pid := range row.PIDs {
			if _, ok := s.Paths[pid]; ok {
				continue
			}
			s.Paths[pid], _ = os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
		}
	}
	s.Notes = append(s.Notes, "Linux coverage is limited to the current network namespace. Legacy iptables alongside nftables, eBPF, other hooks/tables, container/host policies and rule traversal are not fully evaluated. WSL does not expose the Windows host firewall here.")
	return s
}
func hasPolicyRead(sources []firewall.Source) bool {
	for _, source := range sources {
		if source.Status == "read" {
			return true
		}
	}
	return false
}
