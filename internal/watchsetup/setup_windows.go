//go:build windows

package watchsetup

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var shellExecute = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteExW")

type shellExecuteInfo struct {
	Size, Mask                        uint32
	Window                            uintptr
	Verb, File, Parameters, Directory *uint16
	Show                              int32
	Instance, IDList                  uintptr
	Class                             *uint16
	ClassKey                          uintptr
	HotKey                            uint32
	Icon                              uintptr
	Process                           syscall.Handle
}

func canElevate() (bool, error) {
	process, err := syscall.GetCurrentProcess()
	if err != nil {
		return false, err
	}
	var token syscall.Token
	if err := syscall.OpenProcessToken(process, syscall.TOKEN_QUERY, &token); err != nil {
		return false, err
	}
	defer token.Close()
	var elevated, size uint32
	if err := syscall.GetTokenInformation(token, syscall.TokenElevation, (*byte)(unsafe.Pointer(&elevated)), 4, &size); err != nil {
		return false, err
	}
	return elevated == 0, nil
}

func elevateWatch(ctx context.Context, args []string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := shellExecute.Find(); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	directory, err := os.Getwd()
	if err != nil {
		return err
	}
	quoted := []string{syscall.EscapeArg("watch")}
	for _, arg := range args {
		quoted = append(quoted, syscall.EscapeArg(arg))
	}
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, err := syscall.UTF16PtrFromString(executable)
	if err != nil {
		return err
	}
	parameters, err := syscall.UTF16PtrFromString(strings.Join(quoted, " "))
	if err != nil {
		return err
	}
	workingDirectory, err := syscall.UTF16PtrFromString(directory)
	if err != nil {
		return err
	}
	info := shellExecuteInfo{Mask: 0x140, Verb: verb, File: file, Parameters: parameters, Directory: workingDirectory, Show: 1} // NOCLOSEPROCESS | NOASYNC
	info.Size = uint32(unsafe.Sizeof(info))
	ok, _, callErr := shellExecute.Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		if callErr == syscall.Errno(1223) {
			return fmt.Errorf("administrator access cancelled")
		}
		return callErr
	}
	if info.Process != 0 {
		syscall.CloseHandle(info.Process)
	}
	return nil
}

func installPlan() ([]command, error) {
	return nil, fmt.Errorf("Windows ETW requires no helper installation")
}
