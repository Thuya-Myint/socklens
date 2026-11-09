//go:build windows

package watchsetup

import (
	"testing"
	"unsafe"
)

func TestShellExecuteABI(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("64-bit release ABI")
	}
	if unsafe.Sizeof(shellExecuteInfo{}) != 112 || unsafe.Offsetof(shellExecuteInfo{}.Process) != 104 {
		t.Fatal("SHELLEXECUTEINFOW layout differs from Windows SDK")
	}
}
