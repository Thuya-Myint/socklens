//go:build windows

package update

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// Readers must allow deletion/rename so another SockLens process can replace
// the settings atomically while a reader holds the previous file open.
func openStateFile(path string) (*os.File, error) {
	native := path
	if filepath.IsAbs(native) && !strings.HasPrefix(native, `\\?\`) {
		if strings.HasPrefix(native, `\\`) {
			native = `\\?\UNC\` + native[2:]
		} else {
			native = `\\?\` + native
		}
	}
	name, err := syscall.UTF16PtrFromString(native)
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		var code syscall.Errno
		if errors.As(err, &code) && (code == 32 || code == 33 || code == 303) {
			err = ErrBusy
		}
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}

// NTFS can report access denied while a removed lock directory is still
// delete-pending. Treat it as contention and skip this optional check.
func lockContention(err error) bool {
	var code syscall.Errno
	return errors.As(err, &code) && (code == 5 || code == 32 || code == 33 || code == 303)
}

// ReplaceFile preserves the destination permissions and allows readers that
// share deletion to finish reading the old settings during replacement.
func replaceStateFile(oldpath, newpath string) error {
	oldname, err := syscall.UTF16PtrFromString(oldpath)
	if err != nil {
		return err
	}
	newname, err := syscall.UTF16PtrFromString(newpath)
	if err != nil {
		return err
	}
	result, _, callErr := syscall.NewLazyDLL("kernel32.dll").NewProc("ReplaceFileW").Call(uintptr(unsafe.Pointer(newname)), uintptr(unsafe.Pointer(oldname)), 0, 0, 0, 0)
	if result != 0 {
		return nil
	}
	if errors.Is(callErr, syscall.ERROR_FILE_NOT_FOUND) {
		return os.Rename(oldpath, newpath)
	}
	return &os.LinkError{Op: "replace", Old: oldpath, New: newpath, Err: callErr}
}
