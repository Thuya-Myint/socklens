//go:build darwin

package platform

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// rawSysctl preserves binary data; syscall.Sysctl removes a trailing zero byte.
func rawSysctl(name string) ([]byte, error) {
	query := [2]int32{0, 3} // CTL_SYSCTL / name-to-OID, as used by Go's syscall package.
	var mib [26]int32
	length := uintptr(24 * 4)
	key := []byte(name)
	_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&query[0])), 2, uintptr(unsafe.Pointer(&mib[0])), uintptr(unsafe.Pointer(&length)), uintptr(unsafe.Pointer(&key[0])), uintptr(len(key)))
	if errno != 0 {
		return nil, errno
	}
	if length == 0 || length%4 != 0 || length > 24*4 {
		return nil, fmt.Errorf("invalid macOS sysctl OID")
	}
	count := length / 4
	for attempt := 0; attempt < 8; attempt++ {
		size := uintptr(0)
		_, _, errno = syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), count, 0, uintptr(unsafe.Pointer(&size)), 0, 0)
		if errno != 0 {
			return nil, errno
		}
		if size > 64<<20 {
			return nil, fmt.Errorf("macOS socket table is too large")
		}
		data := make([]byte, size+4096)
		size = uintptr(len(data))
		_, _, errno = syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), count, uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&size)), 0, 0)
		if errno == syscall.ENOMEM {
			continue
		}
		if errno != 0 {
			return nil, errno
		}
		if size > uintptr(len(data)) {
			return nil, fmt.Errorf("oversized macOS sysctl result")
		}
		return data[:size], nil
	}
	return nil, fmt.Errorf("macOS socket table kept growing; retry")
}

func List() ([]Socket, error) {
	result := make([]Socket, 0)
	names := map[int]string{}
	for _, protocol := range []string{"tcp", "udp"} {
		data, err := rawSysctl("net.inet." + protocol + ".pcblist_n")
		if err != nil {
			return nil, fmt.Errorf("macOS %s socket query: %w", protocol, err)
		}
		rows, err := parseDarwinPCBList(data, protocol)
		if err != nil {
			return nil, err
		}
		for i := range rows {
			for _, pid := range rows[i].PIDs {
				name, ok := names[pid]
				if !ok {
					info, err := darwinProcessInfo(pid)
					if err == nil {
						name = darwinName(info)
					}
					names[pid] = name
				}
				rows[i].Processes = append(rows[i].Processes, Process{PID: pid, Name: name})
			}
		}
		result = append(result, rows...)
	}
	sortSockets(result)
	return result, nil
}

// PROC_PIDT_BSDINFOWITHUNIQID returns BSD identity and PID version atomically.
func darwinProcessInfo(pid int) ([]byte, error) {
	data := make([]byte, 192)
	count, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, 2, uintptr(pid), 18, 0, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	if errno != 0 {
		return nil, errno
	}
	if count != uintptr(len(data)) {
		return nil, fmt.Errorf("unsupported macOS process identity layout")
	}
	return data, nil
}
func darwinName(data []byte) string {
	name := bytes.TrimRight(data[64:96], "\x00")
	if len(name) == 0 {
		name = bytes.TrimRight(data[48:64], "\x00")
	}
	return string(name)
}
func darwinIdentity(pid int) (Identity, [8]uint32, error) {
	var token [8]uint32
	if pid <= 1 || uint64(pid) > 1<<31-1 || pid == os.Getpid() {
		return Identity{}, token, fmt.Errorf("refusing protected PID %d", pid)
	}
	data, err := darwinProcessInfo(pid)
	if err != nil {
		return Identity{}, token, err
	}
	le := binary.LittleEndian
	if le.Uint32(data[12:16]) != uint32(pid) {
		return Identity{}, token, fmt.Errorf("macOS process identity changed")
	}
	if le.Uint32(data[20:24]) != uint32(os.Geteuid()) {
		return Identity{}, token, fmt.Errorf("PID %d is not owned by the current user", pid)
	}
	token[1] = le.Uint32(data[20:24])
	token[2] = le.Uint32(data[24:28])
	token[3] = le.Uint32(data[28:32])
	token[4] = le.Uint32(data[32:36])
	token[5] = uint32(pid)
	token[7] = le.Uint32(data[168:172])
	start := fmt.Sprintf("%d.%d-%x-%d", le.Uint64(data[120:128]), le.Uint64(data[128:136]), le.Uint64(data[152:160]), token[7])
	return Identity{PID: pid, Name: darwinName(data), Start: start}, token, nil
}
func Identify(pid int) (Identity, error) { id, _, err := darwinIdentity(pid); return id, err }

func darwinSignal(token [8]uint32, signal syscall.Signal) error {
	// PROC_INFO_CALL_SIGNAL_AUDITTOKEN. Unlike PID-only kill, the kernel
	// compares the captured PID version. Never use sudden-termination APIs.
	_, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, 0x11, 0, uintptr(signal), 0, uintptr(unsafe.Pointer(&token[0])), 32)
	if errno != 0 {
		if errno == syscall.ENOSYS || errno == syscall.EINVAL {
			return fmt.Errorf("%w: macOS identity-checked signaling is unavailable; no signal sent", ErrSafeTerminationUnsupported)
		}
		return errno
	}
	return nil
}

type darwinTarget struct {
	id     Identity
	token  [8]uint32
	closed bool
}

func OpenTarget(pid int) (Target, error) {
	id, token, err := darwinIdentity(pid)
	if err != nil {
		return nil, err
	}
	if err := darwinSignal(token, 0); err != nil {
		return nil, err
	}
	return &darwinTarget{id: id, token: token}, nil
}
func (t *darwinTarget) Identity() Identity { return t.id }
func (t *darwinTarget) Close() error       { t.closed = true; return nil }
func (t *darwinTarget) Terminate() error {
	if t.closed {
		return fmt.Errorf("process target is closed")
	}
	current, err := Identify(t.id.PID)
	if err != nil {
		return err
	}
	if current.Start != t.id.Start {
		return fmt.Errorf("process identity changed")
	}
	return darwinSignal(t.token, syscall.SIGTERM)
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
