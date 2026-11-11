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
	"github.com/Thuya-Myint/socklens/internal/watchsetup"
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

type testSocketEvents struct {
	wait func(context.Context) error
}

func (s testSocketEvents) Wait(ctx context.Context) error { return s.wait(ctx) }
func (s testSocketEvents) Close() error                   { return nil }

func TestWatchEventSnapshots(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls, waits := 0, 0
	events := testSocketEvents{wait: func(context.Context) error { waits++; return nil }}
	err := watchSnapshotsWithEvents(ctx, io.Discard, socketOptions{json: true}, time.Hour, 3, func() ([]platform.Socket, error) {
		calls++
		return nil, nil
	}, events)
	if err != nil || ctx.Err() != nil || calls != 3 || waits != 2 {
		t.Fatalf("err=%v calls=%d waits=%d", err, calls, waits)
	}
}

func TestWatchDefaultsRequireEvents(t *testing.T) {
	want := errors.New("tracing unavailable")
	var out, stderr bytes.Buffer
	collected := false
	code := watchCommandWithSources(context.Background(), nil, &out, &stderr,
		func(context.Context) (platform.SocketEvents, error) { return nil, want },
		func() ([]platform.Socket, error) { collected = true; return nil, nil })
	if code != 1 || collected || out.Len() != 0 || !strings.Contains(stderr.String(), want.Error()) {
		t.Fatalf("code=%d collected=%v out=%s stderr=%s", code, collected, &out, &stderr)
	}
}

func TestWatchRelaunchedDoesNotCollectOrFallBack(t *testing.T) {
	for _, args := range [][]string{nil, {"--backend", "auto"}} {
		var out, stderr bytes.Buffer
		code := watchCommandWithSources(context.Background(), args, &out, &stderr,
			func(context.Context) (platform.SocketEvents, error) { return nil, watchsetup.ErrRelaunched },
			func() ([]platform.Socket, error) { t.Fatal("collected after elevation"); return nil, nil })
		if code != 0 || out.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("code=%d out=%s err=%s", code, &out, &stderr)
		}
	}
}

func TestWatchDeclinedSetupDoesNotFallBack(t *testing.T) {
	var out, stderr bytes.Buffer
	code := watchCommandWithSources(context.Background(), []string{"--backend", "auto"}, &out, &stderr,
		func(context.Context) (platform.SocketEvents, error) { return nil, watchsetup.ErrSetupStopped },
		func() ([]platform.Socket, error) { t.Fatal("polled after setup was declined"); return nil, nil })
	if code != 1 || out.Len() != 0 || strings.Contains(stderr.String(), "using polling") {
		t.Fatalf("code=%d out=%s err=%s", code, &out, &stderr)
	}
}

func TestWatchDefaultsSuppressUnchangedEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var out, stderr bytes.Buffer
	calls, waits := 0, 0
	row := platform.Socket{Protocol: "tcp", Local: "127.0.0.1:8080", State: "LISTEN"}
	events := testSocketEvents{wait: func(context.Context) error { waits++; return nil }}
	code := watchCommandWithSources(ctx, []string{"--json", "--count", "3"}, &out, &stderr,
		func(context.Context) (platform.SocketEvents, error) { return events, nil },
		func() ([]platform.Socket, error) {
			calls++
			if calls == 3 {
				return nil, nil
			}
			return []platform.Socket{row}, nil
		})
	if code != 0 || ctx.Err() != nil || calls != 3 || waits != 2 {
		t.Fatalf("code=%d calls=%d waits=%d stderr=%s", code, calls, waits, &stderr)
	}
	decoder := json.NewDecoder(&out)
	var baseline, removal changeSet
	if err := decoder.Decode(&baseline); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&removal); err != nil {
		t.Fatal(err)
	}
	if !baseline.Baseline || len(baseline.Added) != 1 || removal.Baseline || len(removal.Removed) != 1 {
		t.Fatalf("baseline=%+v removal=%+v", baseline, removal)
	}
	var extra changeSet
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("unchanged event produced output: %v", err)
	}
}

func TestWatchEventFailure(t *testing.T) {
	want := errors.New("tracer disconnected")
	events := testSocketEvents{wait: func(context.Context) error { return want }}
	err := watchSnapshotsWithEvents(context.Background(), io.Discard, socketOptions{json: true}, time.Hour, 0, func() ([]platform.Socket, error) { return nil, nil }, events)
	if !errors.Is(err, want) {
		t.Fatalf("lost event error: %v", err)
	}
}

func TestWatchEventCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := testSocketEvents{wait: func(ctx context.Context) error { cancel(); return ctx.Err() }}
	if err := watchSnapshotsWithEvents(ctx, io.Discard, socketOptions{json: true}, time.Hour, 0, func() ([]platform.Socket, error) { return nil, nil }, events); err != nil {
		t.Fatal(err)
	}
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
	if err != nil || ctx.Err() != nil || !strings.Contains(out.String(), "Snapshot ") || !strings.Contains(out.String(), "No matching sockets.") {
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
		{[]string{"watch", "--backend", "invalid"}, 1, "backend must"},
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

func TestWatchReconcilesMissedEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	events := testSocketEvents{wait: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	var out bytes.Buffer
	opts := socketOptions{json: true, changes: true, reconcile: 10 * time.Millisecond}
	err := watchSnapshotsWithEvents(ctx, &out, opts, time.Hour, 2, func() ([]platform.Socket, error) {
		calls++
		if calls == 1 {
			return nil, nil
		}
		return []platform.Socket{{Protocol: "tcp", Local: "127.0.0.1:8080", State: "LISTEN"}}, nil
	}, events)
	if err != nil || calls != 2 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	var baseline, change changeSet
	decoder := json.NewDecoder(&out)
	if err := decoder.Decode(&baseline); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&change); err != nil {
		t.Fatal(err)
	}
	if !baseline.Baseline || len(change.Added) != 1 {
		t.Fatalf("baseline=%+v change=%+v", baseline, change)
	}
}
