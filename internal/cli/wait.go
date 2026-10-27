package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
	"github.com/Thuya-Myint/socklens/internal/terminal"
)

type waitResult struct {
	Timestamp time.Time         `json:"timestamp"`
	Condition string            `json:"condition"`
	Port      int               `json:"port"`
	Sockets   []platform.Socket `json:"sockets"`
}

func waitCommand(ctx context.Context, args []string, out, errOut io.Writer) int {
	fs := newCommandFlags("wait", errOut)
	opts := addSocketFlags(fs)
	until := fs.String("until", "present", "socket condition: present or absent")
	timeout := fs.Duration("timeout", 30*time.Second, "maximum wait; 0 waits until interrupted")
	interval := fs.Duration("interval", 500*time.Millisecond, "delay between polls (minimum 100ms)")
	if err := parseCommandFlags(fs, args, errOut); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if err := opts.validate(fs); err != nil {
		return fail(errOut, err)
	}
	if *until != "present" && *until != "absent" {
		return fail(errOut, fmt.Errorf("until must be present or absent"))
	}
	if *timeout < 0 {
		return fail(errOut, fmt.Errorf("timeout must be zero or positive"))
	}
	if *interval < 100*time.Millisecond {
		return fail(errOut, fmt.Errorf("interval must be at least 100ms"))
	}
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	rows, err := waitForSockets(ctx, opts.filter, *until, *interval, platform.List)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			_ = status(errOut, terminal.Warning, "TIMEOUT", fmt.Sprintf("socklens: timed out waiting for port %d to become %s", opts.filter.port, *until))
			return 2
		case errors.Is(err, context.Canceled):
			_ = status(errOut, terminal.Info, "CANCELLED", "socklens: wait interrupted")
			return 130
		default:
			return fail(errOut, err)
		}
	}
	result := waitResult{Timestamp: time.Now().UTC(), Condition: *until, Port: opts.filter.port, Sockets: rows}
	if err := writeWaitResult(out, result, opts.json); err != nil {
		return fail(errOut, err)
	}
	return 0
}

// waitForSockets observes local tables, without probing or connecting to a port.
// The initial poll is immediate. Cancellation is checked after collection too,
// so a result collected after the deadline cannot be reported as success.
func waitForSockets(ctx context.Context, filter socketFilter, until string, interval time.Duration, list func() ([]platform.Socket, error)) ([]platform.Socket, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rows, err := list()
		if stopped := ctx.Err(); stopped != nil {
			return nil, stopped
		}
		if err != nil {
			return nil, fmt.Errorf("collect wait snapshot: %w", err)
		}
		rows, err = filterSockets(rows, filter)
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if (until == "present" && len(rows) > 0) || (until == "absent" && len(rows) == 0) {
			return rows, nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func writeWaitResult(out io.Writer, result waitResult, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(out).Encode(result)
	}
	if err := status(out, terminal.Success, "OK", fmt.Sprintf("Port %d condition %q matched (%d sockets).", result.Port, result.Condition, len(result.Sockets))); err != nil {
		return err
	}
	if len(result.Sockets) > 0 {
		return writeSockets(out, result.Sockets, false)
	}
	return nil
}
