//go:build darwin

package terminal

import (
	"bytes"
	"os"
	"syscall"
	"testing"
	"unsafe"
)

// A macOS master is not a fully initialized terminal until its slave has been
// granted, unlocked and opened. Exercise size/color detection on that slave,
// just as a real application receives an initialized terminal from its shell.
func testTerminalSide(t *testing.T, master *os.File) *os.File {
	t.Helper()
	var name [128]byte
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), syscall.TIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0])))
	if err != 0 {
		t.Fatal("get macOS test terminal path:", err)
	}
	end := bytes.IndexByte(name[:], 0)
	if end <= 0 {
		t.Fatal("invalid macOS test terminal path")
	}
	for _, request := range []uintptr{syscall.TIOCPTYGRANT, syscall.TIOCPTYUNLK} {
		_, _, err := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), request, 0)
		if err != 0 {
			t.Fatal("initialize macOS test terminal:", err)
		}
	}
	slave, openErr := os.OpenFile(string(name[:end]), os.O_RDWR|syscall.O_NOCTTY, 0)
	if openErr != nil {
		t.Fatal(openErr)
	}
	t.Cleanup(func() { slave.Close() })
	return slave
}
