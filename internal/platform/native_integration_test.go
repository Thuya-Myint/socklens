//go:build windows || darwin

package platform

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestNativeChild(t *testing.T) {
	if os.Getenv("SOCKLENS_NATIVE_TEST_CHILD") == "1" {
		time.Sleep(30 * time.Second)
	}
}
func startNativeChild(t *testing.T) (*exec.Cmd, chan error) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestNativeChild$")
	command.Env = append(os.Environ(), "SOCKLENS_NATIVE_TEST_CHILD=1")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		command.Process.Kill()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	})
	return command, done
}
func TestNativeLiveSockets(t *testing.T) {
	for _, network := range []string{"tcp4", "tcp6", "udp4", "udp6"} {
		t.Run(network, func(t *testing.T) {
			address := "127.0.0.1:0"
			if network[len(network)-1] == '6' {
				address = "[::1]:0"
			}
			local := ""
			if network[:3] == "tcp" {
				listener, err := net.Listen(network, address)
				if err != nil {
					t.Skip(err)
				}
				defer listener.Close()
				local = listener.Addr().String()
			} else {
				listener, err := net.ListenPacket(network, address)
				if err != nil {
					t.Skip(err)
				}
				defer listener.Close()
				local = listener.LocalAddr().String()
			}
			rows, err := List()
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				if row.Protocol == network[:3] && row.Local == local {
					for _, pid := range row.PIDs {
						if pid == os.Getpid() {
							return
						}
					}
				}
			}
			t.Fatalf("native listener/owning PID missing: %s", local)
		})
	}
}
func TestNativeProcessTarget(t *testing.T) {
	child, done := startNativeChild(t)
	id, err := Identify(child.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	stale := id
	stale.Start += "stale"
	if err := Terminate(stale); err == nil {
		t.Fatal("stale identity accepted")
	}
	target, err := OpenTarget(id.PID)
	if errors.Is(err, ErrSafeTerminationUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if target.Identity().Start != id.Start {
		t.Fatal("target identity changed")
	}
	if err := target.Terminate(); err != nil {
		if errors.Is(err, ErrSafeTerminationUnsupported) {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected termination exit")
		}
		done <- err
	case <-time.After(3 * time.Second):
		t.Fatal("test child did not exit")
	}
}
func TestNativeClosedTarget(t *testing.T) {
	child, done := startNativeChild(t)
	target, err := OpenTarget(child.Process.Pid)
	if errors.Is(err, ErrSafeTerminationUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	if err := target.Terminate(); err == nil {
		t.Fatal("closed target accepted")
	}
	select {
	case err := <-done:
		done <- err
		t.Fatal("closing target killed child")
	case <-time.After(30 * time.Millisecond):
	}
}
func TestNativeProtectedPIDs(t *testing.T) {
	for _, pid := range []int{0, 1, os.Getpid(), 1 << 40} {
		if _, err := Identify(pid); err == nil {
			t.Fatalf("protected PID accepted: %d", pid)
		}
		if target, err := OpenTarget(pid); err == nil {
			target.Close()
			t.Fatalf("protected target accepted: %d", pid)
		}
	}
}
