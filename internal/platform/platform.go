// Package platform isolates operating-system-specific socket and process access.
package platform

import (
	"errors"
	"runtime"
	"sort"
)

var ErrUnsupported = errors.New("unsupported operating system")

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

// TerminationAction describes the action truthfully on each operating system.
func TerminationAction() string {
	if runtime.GOOS == "windows" {
		return "immediate termination"
	}
	return "SIGTERM"
}

func sortSockets(rows []Socket) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Protocol != b.Protocol {
			return a.Protocol < b.Protocol
		}
		if a.Local != b.Local {
			return a.Local < b.Local
		}
		if a.Remote != b.Remote {
			return a.Remote < b.Remote
		}
		return a.Inode < b.Inode
	})
}
