//go:build !windows

package platform

import (
	"os"
	"os/exec"
)

func preparePolicy(command *exec.Cmd) {}

func policyEnvironment() []string { return append(os.Environ(), "LC_ALL=C", "LANG=C") }
