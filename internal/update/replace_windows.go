//go:build windows

package update

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func longWindowsPath(path string) string {
	if filepath.IsAbs(path) && !strings.HasPrefix(path, `\\?\`) {
		if strings.HasPrefix(path, `\\`) {
			return `\\?\UNC\` + path[2:]
		}
		return `\\?\` + path
	}
	return path
}
func replaceExecutable(target, candidate, backup string) error {
	destination, err := syscall.UTF16PtrFromString(longWindowsPath(target))
	if err != nil {
		return err
	}
	source, err := syscall.UTF16PtrFromString(longWindowsPath(candidate))
	if err != nil {
		return err
	}
	var saved *uint16
	if backup != "" {
		saved, err = syscall.UTF16PtrFromString(longWindowsPath(backup))
		if err != nil {
			return err
		}
	}
	result, _, callErr := syscall.NewLazyDLL("kernel32.dll").NewProc("ReplaceFileW").Call(uintptr(unsafe.Pointer(destination)), uintptr(unsafe.Pointer(source)), uintptr(unsafe.Pointer(saved)), 0, 0, 0)
	if result == 0 {
		return fmt.Errorf("Windows could not replace the executable; close other SockLens processes and retry: %w", callErr)
	}
	return nil
}
func restoreExecutable(target, backup string) error {
	if _, err := os.Lstat(target); os.IsNotExist(err) {
		return os.Rename(backup, target)
	} else if err != nil {
		return err
	}
	return replaceExecutable(target, backup, "")
}
