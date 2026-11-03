//go:build windows

package platform

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/Thuya-Myint/socklens/internal/firewall"
)

func preparePolicy(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
func policyEnvironment() []string { return os.Environ() }
func CollectFirewall(ctx context.Context, rows []Socket) firewall.Snapshot {
	s := policyBase()
	var directory [32768]uint16
	n, _, err := getSystemDirectory.Call(uintptr(unsafe.Pointer(&directory[0])), uintptr(len(directory)))
	if n == 0 || n >= uintptr(len(directory)) {
		policySource(&s, "windows-active-store", nil, err)
		return s
	}
	path := filepath.Join(syscall.UTF16ToString(directory[:n]), "WindowsPowerShell", "v1.0", "powershell.exe")
	data, err := runPolicy(ctx, path, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", windowsFirewallScript)
	if policySource(&s, "windows-active-store", data, err) {
		if err := decodeWindowsFirewall(data, &s); err != nil {
			s.Coverage = "unknown"
			s.Sources[len(s.Sources)-1].Status = "invalid-data"
			s.Rules = []firewall.Rule{}
			s.Notes = append(s.Notes, "Firewall data could not be decoded: "+err.Error())
		}
	}
	if s.Coverage == "unknown" && ctx.Err() == nil {
		data, err := runPolicy(ctx, path, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", windowsCOMFirewallScript)
		if policySource(&s, "windows-policy-com", data, err) {
			if err := decodeWindowsFirewall(data, &s); err != nil {
				s.Coverage = "unknown"
				s.Sources[len(s.Sources)-1].Status = "invalid-data"
				s.Rules = []firewall.Rule{}
				s.Notes = append(s.Notes, "COM firewall data could not be decoded: "+err.Error())
			}
		}
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
			handle, err := syscall.OpenProcess(0x1000, false, uint32(pid))
			if err != nil {
				continue
			}
			buffer := make([]uint16, 32768)
			size := uint32(len(buffer))
			ok, _, _ := queryImage.Call(uintptr(handle), 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))
			syscall.CloseHandle(handle)
			if ok != 0 && size <= uint32(len(buffer)) {
				s.Paths[pid] = syscall.UTF16ToString(buffer[:size])
			}
		}
	}
	return s
}
