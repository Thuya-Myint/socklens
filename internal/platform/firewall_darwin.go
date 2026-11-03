//go:build darwin

package platform

import (
	"bytes"
	"context"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/Thuya-Myint/socklens/internal/firewall"
)

func CollectFirewall(ctx context.Context, rows []Socket) firewall.Snapshot {
	s := policyBase()
	tool := "/usr/libexec/ApplicationFirewall/socketfilterfw"
	for _, flag := range []string{"--getglobalstate", "--getblockall", "--getallowsigned", "--listapps"} {
		data, err := runPolicy(ctx, tool, flag)
		if policySource(&s, "macos-application-firewall "+flag, data, err) {
			if flag == "--listapps" {
				decodeALF(data, &s)
			} else {
				s.Profiles = append(s.Profiles, firewall.Profile{Name: "Application Firewall " + flag, Enabled: "see detail", Inbound: "unknown", Outbound: "not covered by Application Firewall", Detail: string(bytes.TrimSpace(data))})
			}
		}
	}
	data, err := runPolicy(ctx, "/sbin/pfctl", "-s", "info")
	if policySource(&s, "macos-pf-status", data, err) {
		s.Profiles = append(s.Profiles, firewall.Profile{Name: "PF", Enabled: "see detail", Inbound: "unknown", Outbound: "unknown", Detail: string(bytes.TrimSpace(data))})
	}
	data, err = runPolicy(ctx, "/sbin/pfctl", "-a", "*", "-sr")
	if policySource(&s, "macos-pf-rules", data, err) {
		decodePF(data, &s)
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return s
		}
		for _, pid := range row.PIDs {
			if _, ok := s.Paths[pid]; ok {
				continue
			}
			s.Paths[pid] = ""
			if pid <= 0 || uint64(pid) > 1<<31-1 {
				continue
			}
			buffer := make([]byte, 4096)
			count, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, 2, uintptr(pid), 11, 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
			if errno == 0 && count > 0 && count <= uintptr(len(buffer)) {
				s.Paths[pid] = string(bytes.TrimRight(buffer[:count], "\x00"))
			}
		}
	}
	s.Notes = append(s.Notes, "macOS Application Firewall is application-based and inbound-focused; PF and Network Extension filters are separate layers. PF syntax/anchors and signed-app exemptions are not simulated. Privileged queries may be unavailable; no elevation is attempted.", "PF identifiers are snapshot line numbers, not persistent rule handles: "+strconv.Itoa(len(s.Rules))+" candidate records collected.")
	return s
}
