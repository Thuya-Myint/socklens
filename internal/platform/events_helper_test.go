//go:build linux || darwin

package platform

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func eventHelper(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bpftrace")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSocketEventsLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	path := eventHelper(t, "printf 'READY\\nCHANGE\\n'\nexec sleep 30\n")
	source, err := startEventHelper(ctx, "test", path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if err := source.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	waitCtx, stop := context.WithTimeout(ctx, 20*time.Millisecond)
	defer stop()
	if err := source.Wait(waitCtx); err != context.DeadlineExceeded {
		t.Fatalf("idle wait: %v", err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	if err := source.Wait(ctx); err == nil {
		t.Fatal("helper exit was hidden")
	}
}

func TestSocketEventsAttachmentFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	path := eventHelper(t, "printf 'permission denied' >&2\nexit 1\n")
	if _, err := startEventHelper(ctx, "test", path); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("attachment error: %v", err)
	}
}

func TestSocketEventsCancelDuringAttachment(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	path := eventHelper(t, "exec sleep 30\n")
	if _, err := startEventHelper(ctx, "test", path); err == nil {
		t.Fatal("cancelled attachment succeeded")
	}
}

func TestEventDiagnosticsBounded(t *testing.T) {
	d := &limitedDiagnostics{}
	input := []byte(strings.Repeat("x", 8192))
	if n, err := d.Write(input); n != len(input) || err != nil || len(d.text) != 4096 {
		t.Fatalf("n=%d err=%v size=%d", n, err, len(d.text))
	}
}
