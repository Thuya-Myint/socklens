//go:build linux || darwin

package terminal

import (
	"os"
	"syscall"
	"unsafe"
)

func windowSize(file *os.File) ([4]uint16, bool) {
	var size [4]uint16
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&size)))
	return size, err == 0
}
func isTerminal(file *os.File) bool { _, ok := windowSize(file); return ok }
func terminalWidth(file *os.File) int {
	size, ok := windowSize(file)
	if !ok {
		return 0
	}
	return int(size[1])
}
func enableColor(file *os.File) bool { return isTerminal(file) }
