//go:build linux

package watchsetup

import (
	"fmt"
	"os"
)

func installPlan() ([]command, error) {
	// Only system package managers at fixed locations may run with privileges.
	// Never elevate a package-manager executable discovered through user PATH.
	managers := [][]command{
		{{path: "/usr/bin/apt-get", args: []string{"update"}}, {path: "/usr/bin/apt-get", args: []string{"install", "-y", "bpftrace"}}},
		{{path: "/usr/bin/dnf", args: []string{"install", "-y", "bpftrace"}}},
		{{path: "/usr/bin/yum", args: []string{"install", "-y", "bpftrace"}}},
		{{path: "/usr/bin/pacman", args: []string{"-S", "--needed", "--noconfirm", "bpftrace"}}},
		{{path: "/usr/bin/zypper", args: []string{"--non-interactive", "install", "bpftrace"}}},
		{{path: "/sbin/apk", args: []string{"add", "bpftrace"}}},
	}
	for _, plan := range managers {
		if info, err := os.Stat(plan[0].path); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return withAdmin(plan)
		}
	}
	return nil, fmt.Errorf("no supported system package manager found; install bpftrace using your distribution's package manager")
}
