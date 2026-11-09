//go:build linux || darwin

package watchsetup

import (
	"context"
	"fmt"
	"os"
	"syscall"
)

func canElevate() (bool, error) { return os.Geteuid() != 0, nil }

func elevateWatch(ctx context.Context, args []string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	// Replace this process so sudo owns foreground signals and its password
	// prompt. No password is read or stored by SockLens, and no parent/helper
	// process is left behind when watch is interrupted.
	return syscall.Exec("/usr/bin/sudo", append([]string{"sudo", "--", executable, "watch"}, args...), os.Environ())
}

func withAdmin(plan []command) ([]command, error) {
	if os.Geteuid() == 0 {
		return plan, nil
	}
	if _, err := os.Stat("/usr/bin/sudo"); err != nil {
		return nil, fmt.Errorf("sudo is unavailable; ask your system administrator to install bpftrace")
	}
	for i := range plan {
		plan[i] = command{path: "/usr/bin/sudo", args: append([]string{"--", plan[i].path}, plan[i].args...)}
	}
	return plan, nil
}
