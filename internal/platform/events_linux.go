//go:build linux

package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Descriptor-close probes also produce non-socket invalidations. Excluding our
// own enumeration prevents its descriptor closes from causing a feedback loop.
func socketEventProgram(pid int) string {
	return fmt.Sprintf(`BEGIN { printf("READY\n"); }
tracepoint:sock:inet_sock_set_state,
tracepoint:syscalls:sys_exit_bind,
tracepoint:syscalls:sys_exit_connect,
tracepoint:syscalls:sys_exit_listen,
tracepoint:syscalls:sys_exit_accept*,
tracepoint:syscalls:sys_exit_shutdown,
tracepoint:syscalls:sys_exit_close*
/pid != %d/ { printf("CHANGE\n"); }
`, pid)
}

func WatchSocketEvents(ctx context.Context) (SocketEvents, error) {
	path, err := exec.LookPath("bpftrace")
	if err != nil {
		return nil, fmt.Errorf("Linux events require bpftrace and kernel tracing privileges: %w", err)
	}
	return startSocketEvents(ctx, path)
}

func startSocketEvents(ctx context.Context, path string) (SocketEvents, error) {
	return startEventHelper(ctx, "bpftrace", path, "-q", "-B", "line", "-e", socketEventProgram(os.Getpid()))
}
