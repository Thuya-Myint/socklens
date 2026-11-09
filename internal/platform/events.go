package platform

import (
	"context"
	"errors"
)

var ErrEventHelperMissing = errors.New("bpftrace is not installed")
var ErrEventPrivileges = errors.New("socket tracing requires administrator access")

// SocketEvents invalidates socket snapshots. Notifications may be coalesced;
// callers must enumerate sockets again rather than interpret them as history.
type SocketEvents interface {
	Wait(context.Context) error
	Close() error
}
