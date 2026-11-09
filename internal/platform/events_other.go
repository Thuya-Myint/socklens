//go:build !linux && !windows && !darwin

package platform

import (
	"context"
	"fmt"
	"runtime"
)

func WatchSocketEvents(context.Context) (SocketEvents, error) {
	return nil, fmt.Errorf("socket events on %s: %w", runtime.GOOS, ErrUnsupported)
}
