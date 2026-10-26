//go:build windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"unsafe"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var queryImage = kernel32.NewProc("QueryFullProcessImageNameW")
var getSystemDirectory = kernel32.NewProc("GetSystemDirectoryW")
var ipOnce sync.Once
var ipDLL *syscall.DLL
var ipError error

func loadIPHelper() (*syscall.DLL, error) {
	ipOnce.Do(func() {
		var directory [32768]uint16
		n, _, err := getSystemDirectory.Call(uintptr(unsafe.Pointer(&directory[0])), uintptr(len(directory)))
		if n == 0 || n >= uintptr(len(directory)) {
			ipError = fmt.Errorf("locate Windows system directory: %v", err)
			return
		}
		ipDLL, ipError = syscall.LoadDLL(filepath.Join(syscall.UTF16ToString(directory[:n]), "iphlpapi.dll"))
	})
	return ipDLL, ipError
}

func windowsTable(proc *syscall.Proc, family, tableClass uintptr) ([]byte, error) {
	size := uint32(4096)
	for attempt := 0; attempt < 8; attempt++ {
		if size < 4 || size > 64<<20 {
			return nil, fmt.Errorf("invalid Windows socket table size")
		}
		data := make([]byte, size)
		code, _, _ := proc.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&size)), 0, family, tableClass, 0)
		if code == 0 {
			if int(size) > len(data) {
				return nil, fmt.Errorf("Windows returned oversized socket data")
			}
			return data[:size], nil
		}
		if code == 232 {
			return make([]byte, 4), nil
		} // ERROR_NO_DATA.
		if code != 122 {
			return nil, syscall.Errno(code)
		} // ERROR_INSUFFICIENT_BUFFER.
		if size <= uint32(len(data)) {
			size = uint32(len(data)) + 4096
		}
	}
	return nil, fmt.Errorf("socket table kept growing; retry")
}

func List() ([]Socket, error) {
	dll, err := loadIPHelper()
	if err != nil {
		return nil, err
	}
	result := make([]Socket, 0)
	names := map[int]string{}
	for _, protocol := range []string{"tcp", "udp"} {
		name, class := "GetExtendedTcpTable", uintptr(5)
		if protocol == "udp" {
			name = "GetExtendedUdpTable"
			class = 1
		}
		proc, err := dll.FindProc(name)
		if err != nil {
			return nil, err
		}
		for _, ipv6 := range []bool{false, true} {
			family := uintptr(2)
			if ipv6 {
				family = 23
			}
			data, err := windowsTable(proc, family, class)
			if err != nil {
				return nil, fmt.Errorf("%s family %d: %w", name, family, err)
			}
			rows, err := parseWindowsTable(data, protocol, ipv6)
			if err != nil {
				return nil, err
			}
			for i := range rows {
				for _, pid := range rows[i].PIDs {
					name, ok := names[pid]
					if !ok {
						handle, err := syscall.OpenProcess(0x1000, false, uint32(pid))
						if err == nil {
							name, _ = windowsProcessName(handle)
							syscall.CloseHandle(handle)
						}
						names[pid] = name
					}
					rows[i].Processes = append(rows[i].Processes, Process{PID: pid, Name: name})
				}
			}
			result = append(result, rows...)
		}
	}
	sortSockets(result)
	return result, nil
}

func windowsProcessName(handle syscall.Handle) (string, error) {
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	ok, _, err := queryImage.Call(uintptr(handle), 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return "", err
	}
	return filepath.Base(syscall.UTF16ToString(buffer[:size])), nil
}

func windowsOwner(handle syscall.Handle) (string, error) {
	var token syscall.Token
	if err := syscall.OpenProcessToken(handle, syscall.TOKEN_QUERY, &token); err != nil {
		return "", err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String()
}

func windowsIdentity(handle syscall.Handle, pid int) (Identity, error) {
	if pid <= 4 || uint64(pid) > uint64(^uint32(0)) || pid == os.Getpid() {
		return Identity{}, fmt.Errorf("refusing protected PID %d", pid)
	}
	owner, err := windowsOwner(handle)
	if err != nil {
		return Identity{}, err
	}
	current, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return Identity{}, err
	}
	defer current.Close()
	user, err := current.GetTokenUser()
	if err != nil {
		return Identity{}, err
	}
	me, err := user.User.Sid.String()
	if err != nil {
		return Identity{}, err
	}
	if owner != me {
		return Identity{}, fmt.Errorf("PID %d is not owned by the current user", pid)
	}
	status, err := syscall.WaitForSingleObject(handle, 0)
	if err != nil {
		return Identity{}, err
	}
	if status != syscall.WAIT_TIMEOUT {
		return Identity{}, fmt.Errorf("PID %d has exited", pid)
	}
	var created, exited, kernel, userTime syscall.Filetime
	if err := syscall.GetProcessTimes(handle, &created, &exited, &kernel, &userTime); err != nil {
		return Identity{}, err
	}
	name, err := windowsProcessName(handle)
	if err != nil {
		return Identity{}, err
	}
	start := uint64(created.HighDateTime)<<32 | uint64(created.LowDateTime)
	return Identity{PID: pid, Start: strconv.FormatUint(start, 10), Name: name}, nil
}

func Identify(pid int) (Identity, error) {
	if pid <= 4 || uint64(pid) > uint64(^uint32(0)) || pid == os.Getpid() {
		return Identity{}, fmt.Errorf("refusing protected PID %d", pid)
	}
	handle, err := syscall.OpenProcess(0x1000|0x100000, false, uint32(pid))
	if err != nil {
		return Identity{}, err
	}
	defer syscall.CloseHandle(handle)
	return windowsIdentity(handle, pid)
}

type windowsTarget struct {
	handle syscall.Handle
	id     Identity
}

func OpenTarget(pid int) (Target, error) {
	if pid <= 4 || uint64(pid) > uint64(^uint32(0)) || pid == os.Getpid() {
		return nil, fmt.Errorf("refusing protected PID %d", pid)
	}
	handle, err := syscall.OpenProcess(0x1|0x1000|0x100000, false, uint32(pid))
	if err != nil {
		return nil, err
	}
	id, err := windowsIdentity(handle, pid)
	if err != nil {
		syscall.CloseHandle(handle)
		return nil, err
	}
	return &windowsTarget{handle: handle, id: id}, nil
}
func (t *windowsTarget) Identity() Identity { return t.id }
func (t *windowsTarget) Close() error {
	if t.handle == 0 {
		return nil
	}
	handle := t.handle
	t.handle = 0
	return syscall.CloseHandle(handle)
}
func (t *windowsTarget) Terminate() error {
	if t.handle == 0 {
		return fmt.Errorf("process handle is closed")
	}
	current, err := windowsIdentity(t.handle, t.id.PID)
	if err != nil {
		return err
	}
	if current.Start != t.id.Start {
		return fmt.Errorf("process identity changed")
	}
	return syscall.TerminateProcess(t.handle, 1)
}
func Terminate(id Identity) error {
	target, err := OpenTarget(id.PID)
	if err != nil {
		return err
	}
	defer target.Close()
	if target.Identity().Start != id.Start {
		return fmt.Errorf("process identity changed")
	}
	return target.Terminate()
}
