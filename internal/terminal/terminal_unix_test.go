//go:build linux || darwin

package terminal

import (
	"os"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

func TestTerminalWidthTracksResizeWithAndWithoutColor(t *testing.T) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("pseudo-terminal unavailable: %v", err)
	}
	t.Cleanup(func() { master.Close() })
	tty := testTerminalSide(t, master)
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm")
	for _, columns := range []uint16{40, 120, 80} {
		size := [4]uint16{24, columns, 0, 0}
		_, _, err := syscall.Syscall(syscall.SYS_IOCTL, tty.Fd(), syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(&size)))
		if err != 0 {
			t.Fatal("cannot resize test terminal:", err)
		}
		for _, mode := range []string{"auto", "never"} {
			if got := Width(Wrap(tty, mode)); got != int(columns) {
				t.Fatalf("width %d after resize, want %d", got, columns)
			}
		}
	}
	if !strings.Contains(Style(Wrap(tty, "auto"), Success, "OK"), "\x1b[") {
		t.Fatal("native pseudo-terminal did not enable default colors")
	}
}
