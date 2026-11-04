//go:build windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

// Toolhelp reports executable names without opening each process for its path.
// It is display metadata only, never a termination or executable-path fallback.
func windowsSnapshotNames() (map[int]string, error) {
	snapshot, err := syscall.CreateToolhelp32Snapshot(0x2, 0) // TH32CS_SNAPPROCESS.
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(snapshot)
	result := map[int]string{}
	entry := syscall.ProcessEntry32{Size: uint32(unsafe.Sizeof(syscall.ProcessEntry32{}))}
	err = syscall.Process32First(snapshot, &entry)
	for count := 0; err == nil && count < 1<<20; count++ {
		result[int(entry.ProcessID)] = syscall.UTF16ToString(entry.ExeFile[:])
		entry = syscall.ProcessEntry32{Size: uint32(unsafe.Sizeof(syscall.ProcessEntry32{}))}
		err = syscall.Process32Next(snapshot, &entry)
	}
	if err != nil && !errors.Is(err, syscall.ERROR_NO_MORE_FILES) {
		return result, err
	}
	return result, nil
}

var serviceDLL = syscall.NewLazyDLL("advapi32.dll")
var openServiceManager = serviceDLL.NewProc("OpenSCManagerW")
var enumerateServiceStatus = serviceDLL.NewProc("EnumServicesStatusExW")
var closeServiceManager = serviceDLL.NewProc("CloseServiceHandle")

func CollectServices(ctx context.Context, rows []Socket) *ServiceSnapshot {
	snapshot := &ServiceSnapshot{Status: "unavailable", Source: "windows-scm", Detail: "Service metadata is unavailable.", Services: []Service{}}
	wanted := map[int]bool{}
	for _, row := range rows {
		for _, pid := range row.PIDs {
			if pid > 0 {
				wanted[pid] = true
			}
		}
	}
	if len(wanted) == 0 {
		snapshot.Status = "read"
		snapshot.Detail = "No owning PIDs selected; no service query needed."
		return snapshot
	}
	if ctx.Err() != nil {
		snapshot.Detail = ctx.Err().Error()
		return snapshot
	}
	manager, _, err := openServiceManager.Call(0, 0, 0x4) // SC_MANAGER_ENUMERATE_SERVICE only.
	if manager == 0 {
		snapshot.Detail = fmt.Sprintf("Read Windows service names: %v", err)
		return snapshot
	}
	defer closeServiceManager.Call(manager)
	var resume uint32
	for page := 0; page < 256; page++ {
		if ctx.Err() != nil {
			snapshot.Detail = ctx.Err().Error()
			if len(snapshot.Services) > 0 {
				snapshot.Status = "partial"
			}
			return snapshot
		}
		data := make([]byte, 256<<10)
		var needed, count uint32
		previous := resume
		ok, _, err := enumerateServiceStatus.Call(manager, 0, 0x30, 0x3, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&resume)), 0)
		if ok == 0 && !errors.Is(err, syscall.Errno(234)) {
			snapshot.Detail = fmt.Sprintf("Read Windows service status: %v", err)
			if len(snapshot.Services) > 0 {
				snapshot.Status = "partial"
			}
			return snapshot
		}
		services, decodeErr := parseWindowsServices(data, uintptr(unsafe.Pointer(&data[0])), count, int(unsafe.Sizeof(uintptr(0))))
		if decodeErr != nil {
			snapshot.Detail = decodeErr.Error()
			if len(snapshot.Services) > 0 {
				snapshot.Status = "partial"
			}
			return snapshot
		}
		for _, service := range services {
			if wanted[service.PID] {
				snapshot.Services = append(snapshot.Services, service)
			}
		}
		if ok != 0 {
			snapshot.Status = "read"
			snapshot.Detail = "Best-effort service-to-PID snapshot; permissions may silently omit services. Shared-host entries do not identify the service owning a particular socket."
			return snapshot
		}
		if resume == 0 || resume == previous {
			snapshot.Detail = "Windows service enumeration made no progress."
			if len(snapshot.Services) > 0 {
				snapshot.Status = "partial"
			}
			return snapshot
		}
	}
	snapshot.Detail = "Windows service enumeration exceeded the page limit."
	if len(snapshot.Services) > 0 {
		snapshot.Status = "partial"
	}
	return snapshot
}
