package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

type snapshot struct {
	Timestamp time.Time         `json:"timestamp"`
	Sockets   []platform.Socket `json:"sockets"`
}

func watchCommand(ctx context.Context, args []string, out, errOut io.Writer) int {
	fs := newCommandFlags("watch", errOut)
	opts := addSocketFlags(fs)
	fs.BoolVar(&opts.changes, "changes", false, "report only changes after an initial baseline")
	interval := fs.Duration("interval", 2*time.Second, "polling delay (minimum 100ms; unused with events)")
	count := fs.Int("count", 0, "number of snapshots; 0 watches until interrupted")
	backend := fs.String("backend", "auto", "watch backend: auto, events (OS tracing), or poll")
	if err := parseCommandFlags(fs, args, errOut); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if err := opts.validate(fs); err != nil {
		return fail(errOut, err)
	}
	if *interval < 100*time.Millisecond {
		return fail(errOut, fmt.Errorf("interval must be at least 100ms"))
	}
	if *count < 0 {
		return fail(errOut, fmt.Errorf("count must be zero or positive"))
	}
	if *backend != "auto" && *backend != "events" && *backend != "poll" {
		return fail(errOut, fmt.Errorf("backend must be auto, events, or poll"))
	}
	var events platform.SocketEvents
	if *backend != "poll" && *count != 1 && ctx.Err() == nil {
		var err error
		events, err = platform.WatchSocketEvents(ctx)
		if ctx.Err() != nil {
			return 0
		}
		if err != nil {
			if *backend == "events" {
				return fail(errOut, err)
			}
			fmt.Fprintln(errOut, "watch: using polling:", err)
		} else {
			defer events.Close()
			fmt.Fprintf(errOut, "watch: using %s socket events\n", runtime.GOOS)
		}
	}
	if err := watchSnapshotsWithEvents(ctx, out, *opts, *interval, *count, platform.List, events); err != nil {
		return fail(errOut, err)
	}
	return 0
}

// watchSnapshots takes its first snapshot immediately, then waits after each
// completed snapshot. Collection is synchronous and never overlaps.
func watchSnapshots(ctx context.Context, out io.Writer, opts socketOptions, interval time.Duration, count int, list func() ([]platform.Socket, error)) error {
	return watchSnapshotsWithEvents(ctx, out, opts, interval, count, list, nil)
}

func watchSnapshotsWithEvents(ctx context.Context, out io.Writer, opts socketOptions, interval time.Duration, count int, list func() ([]platform.Socket, error), events platform.SocketEvents) error {
	enc := json.NewEncoder(out)
	var previous []platform.Socket
	for n := 0; count == 0 || n < count; n++ {
		if ctx.Err() != nil {
			return nil
		}
		rows, err := list()
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return fmt.Errorf("collect snapshot: %w", err)
		}
		rows, err = filterSockets(rows, opts.filter)
		if err != nil {
			return err
		}
		observed := time.Now().UTC()
		if opts.changes {
			delta := diffSockets(previous, rows)
			delta.Timestamp = observed
			delta.Baseline = n == 0
			if delta.Baseline || delta.hasChanges() {
				if err := writeChanges(out, delta, opts.json); err != nil {
					return err
				}
			}
			previous = cloneSockets(rows)
		} else if opts.json {
			if err := enc.Encode(snapshot{Timestamp: observed, Sockets: rows}); err != nil {
				return err
			}
		} else {
			if err := writeObservationHeading(out, "Snapshot", fmt.Sprintf("%s | %s", observed.Format("2006-01-02 15:04:05.000Z07:00"), socketCount(len(rows)))); err != nil {
				return err
			}
			if err := writeSockets(out, rows, false); err != nil {
				return err
			}
		}
		if count > 0 && n+1 == count {
			return nil
		}
		if events != nil {
			if err := events.Wait(ctx); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("watch events: %w", err)
			}
			continue
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
	return nil
}
