//go:build linux && (amd64 || arm64)

package platform

import (
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"testing"
)

func testTarget(t *testing.T) (*exec.Cmd, Target) {
	t.Helper()
	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { child.Process.Kill(); child.Wait() })
	target, err := OpenTarget(child.Process.Pid)
	if errors.Is(err, ErrSafeTerminationUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { target.Close() })
	return child, target
}

func TestTargetCloseAndStaleIdentity(t *testing.T) {
	child, target := testTarget(t)
	if target.Identity().PID != child.Process.Pid {
		t.Fatal("wrong captured identity")
	}
	handle := target.(*linuxTarget)
	handle.id.Start += "0"
	if err := target.Terminate(); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("stale identity: %v", err)
	}
	if err := pidfdSignal(handle.fd, 0); err != nil {
		t.Fatal("stale attempt affected child", err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal("second close should be harmless", err)
	}
	if err := target.Terminate(); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatal("closed handle accepted", err)
	}
}

func TestPinnedHandleCannotSignalExitedChild(t *testing.T) {
	child, target := testTarget(t)
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	child.Wait()
	handle := target.(*linuxTarget)
	if err := pidfdSignal(handle.fd, syscall.SIGTERM); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("exited process should fail with ESRCH: %v", err)
	}
	if err := target.Terminate(); err == nil {
		t.Fatal("exited target accepted")
	}
}

func TestPidfdErrors(t *testing.T) {
	if fd, err := pidfdOpen(-1); err == nil {
		syscall.Close(fd)
		t.Fatal("invalid PID accepted")
	}
	if err := pidfdSignal(-1, syscall.SIGTERM); !errors.Is(err, syscall.EBADF) {
		t.Fatalf("invalid handle: %v", err)
	}
}

func TestOpenTargetProtectsPIDs(t *testing.T) {
	for _, pid := range []int{0, 1, -1} {
		if target, err := OpenTarget(pid); err == nil {
			target.Close()
			t.Fatalf("protected PID accepted: %d", pid)
		}
	}
}
