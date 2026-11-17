package platform

import (
	"context"
	"errors"
	"sync/atomic"
)

var ErrEventHelperMissing = errors.New("bpftrace is not installed")
var ErrEventPrivileges = errors.New("socket tracing requires administrator access")

// SocketEvents invalidates socket snapshots. Notifications may be coalesced;
// callers must enumerate sockets again rather than interpret them as history.
type SocketEvents interface {
	Wait(context.Context) error
	Close() error
}

// SocketEventStats counts notifications delivered to this process. Coalesced
// notifications found an invalidation already pending; they are not lost sockets
// or a measurement of events dropped by the OS/tracer. Reads are concurrent and
// the two cumulative counters are not a transactional snapshot.
type SocketEventStats struct {
	Notifications uint64
	Coalesced     uint64
}

// SocketEventDiagnostics is optionally implemented by an event source.
type SocketEventDiagnostics interface {
	EventStats() SocketEventStats
}

type notificationStats struct {
	notifications atomic.Uint64
	coalesced     atomic.Uint64
}

func (s *notificationStats) notify(changes chan<- struct{}) {
	s.notifications.Add(1)
	select {
	case changes <- struct{}{}:
	default:
		s.coalesced.Add(1)
	}
}

func (s *notificationStats) snapshot() SocketEventStats {
	return SocketEventStats{Notifications: s.notifications.Load(), Coalesced: s.coalesced.Load()}
}
