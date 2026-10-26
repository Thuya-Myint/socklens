//go:build linux && (amd64 || arm64)

package platform

import (
	"fmt"
	"syscall"
)

// These numbers are shared by Linux x86-64 and the arm64 generic syscall ABI.
// https://github.com/torvalds/linux/blob/master/arch/x86/entry/syscalls/syscall_64.tbl
// https://github.com/torvalds/linux/blob/master/include/uapi/asm-generic/unistd.h
const (
	sysPidfdOpen       = 434
	sysPidfdSendSignal = 424
)

func pidfdOpen(pid int) (int, error) {
	fd, _, errno := syscall.Syscall(sysPidfdOpen, uintptr(pid), 0, 0)
	if errno != 0 {
		if errno == syscall.ENOSYS {
			return -1, fmt.Errorf("%w: pidfd_open requires Linux 5.3 or newer; no signal sent", ErrSafeTerminationUnsupported)
		}
		return -1, fmt.Errorf("open process handle: %w", errno)
	}
	return int(fd), nil
}

func pidfdSignal(fd int, signal syscall.Signal) error {
	_, _, errno := syscall.Syscall6(sysPidfdSendSignal, uintptr(fd), uintptr(signal), 0, 0, 0, 0)
	if errno != 0 {
		if errno == syscall.ENOSYS {
			return fmt.Errorf("%w: pidfd_send_signal is unavailable; no signal sent", ErrSafeTerminationUnsupported)
		}
		return fmt.Errorf("signal process handle: %w", errno)
	}
	return nil
}
