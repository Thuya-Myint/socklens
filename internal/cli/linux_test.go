//go:build linux

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

func TestInspectLiveSocket(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	var out, stderr bytes.Buffer
	code := Run([]string{"inspect", "--json", port}, strings.NewReader(""), &out, &stderr, "test")
	if code != 0 {
		t.Fatal(stderr.String())
	}
	var rows []platform.Socket
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Local == listener.Addr().String() {
			return
		}
	}
	t.Fatal("inspection did not return listener")
}

func TestTerminationConfirmation(t *testing.T) {
	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer child.Process.Kill()
	target, openErr := platform.OpenTarget(child.Process.Pid)
	if errors.Is(openErr, platform.ErrSafeTerminationUnsupported) {
		t.Skip(openErr)
	}
	if openErr != nil {
		t.Fatal(openErr)
	}
	target.Close()
	pid := strconv.Itoa(child.Process.Pid)
	for _, answer := range []string{"no\n", pid} {
		var out, stderr bytes.Buffer
		if code := Run([]string{"terminate", "--pid", pid}, strings.NewReader(answer), &out, &stderr, "test"); code == 0 {
			t.Fatalf("accepted incomplete or mismatched confirmation %q", answer)
		}
		if _, err := platform.Identify(child.Process.Pid); err != nil {
			t.Fatal("cancelled termination affected child", err)
		}
	}
	var out, stderr bytes.Buffer
	if code := Run([]string{"terminate", "--pid", pid}, strings.NewReader(pid+"\n"), &out, &stderr, "test"); code != 0 {
		t.Fatal(stderr.String())
	}
	if !strings.Contains(out.String(), fmt.Sprintf("SIGTERM sent to PID %s", pid)) {
		t.Fatal(out.String())
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected signal exit")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("child did not terminate")
	}
}

func TestLiveDiscoveryFilters(t *testing.T) {
	tcp, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()
	udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	for _, tc := range []struct{ protocol, address string }{
		{"tcp", tcp.Addr().String()}, {"udp", udp.LocalAddr().String()},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			_, port, _ := net.SplitHostPort(tc.address)
			args := []string{"list", "--listening", "--pid", strconv.Itoa(os.Getpid()), "--port", port, "--protocol", tc.protocol, "--json"}
			var out, stderr bytes.Buffer
			if code := Run(args, strings.NewReader(""), &out, &stderr, "test"); code != 0 {
				t.Fatal(stderr.String())
			}
			var rows []platform.Socket
			if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 || rows[0].Local != tc.address {
				t.Fatalf("unexpected sockets: %+v", rows)
			}
			for _, process := range rows[0].Processes {
				if process.PID == os.Getpid() && process.Name != "" {
					return
				}
			}
			t.Fatal("owning process name missing")
		})
	}
}

func TestWatchLiveListenerLifecycle(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	var stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		code := RunContext(ctx, []string{"watch", "--port", port, "--pid", strconv.Itoa(os.Getpid()), "--listening", "--interval", "100ms", "--count", "3", "--json"},
			strings.NewReader(""), writer, &stderr, "test")
		writer.Close()
		done <- code
	}()
	decoder := json.NewDecoder(reader)
	var first snapshot
	if err := decoder.Decode(&first); err != nil {
		t.Fatal(err)
	}
	if len(first.Sockets) != 1 || first.Sockets[0].Local != listener.Addr().String() {
		t.Fatalf("listener missing: %+v", first)
	}
	listener.Close()
	sawEmpty := false
	for i := 0; i < 2; i++ {
		var next snapshot
		if err := decoder.Decode(&next); err != nil {
			t.Fatal(err)
		}
		if len(next.Sockets) == 0 {
			sawEmpty = true
		}
	}
	var extra snapshot
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("expected EOF after 3 snapshots: %v", err)
	}
	if code := <-done; code != 0 {
		t.Fatalf("code=%d error=%s", code, &stderr)
	}
	if !sawEmpty {
		t.Fatal("closed listener did not disappear")
	}
}

func TestWatchChangesLiveRemoval(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	address := listener.Addr().String()
	_, port, _ := net.SplitHostPort(address)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	var stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		code := RunContext(ctx, []string{"watch", "--changes", "--json", "--port", port, "--pid", strconv.Itoa(os.Getpid()), "--interval", "100ms", "--count", "3"},
			strings.NewReader(""), writer, &stderr, "test")
		writer.Close()
		done <- code
	}()
	decoder := json.NewDecoder(reader)
	var baseline changeSet
	if err := decoder.Decode(&baseline); err != nil {
		t.Fatal(err)
	}
	if !baseline.Baseline || len(baseline.Added) != 1 || baseline.Added[0].Local != address {
		t.Fatalf("bad baseline: %+v", baseline)
	}
	listener.Close()
	var removed changeSet
	if err := decoder.Decode(&removed); err != nil {
		t.Fatal(err)
	}
	if removed.Baseline || len(removed.Removed) != 1 || removed.Removed[0].Local != address {
		t.Fatalf("removal missing: %+v", removed)
	}
	var extra changeSet
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("unexpected extra changes: %+v %v", extra, err)
	}
	if code := <-done; code != 0 {
		t.Fatalf("code=%d error=%s", code, &stderr)
	}
}

func TestWaitLiveListener(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	for _, condition := range []string{"present", "absent"} {
		if condition == "absent" {
			listener.Close()
		}
		var out, stderr bytes.Buffer
		code := Run([]string{"wait", "--listening", "--protocol", "tcp", "--pid", strconv.Itoa(os.Getpid()), "--until", condition, "--timeout", "1s", "--json", port},
			strings.NewReader(""), &out, &stderr, "test")
		if code != 0 {
			t.Fatalf("code=%d err=%s", code, &stderr)
		}
		var result waitResult
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Condition != condition || result.Timestamp.IsZero() {
			t.Fatalf("bad result: %+v", result)
		}
		if condition == "present" && len(result.Sockets) != 1 {
			t.Fatalf("listener missing: %+v", result)
		}
		if condition == "absent" && len(result.Sockets) != 0 {
			t.Fatalf("closed listener still present: %+v", result)
		}
	}
}

func TestTerminateDryRunAndPromptFailure(t *testing.T) {
	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	defer func() { child.Process.Kill(); <-done }()
	pid := strconv.Itoa(child.Process.Pid)
	var out, stderr bytes.Buffer
	code := Run([]string{"terminate", "--pid", pid, "--dry-run"}, strings.NewReader(""), &out, &stderr, "test")
	if code != 0 || !strings.Contains(out.String(), "No signal sent") || strings.Contains(out.String(), "confirm:") {
		t.Fatalf("code=%d output=%s%s", code, &out, &stderr)
	}
	select {
	case err := <-done:
		// Put the result back for deferred cleanup.
		done <- err
		t.Fatalf("dry-run terminated the child: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	target, err := platform.OpenTarget(child.Process.Pid)
	if errors.Is(err, platform.ErrSafeTerminationUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	target.Close()
	stderr.Reset()
	code = Run([]string{"terminate", "--pid", pid}, strings.NewReader(pid+"\n"), brokenWriter{}, &stderr, "test")
	if code != 1 || !strings.Contains(stderr.String(), "output unavailable") {
		t.Fatalf("prompt error lost: %d %s", code, &stderr)
	}
	select {
	case err := <-done:
		done <- err
		t.Fatalf("failed prompt still signalled child: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	stderr.Reset()
	if code := Run([]string{"terminate", "--pid", pid, "--dry-run"}, strings.NewReader(""), brokenWriter{}, &stderr, "test"); code != 1 {
		t.Fatal("dry-run output error lost")
	}
}
