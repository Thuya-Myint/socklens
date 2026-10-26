// Package platform isolates operating-system-specific socket and process access.
package platform

import "errors"

var ErrUnsupported = errors.New("this platform is not supported yet; use Linux or WSL2")

// Process is a best-effort process name observed during socket enumeration.
type Process struct {
	PID  int
	Name string
}

type Socket struct {
	Protocol  string
	Local     string
	Remote    string
	State     string
	Inode     string
	PIDs      []int
	Processes []Process
}

// Identity binds a PID to its kernel start time to detect PID reuse.
type Identity struct {
	PID   int
	Start string
	Name  string
}

// Target holds a stable reference to one process across user confirmation.
// Callers must close it, including on cancellation.
type Target interface {
	Identity() Identity
	Terminate() error
	Close() error
}

var ErrSafeTerminationUnsupported = errors.New("safe process handles are unavailable")
