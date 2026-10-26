//go:build windows

package update

import (
	"errors"
	"syscall"
	"testing"
)

func TestSettingsCanBeReplacedWhileReadHandleIsOpen(t *testing.T) {
	service, _, _ := newService(t)
	if err := service.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	reader, err := openStateFile(service.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if err := service.SetEnabled(false); err != nil {
		t.Fatal("open reader blocked atomic replacement:", err)
	}
	state, err := service.Read()
	if err != nil || state.Enabled {
		t.Fatalf("new settings not visible: %+v %v", state, err)
	}
}
func TestExclusiveWindowsFileLockIsReportedAsBusy(t *testing.T) {
	service, _, _ := newService(t)
	if err := service.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	path, err := syscall.UTF16PtrFromString(service.Path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(path, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	if _, err := service.Read(); !errors.Is(err, ErrBusy) {
		t.Fatalf("sharing conflict was not identified: %v", err)
	}
	message, nonce, err := service.PrepareAutomatic("v0.2.0")
	if !errors.Is(err, ErrBusy) || message != "" || nonce != "" {
		t.Fatal("locked settings produced a notice or request")
	}
}
