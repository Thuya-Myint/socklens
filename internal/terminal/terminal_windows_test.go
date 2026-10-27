//go:build windows

package terminal

import (
	"testing"
	"unsafe"
)

func TestWindowsConsoleInfoABI(t *testing.T) {
	var info consoleScreenBufferInfo
	if unsafe.Sizeof(info) != 22 || unsafe.Offsetof(info.Window) != 10 || unsafe.Offsetof(info.MaximumSize) != 18 {
		t.Fatal("console screen buffer layout does not match the Windows API")
	}
}
