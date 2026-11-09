//go:build windows

package platform

import (
	"errors"
	"syscall"
	"testing"
	"unsafe"
)

func TestWindowsWatchPrivilegeErrors(t *testing.T) {
	err := windowsEventError("start tracing", 5)
	if !errors.Is(err, ErrEventPrivileges) || !errors.Is(err, syscall.Errno(5)) {
		t.Fatalf("lost permission error: %v", err)
	}
	if err := windowsEventError("start tracing", 87); errors.Is(err, ErrEventPrivileges) {
		t.Fatalf("invalid parameters requested elevation: %v", err)
	}
}

// These ABI sizes/offsets are fixed by the Windows SDK on amd64 and arm64.
// A layout regression would corrupt memory in OpenTraceW or its callback.
func TestSocketETWABI(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("ETW backend requires 64-bit Windows")
	}
	for _, check := range []struct {
		name      string
		got, want uintptr
	}{
		{"WNODE_HEADER", unsafe.Sizeof(etwWnode{}), 48},
		{"EVENT_TRACE_PROPERTIES", unsafe.Sizeof(etwProperties{}), 120},
		{"EVENT_TRACE", unsafe.Sizeof(etwCurrentEvent{}), 88},
		{"TRACE_LOGFILE_HEADER", unsafe.Sizeof(etwLogHeader{}), 280},
		{"EVENT_TRACE_LOGFILEW", unsafe.Sizeof(etwLogfile{}), 448},
		{"EventRecordCallback", unsafe.Offsetof(etwLogfile{}.EventRecordCallback), 424},
		{"Context", unsafe.Offsetof(etwLogfile{}.Context), 440},
		{"UserContext", unsafe.Offsetof(etwRecord{}.UserContext), 104},
	} {
		if check.got != check.want {
			t.Errorf("%s: got %d want %d", check.name, check.got, check.want)
		}
	}
}

func TestSocketETWRouting(t *testing.T) {
	first, second := etwSourceID.Add(1), etwSourceID.Add(1)
	a := &windowsSocketEvents{changes: make(chan struct{}, 1)}
	b := &windowsSocketEvents{changes: make(chan struct{}, 1)}
	etwSources.Store(first, a)
	etwSources.Store(second, b)
	defer etwSources.Delete(first)
	defer etwSources.Delete(second)
	for i := 0; i < 100; i++ {
		notifySocketETW(&etwRecord{UserContext: first})
	}
	if len(a.changes) != 1 || len(b.changes) != 0 {
		t.Fatal("callbacks must coalesce and notify only their owning session")
	}
	etwSources.Delete(first)
	<-a.changes
	notifySocketETW(&etwRecord{UserContext: first})
	if len(a.changes) != 0 {
		t.Fatal("late callback reached a closed session")
	}
}
