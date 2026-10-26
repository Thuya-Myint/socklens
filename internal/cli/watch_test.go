package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

func TestWatchSnapshotsStream(t *testing.T) {
	calls := 0
	list := func() ([]platform.Socket, error) {
		calls++
		if calls == 2 {
			return nil, nil
		}
		return []platform.Socket{
			{Protocol: "tcp", Local: "[::1]:8080", State: "LISTEN", PIDs: []int{10}},
			{Protocol: "tcp", Local: "127.0.0.1:9000", State: "ESTABLISHED", PIDs: []int{20}},
		}, nil
	}
	var out bytes.Buffer
	opts := socketOptions{json: true, filter: socketFilter{port: 8080, pid: 10, listening: true}}
	if err := watchSnapshots(context.Background(), &out, opts, time.Millisecond, 3, list); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if calls != 3 || len(lines) != 3 {
		t.Fatalf("calls=%d lines=%d", calls, len(lines))
	}
	for i, line := range lines {
		var s snapshot
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			t.Fatal(err)
		}
		if s.Timestamp.IsZero() {
			t.Fatal("missing timestamp")
		}
		if i == 1 {
			if s.Sockets == nil || len(s.Sockets) != 0 {
				t.Fatalf("empty snapshot should contain []: %s", line)
			}
		} else if len(s.Sockets) != 1 || s.Sockets[0].Local != "[::1]:8080" {
			t.Fatalf("filters not applied: %s", line)
		}
	}
}

func TestWatchCancellationBeforeCollection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := watchSnapshots(ctx, io.Discard, socketOptions{}, time.Hour, 0, func() ([]platform.Socket, error) {
		called = true
		return nil, nil
	})
	if err != nil || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

type cancelWriter struct {
	cancel context.CancelFunc
	calls  int
}

func (w *cancelWriter) Write(p []byte) (int, error) {
	w.calls++
	w.cancel()
	return len(p), nil
}

func TestWatchCancellationDuringWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out := &cancelWriter{cancel: cancel}
	collections := 0
	err := watchSnapshots(ctx, out, socketOptions{json: true}, time.Hour, 0, func() ([]platform.Socket, error) {
		collections++
		return nil, nil
	})
	if err != nil || collections != 1 || out.calls != 1 {
		t.Fatalf("err=%v collections=%d writes=%d", err, collections, out.calls)
	}
}

func TestWatchCancellationDuringCollection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out bytes.Buffer
	err := watchSnapshots(ctx, &out, socketOptions{}, time.Hour, 0, func() ([]platform.Socket, error) {
		cancel()
		return nil, errors.New("interrupted read")
	})
	if err != nil || out.Len() != 0 {
		t.Fatalf("err=%v output=%s", err, &out)
	}
}

func TestWatchSingleSnapshotDoesNotWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var out bytes.Buffer
	err := watchSnapshots(ctx, &out, socketOptions{}, time.Hour, 1, func() ([]platform.Socket, error) { return nil, nil })
	if err != nil || ctx.Err() != nil || !strings.Contains(out.String(), "Snapshot ") || !strings.Contains(out.String(), "PROCESSES") {
		t.Fatalf("err=%v context=%v output=%s", err, ctx.Err(), &out)
	}
}

func TestWatchErrors(t *testing.T) {
	collectErr := errors.New("proc unavailable")
	err := watchSnapshots(context.Background(), io.Discard, socketOptions{}, time.Millisecond, 1, func() ([]platform.Socket, error) {
		return nil, collectErr
	})
	if !errors.Is(err, collectErr) {
		t.Fatalf("lost collector error: %v", err)
	}
	err = watchSnapshots(context.Background(), io.Discard, socketOptions{}, time.Millisecond, 1, func() ([]platform.Socket, error) {
		return []platform.Socket{{Local: "bad"}}, nil
	})
	if err == nil {
		t.Fatal("lost filter error")
	}
	for _, asJSON := range []bool{false, true} {
		err = watchSnapshots(context.Background(), brokenWriter{}, socketOptions{json: asJSON}, time.Millisecond, 1, func() ([]platform.Socket, error) {
			return nil, nil
		})
		if err == nil {
			t.Fatal("lost output error")
		}
	}
}

func TestWatchArguments(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		code    int
		message string
	}{
		{[]string{"watch", "--help"}, 0, "interval"},
		{[]string{"watch", "--interval", "bad"}, 1, "invalid value"},
		{[]string{"watch", "--interval", "0s"}, 1, "at least 100ms"},
		{[]string{"watch", "--interval", "-1s"}, 1, "at least 100ms"},
		{[]string{"watch", "--interval", "99ms"}, 1, "at least 100ms"},
		{[]string{"watch", "--count", "-1"}, 1, "zero or positive"},
		{[]string{"watch", "--port", "0"}, 1, "port must"},
		{[]string{"watch", "--pid", "-1"}, 1, "PID must"},
		{[]string{"watch", "--protocol", "icmp"}, 1, "protocol must"},
		{[]string{"watch", "8080"}, 1, "unexpected arguments"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var out, stderr bytes.Buffer
			code := Run(tc.args, strings.NewReader(""), &out, &stderr, "test")
			if code != tc.code || !strings.Contains(out.String()+stderr.String(), tc.message) {
				t.Fatalf("code=%d output=%s%s", code, &out, &stderr)
			}
		})
	}
}
