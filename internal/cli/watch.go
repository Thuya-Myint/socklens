package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
	"github.com/Thuya-Myint/socklens/internal/watchsetup"
)

type snapshot struct {
	Timestamp time.Time         `json:"timestamp"`
	Sockets   []platform.Socket `json:"sockets"`
}

func watchCommand(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, colorMode string) int {
	restartArgs := append([]string{"--color=" + colorMode}, args...)
	return watchCommandWithSources(ctx, args, out, errOut, func(ctx context.Context) (platform.SocketEvents, error) {
		return watchsetup.Subscribe(ctx, restartArgs, in, out, errOut)
	}, platform.List)
}

func watchCommandWithSources(ctx context.Context, args []string, out, errOut io.Writer, subscribe func(context.Context) (platform.SocketEvents, error), list func() ([]platform.Socket, error)) int {
	fs := newCommandFlags("watch", errOut)
	opts := addSocketFlags(fs)
	fs.BoolVar(&opts.changes, "changes", true, "report only changes after an initial baseline")
	fs.BoolVar(&opts.stats, "stats", false, "print collection diagnostics to stderr when watch ends")
	interval := fs.Duration("interval", 2*time.Second, "polling delay (minimum 100ms; unused with events)")
	reconcile := fs.Duration("reconcile", 20*time.Second, "maximum time between event-mode snapshots (minimum 1s)")
	count := fs.Int("count", 0, "number of snapshots; 0 watches until interrupted")
	backend := fs.String("backend", "events", "watch backend: events (default), auto, or poll")
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
	if *reconcile < time.Second {
		return fail(errOut, fmt.Errorf("reconcile must be at least 1s"))
	}
	if *count < 0 {
		return fail(errOut, fmt.Errorf("count must be zero or positive"))
	}
	if *backend != "auto" && *backend != "events" && *backend != "poll" {
		return fail(errOut, fmt.Errorf("backend must be auto, events, or poll"))
	}
	var events platform.SocketEvents
	selectedBackend := "snapshot"
	if *count != 1 {
		selectedBackend = "poll"
	}
	if *backend != "poll" && *count != 1 && ctx.Err() == nil {
		var err error
		events, err = subscribe(ctx)
		if events != nil {
			defer events.Close()
		}
		if ctx.Err() != nil {
			return 0
		}
		if errors.Is(err, watchsetup.ErrRelaunched) {
			return 0
		}
		if errors.Is(err, watchsetup.ErrSetupStopped) {
			return fail(errOut, err)
		}
		if err != nil {
			if *backend == "events" {
				return fail(errOut, err)
			}
			fmt.Fprintln(errOut, "watch: using polling:", err)
		} else {
			selectedBackend = "events"
			fmt.Fprintf(errOut, "watch: using %s socket events\n", runtime.GOOS)
		}
	}
	opts.reconcile = *reconcile
	if opts.stats {
		var collections int
		var total time.Duration
		original := list
		list = func() ([]platform.Socket, error) {
			start := time.Now()
			rows, err := original()
			collections++
			total += time.Since(start)
			return rows, err
		}
		defer func() {
			fmt.Fprintf(errOut, "watch diagnostics: backend=%s collections=%d collection_total=%s reconcile=%s\n", selectedBackend, collections, total, opts.reconcile)
		}()
	}
	if err := watchSnapshotsWithEvents(ctx, out, *opts, *interval, *count, list, events); err != nil {
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
			reconcile := opts.reconcile
			if reconcile == 0 {
				reconcile = 20 * time.Second
			}
			waitCtx, cancel := context.WithTimeout(ctx, reconcile)
			err := events.Wait(waitCtx)
			cancel()
			if err != nil && errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
				// A reconciliation snapshot recovers changes missed by the event source.
				continue
			}
			if err != nil {
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
