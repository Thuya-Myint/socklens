package platform

import "context"

// SocketEvents invalidates socket snapshots. Notifications may be coalesced;
// callers must enumerate sockets again rather than interpret them as history.
type SocketEvents interface {
	Wait(context.Context) error
	Close() error
}
