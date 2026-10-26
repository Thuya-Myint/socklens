//go:build linux

package platform

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestEndpoint(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"0100007F:1F90", "127.0.0.1:8080"},
		{"00000000000000000000000001000000:0035", "[::1]:53"},
	} {
		got, err := endpoint(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("%s: %s %v", tc.in, got, err)
		}
	}
	for _, raw := range []string{"x", "GGGGGGGG:0050", "0100007F:FFFFF"} {
		if _, err := endpoint(raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}

func TestParseTable(t *testing.T) {
	rows, err := parseTable("header\n 0: 0100007F:1F90 00000000:0000 0A 0:0 0:0 0 1000 0 12345\n", "tcp")
	if err != nil || len(rows) != 1 || rows[0].Inode != "12345" || rows[0].State != "LISTEN" {
		t.Fatalf("%+v %v", rows, err)
	}
	if _, err := parseTable("header\nbad\n", "tcp"); err == nil {
		t.Fatal("accepted malformed row")
	}
}

func TestIdentityNameWithParentheses(t *testing.T) {
	fields := make([]string, 20)
	for i := range fields {
		fields[i] = "0"
	}
	fields[19] = "123"
	id, err := parseIdentity(42, "42 (a tricky ) name) "+strings.Join(fields, " "))
	if err != nil || id.Name != "a tricky ) name" || id.Start != "123" {
		t.Fatalf("%+v %v", id, err)
	}
}

func TestLiveListener(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	rows, err := List()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Local == listener.Addr().String() && row.State == "LISTEN" {
			for _, pid := range row.PIDs {
				if pid == os.Getpid() {
					return
				}
			}
		}
	}
	t.Fatal("listener or owning PID not found")
}

func TestTerminateChildAndRejectStaleIdentity(t *testing.T) {
	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer child.Process.Kill()
	id, err := Identify(child.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	stale := id
	stale.Start += "0"
	if err := Terminate(stale); err == nil {
		t.Fatal("accepted stale identity")
	}
	if _, err := Identify(child.Process.Pid); err != nil {
		t.Fatal("child was signalled by stale identity")
	}
	if err := Terminate(id); err != nil {
		if errors.Is(err, ErrSafeTerminationUnsupported) {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected signal exit")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("child did not exit")
	}
}

func TestProtectedPID(t *testing.T) {
	for _, pid := range []int{0, 1, os.Getpid()} {
		if _, err := Identify(pid); err == nil {
			t.Fatal("accepted protected PID " + strconv.Itoa(pid))
		}
	}
}

func TestEmptySocketTableIsNotAnEmptyResult(t *testing.T) {
	if _, err := parseTable("", "tcp"); err == nil {
		t.Fatal("empty proc data could falsely report a free port")
	}
}
