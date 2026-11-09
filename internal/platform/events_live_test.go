//go:build linux || windows || darwin

package platform

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestSocketEventsLiveChild(t *testing.T) {
	if os.Getenv("SOCKLENS_EVENT_CHILD") != "1" {
		return
	}
	var socket io.Closer
	var address string
	if os.Getenv("SOCKLENS_EVENT_PROTOCOL") == "udp" {
		listener, err := net.ListenPacket("udp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		socket, address = listener, listener.LocalAddr().String()
	} else {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		socket, address = listener, listener.Addr().String()
	}
	defer socket.Close()
	fmt.Println(address)
	if !bufio.NewScanner(os.Stdin).Scan() {
		t.Fatal("missing close request")
	}
	if err := socket.Close(); err != nil {
		t.Fatal(err)
	}
	fmt.Println("CLOSED")
}

// Run on a tracing-enabled host with sufficient privileges:
// SOCKLENS_EVENT_INTEGRATION=1 go test ./internal/platform -run '^TestSocketEventsLive$'
func TestSocketEventsLive(t *testing.T) {
	if os.Getenv("SOCKLENS_EVENT_INTEGRATION") != "1" {
		t.Skip("requires opt-in privileged OS tracing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	source, err := WatchSocketEvents(ctx)
	if err != nil {
		if runtime.GOOS == "darwin" && os.Getenv("SOCKLENS_EVENT_ALLOW_UNAVAILABLE") == "1" {
			t.Skipf("macOS tracing unavailable (SIP/probe restrictions): %v", err)
		}
		t.Fatal(err)
	}
	defer source.Close()
	for _, protocol := range []string{"tcp", "udp"} {
		t.Run(protocol, func(t *testing.T) { testSocketEventLifecycle(t, ctx, source, protocol) })
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
}

func testSocketEventLifecycle(t *testing.T, ctx context.Context, source SocketEvents, protocol string) {
	t.Helper()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSocketEventsLiveChild$")
	child.Env = append(os.Environ(), "SOCKLENS_EVENT_CHILD=1", "SOCKLENS_EVENT_PROTOCOL="+protocol)
	input, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { child.Process.Kill(); child.Wait() }()
	line := bufio.NewScanner(stdout)
	if !line.Scan() {
		t.Fatal("child did not open a listener")
	}
	address := line.Text()
	waitFor := func(present bool) {
		t.Helper()
		for {
			if err := source.Wait(ctx); err != nil {
				t.Fatal(err)
			}
			rows, err := List()
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, row := range rows {
				if row.Protocol == protocol && row.Local == address && row.IsListener() {
					found = true
				}
			}
			if found == present {
				return
			}
		}
	}
	waitFor(true)
	if _, err := io.WriteString(input, "close\n"); err != nil {
		t.Fatal(err)
	}
	if !line.Scan() || line.Text() != "CLOSED" {
		t.Fatal("child did not close socket")
	}
	waitFor(false)
}
