package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVersionInfo(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, buildDate
	defer func() { version, commit, buildDate = oldVersion, oldCommit, oldDate }()
	version, commit, buildDate = "v1.2.3", "abcdef123456", "2026-10-26T00:00:00Z"
	if got := versionInfo(); got != "v1.2.3" {
		t.Fatalf("version output must contain only the version: %q", got)
	}
}

func TestAutomaticNoticeEligibility(t *testing.T) {
	for _, tc := range []struct {
		args           []string
		terminal, want bool
	}{
		{[]string{"list"}, true, true}, {[]string{"watch", "--changes"}, true, true},
		{[]string{"list", "--json"}, true, false}, {[]string{"inspect", "--json=true", "8080"}, true, false},
		{[]string{"list", "-json"}, true, false}, {[]string{"list"}, false, false},
		{[]string{"version"}, true, false}, {[]string{"terminate", "--pid", "123"}, true, false},
		{[]string{"help"}, true, false}, {[]string{"update-check"}, true, false},
		{[]string{"updates"}, true, false}, {[]string{"update"}, true, false}, {[]string{"list", "--help"}, true, false}, {nil, true, false},
	} {
		if got := shouldAutoNotify(tc.args, tc.terminal); got != tc.want {
			t.Fatalf("%v terminal=%v: %v", tc.args, tc.terminal, got)
		}
	}
}

func TestMain(m *testing.M) {
	if os.Getenv("SOCKLENS_WORKER_TEST_HELPER") == "1" {
		if len(os.Args) != 3 || os.Args[1] != "__update-refresh" {
			os.Exit(2)
		}
		if err := os.WriteFile(os.Getenv("SOCKLENS_WORKER_TEST_MARKER"), []byte(os.Args[2]), 0600); err != nil {
			os.Exit(3)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestDetachedUpdateWorker(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "worker-result")
	t.Setenv("SOCKLENS_WORKER_TEST_HELPER", "1")
	t.Setenv("SOCKLENS_WORKER_TEST_MARKER", marker)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	nonce := "0123456789abcdef0123456789abcdef"
	if err := startUpdateWorker(executable, nonce); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(marker)
		if err == nil {
			if string(data) != nonce {
				t.Fatal("worker arguments changed")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("background worker did not complete")
}
