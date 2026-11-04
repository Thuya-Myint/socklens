//go:build windows

package platform

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsNameSnapshotNative(t *testing.T) {
	names, err := windowsSnapshotNames()
	if err != nil {
		t.Fatal(err)
	}
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(names[os.Getpid()], filepath.Base(path)) {
		t.Fatalf("self process missing: %q", names[os.Getpid()])
	}
	rows := []Socket{{Processes: []Process{{PID: os.Getpid()}}}}
	fillMissingProcessNames(rows, names)
	if rows[0].Processes[0].Name == "" {
		t.Fatal("native name fallback left self unnamed")
	}
}
func TestWindowsServicesNative(t *testing.T) {
	snapshot := CollectServices(context.Background(), []Socket{{PIDs: []int{os.Getpid()}}})
	if snapshot == nil || snapshot.Source != "windows-scm" || snapshot.Detail == "" {
		t.Fatal(snapshot)
	}
	if snapshot.Status != "read" && snapshot.Status != "unavailable" && snapshot.Status != "partial" {
		t.Fatal(snapshot)
	}
	// Cancellation must not enumerate or imply a completed lookup.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snapshot = CollectServices(ctx, []Socket{{PIDs: []int{os.Getpid()}}})
	if snapshot.Status == "read" {
		t.Fatal("cancelled service lookup reported success")
	}
}
