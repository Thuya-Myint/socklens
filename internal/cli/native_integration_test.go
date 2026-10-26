//go:build windows || darwin

package cli

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNativeCLIChild(t *testing.T) {
	if os.Getenv("SOCKLENS_NATIVE_CLI_CHILD") == "1" {
		time.Sleep(30 * time.Second)
	}
}
func TestNativeDryRunAndWindowsConsent(t *testing.T) {
	child := exec.Command(os.Args[0], "-test.run=^TestNativeCLIChild$")
	child.Env = append(os.Environ(), "SOCKLENS_NATIVE_CLI_CHILD=1")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	defer func() { child.Process.Kill(); <-done }()
	pid := strconv.Itoa(child.Process.Pid)
	var out, stderr bytes.Buffer
	if code := Run([]string{"terminate", "--pid", pid, "--dry-run"}, strings.NewReader(""), &out, &stderr, "test"); code != 0 || !strings.Contains(out.String(), "No signal sent") {
		t.Fatalf("code=%d output=%s%s", code, &out, &stderr)
	}
	if runtime.GOOS == "windows" {
		out.Reset()
		stderr.Reset()
		if code := Run([]string{"terminate", "--pid", pid}, strings.NewReader(pid+"\n"), &out, &stderr, "test"); code != 1 || !strings.Contains(stderr.String(), "--force") {
			t.Fatalf("immediate termination without consent: %d %s", code, &stderr)
		}
		out.Reset()
		stderr.Reset()
		if code := Run([]string{"terminate", "--pid", pid, "--force"}, strings.NewReader("no\n"), &out, &stderr, "test"); code != 1 || !strings.Contains(out.String(), "immediate termination") {
			t.Fatalf("bad confirmation behavior: %d %s%s", code, &out, &stderr)
		}
	}
	select {
	case err := <-done:
		done <- err
		t.Fatalf("preview/cancellation killed target: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
}
