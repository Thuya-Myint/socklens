//go:build windows

package terminal

import (
	"os"
	"syscall"
	"unsafe"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var getConsoleMode = kernel32.NewProc("GetConsoleMode")
var setConsoleMode = kernel32.NewProc("SetConsoleMode")

func consoleMode(file *os.File) (uint32, bool) {
	var mode uint32
	result, _, _ := getConsoleMode.Call(file.Fd(), uintptr(unsafe.Pointer(&mode)))
	return mode, result != 0
}
func isTerminal(file *os.File) bool { _, ok := consoleMode(file); return ok }
func enableColor(file *os.File) bool {
	mode, ok := consoleMode(file)
	if !ok {
		return false
	}
	// ANSI processing also requires processed output, as documented by Windows.
	const virtualTerminalProcessing = 0x0004 | 0x0001
	if mode&virtualTerminalProcessing == virtualTerminalProcessing {
		return true
	}
	result, _, _ := setConsoleMode.Call(file.Fd(), uintptr(mode|virtualTerminalProcessing))
	return result != 0
}

var getConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")

// Windows COORD, WORD and SMALL_RECT fields all have two-byte alignment.
type consoleScreenBufferInfo struct {
	Size        [2]int16
	Cursor      [2]int16
	Attributes  uint16
	Window      struct{ Left, Top, Right, Bottom int16 }
	MaximumSize [2]int16
}

func terminalWidth(file *os.File) int {
	var info consoleScreenBufferInfo
	result, _, _ := getConsoleScreenBufferInfo.Call(file.Fd(), uintptr(unsafe.Pointer(&info)))
	if result == 0 {
		return 0
	}
	columns := int(info.Window.Right) - int(info.Window.Left) + 1
	if columns < 1 {
		return 0
	}
	return columns
}
