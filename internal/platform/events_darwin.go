//go:build darwin

package platform

import (
	"context"
	"fmt"
	"os"
)

func darwinSocketEventProgram(pid int) string {
	return fmt.Sprintf(`dtrace:::BEGIN { printf("READY\n"); }
tcp:::state-change,
syscall::bind:return,
syscall::connect:return,
syscall::listen:return,
syscall::accept*:return,
syscall::shutdown:return,
syscall::close*:return
/pid != %d/ { printf("CHANGE\n"); }
`, pid)
}

func WatchSocketEvents(ctx context.Context) (SocketEvents, error) {
	source, err := startEventHelper(ctx, "DTrace", "/usr/sbin/dtrace", "-q", "-x", "switchrate=100ms", "-n", darwinSocketEventProgram(os.Getpid()))
	if err != nil {
		return nil, fmt.Errorf("macOS events require DTrace privileges and available probes; SIP may restrict tracing: %w", err)
	}
	return source, nil
}
