package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

type snapshot struct {
	Timestamp time.Time         `json:"timestamp"`
	Sockets   []platform.Socket `json:"sockets"`
}

func watchCommand(ctx context.Context, args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(errOut)
	opts := addSocketFlags(fs)
	fs.BoolVar(&opts.changes, "changes", false, "report only changes after an initial baseline")
	interval := fs.Duration("interval", 2*time.Second, "delay between snapshots (minimum 100ms)")
	count := fs.Int("count", 0, "number of snapshots; 0 watches until interrupted")
	if err := fs.Parse(args); err != nil {
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
	if err := watchSnapshots(ctx, out, *opts, *interval, *count, platform.List); err != nil {
		return fail(errOut, err)
	}
	return 0
}

// watchSnapshots takes its first snapshot immediately, then waits after each
// completed snapshot. Collection is synchronous and never overlaps.
func watchSnapshots(ctx context.Context, out io.Writer, opts socketOptions, interval time.Duration, count int, list func() ([]platform.Socket, error)) error {
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
			if _, err := fmt.Fprintf(out, "Snapshot %s\n", observed.Format(time.RFC3339Nano)); err != nil {
				return err
			}
			if err := writeSockets(out, rows, false); err != nil {
				return err
			}
		}
		if count > 0 && n+1 == count {
			return nil
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
