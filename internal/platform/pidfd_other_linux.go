//go:build linux && !amd64 && !arm64

package platform

import (
	"fmt"
	"syscall"
)

func pidfdOpen(pid int) (int, error) {
	return -1, fmt.Errorf("%w: termination currently supports Linux amd64 and arm64; no signal sent", ErrSafeTerminationUnsupported)
}

func pidfdSignal(fd int, signal syscall.Signal) error {
	return fmt.Errorf("%w: termination currently supports Linux amd64 and arm64; no signal sent", ErrSafeTerminationUnsupported)
}
