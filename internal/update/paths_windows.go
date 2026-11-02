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

var finalInstallPath = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFinalPathNameByHandleW")

// Resolve through Windows itself: Scoop's current directory is a junction,
// which filepath.EvalSymlinks cannot resolve on some Windows installations.
// Following an open handle also supports junctions in custom Scoop roots.
func resolveInstallPath(path string) (string, error) {
	native, err := syscall.UTF16PtrFromString(longWindowsPath(path))
	if err != nil {
		return "", &os.PathError{Op: "resolve", Path: path, Err: err}
	}
	handle, err := syscall.CreateFile(native, 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", &os.PathError{Op: "resolve", Path: path, Err: err}
	}
	defer syscall.CloseHandle(handle)
	buffer := make([]uint16, 512)
	for attempts := 0; attempts < 3; attempts++ {
		count, _, callErr := finalInstallPath.Call(uintptr(handle), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0)
		if count == 0 {
			return "", &os.PathError{Op: "resolve", Path: path, Err: callErr}
		}
		if count >= uintptr(len(buffer)) {
			if count > 32768 {
				return "", fmt.Errorf("resolved installation path exceeds Windows path limit: %s", path)
			}
			buffer = make([]uint16, int(count)+1)
			continue
		}
		resolved := syscall.UTF16ToString(buffer[:count])
		if strings.HasPrefix(resolved, `\\?\UNC\`) {
			resolved = `\\` + resolved[len(`\\?\UNC\`):]
		} else if strings.HasPrefix(resolved, `\\?\`) && len(resolved) >= 7 && resolved[5] == ':' {
			resolved = resolved[len(`\\?\`):]
		}
		if !filepath.IsAbs(resolved) {
			return "", fmt.Errorf("Windows returned an invalid installation path for %s", path)
		}
		return filepath.Clean(resolved), nil
	}
	return "", fmt.Errorf("installation path changed repeatedly while resolving %s", path)
}
