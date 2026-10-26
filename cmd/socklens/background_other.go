//go:build !linux && !darwin && !windows

package main

import "os/exec"

func configureUpdateWorker(command *exec.Cmd) {}
