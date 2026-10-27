//go:build !linux && !darwin && !windows

package terminal

import "os"

func isTerminal(file *os.File) bool  { return false }
func enableColor(file *os.File) bool { return false }

func terminalWidth(file *os.File) int { return 0 }
