package cli

import (
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

type collectionReason string

const (
	collectionBaseline  collectionReason = "baseline"
	collectionEvent     collectionReason = "event"
	collectionReconcile collectionReason = "reconcile"
	collectionPoll      collectionReason = "poll"
)

type watchDiagnostics struct {
	collections, collectionErrors, eventErrors int
	baseline, events, reconciliations, polls   int
	collectionTime                             time.Duration
}

func (d *watchDiagnostics) collected(reason collectionReason, elapsed time.Duration, err error) {
	if d == nil {
		return
	}
	d.collections++
	d.collectionTime += elapsed
	if err != nil {
		d.collectionErrors++
	}
	switch reason {
	case collectionBaseline:
		d.baseline++
	case collectionEvent:
		d.events++
	case collectionReconcile:
		d.reconciliations++
	case collectionPoll:
		d.polls++
	}
}

func (d *watchDiagnostics) write(out io.Writer, backend string, opts socketOptions, events platform.SocketEvents) error {
	notifications, coalesced, drops, source := "not-applicable", "not-applicable", "not-applicable", "none"
	if backend == "events" {
		notifications, coalesced, drops = "unavailable", "unavailable", "unavailable"
		switch runtime.GOOS {
		case "linux":
			source = "bpftrace"
		case "darwin":
			source = "dtrace"
		case "windows":
			source = "etw"
		default:
			source = runtime.GOOS
		}
		if diagnostics, ok := events.(platform.SocketEventDiagnostics); ok {
			stats := diagnostics.EventStats()
			notifications = fmt.Sprint(stats.Notifications)
			coalesced = fmt.Sprint(stats.Coalesced)
		}
	}
	_, err := fmt.Fprintf(out, "watch diagnostics: backend=%s collections=%d collection_total=%s reconcile=%s source=%s baseline=%d events=%d reconciliations=%d polls=%d collection_errors=%d event_errors=%d notifications=%s coalesced=%s backend_drops=%s event_interval=%s\n", backend, d.collections, d.collectionTime, opts.reconcile, source, d.baseline, d.events, d.reconciliations, d.polls, d.collectionErrors, d.eventErrors, notifications, coalesced, drops, opts.eventInterval)
	return err
}
