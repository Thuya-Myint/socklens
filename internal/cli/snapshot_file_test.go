package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

func TestSnapshotSaveAndDiff(t *testing.T) {
	dir := t.TempDir()
	beforePath := filepath.Join(dir, "before.json")
	afterPath := filepath.Join(dir, "after.json")
	before := []platform.Socket{{Protocol: "tcp", Local: "127.0.0.1:8080", Remote: "0.0.0.0:0", State: "LISTEN", Inode: "1", PIDs: []int{42}}}
	after := []platform.Socket{{Protocol: "tcp", Local: "127.0.0.1:9090", Remote: "0.0.0.0:0", State: "LISTEN", Inode: "2", PIDs: []int{43}}}
	var out, errs bytes.Buffer
	if code := snapshotSave([]string{beforePath}, &out, &errs, func() ([]platform.Socket, error) { return before, nil }); code != 0 {
		t.Fatalf("save before: %s", errs.String())
	}
	out.Reset()
	if code := snapshotSave([]string{afterPath}, &out, &errs, func() ([]platform.Socket, error) { return after, nil }); code != 0 {
		t.Fatalf("save after: %s", errs.String())
	}
	out.Reset()
	if code := snapshotDiff([]string{"--json", beforePath, afterPath}, &out, &errs); code != 0 {
		t.Fatalf("diff: %s", errs.String())
	}
	var delta changeSet
	if err := json.Unmarshal(out.Bytes(), &delta); err != nil {
		t.Fatal(err)
	}
	if len(delta.Added) != 1 || len(delta.Removed) != 1 || len(delta.Updated) != 0 {
		t.Fatalf("unexpected delta: %+v", delta)
	}
	if code := snapshotSave([]string{beforePath}, &out, &errs, func() ([]platform.Socket, error) { return before, nil }); code == 0 {
		t.Fatal("expected overwrite to be refused")
	}
}

func TestSnapshotRejectsMalformedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	for _, content := range []string{"{}", "not json", `{"version":2,"timestamp":"2026-01-01T00:00:00Z","sockets":[]}`, `{"version":1,"timestamp":"2026-01-01T00:00:00Z","sockets":[]} trailing`} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readSnapshotFile(path); err == nil {
			t.Errorf("accepted invalid snapshot %q", content)
		}
	}
}

func TestSnapshotSaveFiltering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filtered.json")
	rows := []platform.Socket{
		{Protocol: "tcp", Local: "127.0.0.1:8080", State: "LISTEN"},
		{Protocol: "tcp", Local: "127.0.0.1:9090", State: "LISTEN"},
	}
	var out, errs bytes.Buffer
	if code := snapshotSave([]string{"--port-range", "8000-8100", path}, &out, &errs, func() ([]platform.Socket, error) { return rows, nil }); code != 0 {
		t.Fatalf("save: %s", errs.String())
	}
	record, err := readSnapshotFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Sockets) != 1 || !strings.HasSuffix(record.Sockets[0].Local, ":8080") {
		t.Fatalf("wrong filter result: %+v", record.Sockets)
	}
	if record.Timestamp.After(time.Now().Add(time.Minute)) {
		t.Fatal("timestamp unexpectedly in the future")
	}
}

func TestSnapshotConcurrentSavePreservesWinner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.json")
	start := make(chan struct{})
	results := make(chan error, 12)
	for i := 0; i < 12; i++ {
		go func() {
			<-start
			results <- saveSnapshotFile(path, savedSnapshot{Version: 1, Timestamp: time.Now().UTC(), Sockets: []platform.Socket{}})
		}()
	}
	close(start)
	winners := 0
	for i := 0; i < 12; i++ {
		if <-results == nil {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("got %d successful saves; want exactly one", winners)
	}
	if _, err := readSnapshotFile(path); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("temporary files remain: %v", entries)
	}
}

func TestSnapshotPreservesDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	target := filepath.Join(dir, "missing.json")
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	err := saveSnapshotFile(path, savedSnapshot{Version: 1, Timestamp: time.Now(), Sockets: []platform.Socket{}})
	if err == nil {
		t.Fatal("overwrote dangling symlink")
	}
	if got, err := os.Readlink(path); err != nil || got != target {
		t.Fatalf("symlink changed: %q %v", got, err)
	}
}

func TestSnapshotRejectsUnknownAndOversizedData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	for _, data := range []string{
		`{"version":1,"timestamp":"2026-01-01T00:00:00Z","sockets":[],"extra":true}`,
		`{"version":1,"timestamp":"2026-01-01T00:00:00Z","sockets":[{"Protocol":"tcp","extra":true}]}`,
		`{"version":1,"timestamp":"2026-01-01T00:00:00Z","sockets":null}`,
		strings.Repeat(" ", maxSnapshotBytes+1),
	} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readSnapshotFile(path); err == nil {
			t.Fatal("accepted invalid file")
		}
	}
}

func TestSnapshotCombinedFilters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "combined.json")
	rows := []platform.Socket{
		{Protocol: "tcp", Local: "127.0.0.1:8080", State: "LISTEN", PIDs: []int{42}, Processes: []platform.Process{{PID: 42, Name: "MyServer"}}},
		{Protocol: "tcp", Local: "0.0.0.0:8080", State: "LISTEN", PIDs: []int{42}},
		{Protocol: "udp", Local: "127.0.0.1:8080", State: "BOUND", PIDs: []int{42}},
		{Protocol: "tcp", Local: "127.0.0.1:8081", State: "LISTEN", PIDs: []int{43}},
	}
	args := []string{"--protocol", "tcp", "--local-address", "127.0.0.1", "--port-range", "8000-8100", "--pid", "42", "--process", "server", "--state", "listen", "--listening", path}
	var out, errs bytes.Buffer
	if code := snapshotSave(args, &out, &errs, func() ([]platform.Socket, error) { return rows, nil }); code != 0 {
		t.Fatal(&errs)
	}
	record, err := readSnapshotFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Sockets) != 1 || record.Sockets[0].Local != "127.0.0.1:8080" {
		t.Fatalf("filters did not intersect: %+v", record.Sockets)
	}
}
