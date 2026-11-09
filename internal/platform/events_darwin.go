//go:build darwin

package platform

import (
	"context"
	"fmt"
	"os"
	"strings"
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
		message := strings.ToLower(err.Error())
		if os.Geteuid() != 0 && (strings.Contains(message, "requires additional privileges") || strings.Contains(message, "permission denied") || strings.Contains(message, "operation not permitted") || strings.Contains(message, "must be run as root")) {
			return nil, fmt.Errorf("%w: %w", ErrEventPrivileges, err)
		}
		return nil, fmt.Errorf("macOS events require DTrace privileges and available probes; SIP may restrict tracing: %w", err)
	}
	return source, nil
}
