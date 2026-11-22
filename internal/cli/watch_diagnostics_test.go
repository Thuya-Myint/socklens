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

type diagnosticSocketEvents struct{ testSocketEvents }

func (diagnosticSocketEvents) EventStats() platform.SocketEventStats {
	return platform.SocketEventStats{Notifications: 100, Coalesced: 98}
}

func TestWatchDiagnosticsEventAndReconciliation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	waits := 0
	source := diagnosticSocketEvents{testSocketEvents{wait: func(waitCtx context.Context) error {
		waits++
		if waits == 1 {
			return nil
		}
		<-waitCtx.Done()
		return waitCtx.Err()
	}}}
	d := &watchDiagnostics{}
	opts := socketOptions{json: true, changes: true, reconcile: 5 * time.Millisecond, diagnostics: d}
	var out bytes.Buffer
	if err := watchSnapshotsWithEvents(ctx, &out, opts, time.Hour, 4, func() ([]platform.Socket, error) { return nil, nil }, source); err != nil {
		t.Fatal(err)
	}
	if d.collections != 4 || d.baseline != 1 || d.events != 1 || d.reconciliations != 2 || d.polls != 0 || d.eventErrors != 0 || d.collectionErrors != 0 {
		t.Fatalf("diagnostics=%+v", d)
	}
	if strings.Count(out.String(), "\n") != 1 {
		t.Fatal("unchanged observations emitted extra JSON")
	}
	var stderr bytes.Buffer
	if err := d.write(&stderr, "events", opts, source); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"baseline=1 events=1 reconciliations=2 polls=0", "notifications=100 coalesced=98 backend_drops=unavailable"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("missing %q: %s", want, &stderr)
		}
	}
}

func TestWatchBurstCollectionsHaveBoundedRate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	d := &watchDiagnostics{}
	const gap = 10 * time.Millisecond
	var previous time.Time
	collection := func() ([]platform.Socket, error) {
		if !previous.IsZero() && time.Since(previous) < gap {
			t.Fatal("burst collection bypassed minimum gap")
		}
		previous = time.Now()
		return nil, nil
	}
	source := testSocketEvents{wait: func(context.Context) error { return nil }}
	err := watchSnapshotsWithEvents(ctx, io.Discard, socketOptions{json: true, diagnostics: d, eventInterval: gap}, time.Hour, 5, collection, source)
	if err != nil || d.collections != 5 || d.events != 4 {
		t.Fatalf("err=%v stats=%+v", err, d)
	}
}

func TestWatchCancellationDuringBurstDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := &watchDiagnostics{}
	source := testSocketEvents{wait: func(context.Context) error { time.AfterFunc(5*time.Millisecond, cancel); return nil }}
	err := watchSnapshotsWithEvents(ctx, io.Discard, socketOptions{json: true, diagnostics: d, eventInterval: time.Hour}, time.Hour, 0, func() ([]platform.Socket, error) { return nil, nil }, source)
	if err != nil || d.collections != 1 {
		t.Fatalf("err=%v stats=%+v", err, d)
	}
}

func TestWatchDiagnosticErrorsKeepStdoutJSON(t *testing.T) {
	for _, collectionFailure := range []bool{false, true} {
		var out, stderr bytes.Buffer
		source := diagnosticSocketEvents{testSocketEvents{wait: func(context.Context) error { return errors.New("event source failed") }}}
		code := watchCommandWithSources(context.Background(), []string{"--stats", "--json"}, &out, &stderr,
			func(context.Context) (platform.SocketEvents, error) { return source, nil },
			func() ([]platform.Socket, error) {
				if collectionFailure {
					return nil, errors.New("collection failed")
				}
				return nil, nil
			})
		want := "collection_errors=0 event_errors=1"
		if collectionFailure {
			want = "collection_errors=1 event_errors=0"
		}
		if code != 1 || !strings.Contains(stderr.String(), want) || strings.Contains(out.String(), "diagnostics") {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, &out, &stderr)
		}
		if !collectionFailure {
			var baseline changeSet
			if err := json.Unmarshal(out.Bytes(), &baseline); err != nil || !baseline.Baseline {
				t.Fatalf("invalid baseline: %s %v", &out, err)
			}
		}
	}
}

func TestWatchAutoFallbackDiscardsFailedSource(t *testing.T) {
	var out, stderr bytes.Buffer
	source := testSocketEvents{wait: func(context.Context) error { t.Fatal("used event source after polling fallback"); return nil }}
	code := watchCommandWithSources(context.Background(), []string{"--backend=auto", "--stats", "--count=2", "--interval=100ms"}, &out, &stderr,
		func(context.Context) (platform.SocketEvents, error) { return source, errors.New("attachment failed") },
		func() ([]platform.Socket, error) { return nil, nil })
	if code != 0 || !strings.Contains(stderr.String(), "backend=poll collections=2") || !strings.Contains(stderr.String(), "baseline=1 events=0 reconciliations=0 polls=1") {
		t.Fatalf("code=%d stderr=%s", code, &stderr)
	}
}

func TestWatchDiagnosticsFailures(t *testing.T) {
	for _, backendErr := range []error{errors.New("tracer failed"), context.DeadlineExceeded} {
		d := &watchDiagnostics{}
		source := testSocketEvents{wait: func(context.Context) error { return backendErr }}
		err := watchSnapshotsWithEvents(context.Background(), io.Discard, socketOptions{json: true, diagnostics: d}, time.Hour, 0, func() ([]platform.Socket, error) { return nil, nil }, source)
		if !errors.Is(err, backendErr) || d.collections != 1 || d.eventErrors != 1 || d.reconciliations != 0 {
			t.Fatalf("err=%v stats=%+v", err, d)
		}
	}
	d := &watchDiagnostics{}
	want := errors.New("discovery failed")
	err := watchSnapshots(context.Background(), io.Discard, socketOptions{diagnostics: d}, time.Hour, 1, func() ([]platform.Socket, error) { return nil, want })
	if !errors.Is(err, want) || d.collectionErrors != 1 || d.baseline != 1 {
		t.Fatalf("err=%v stats=%+v", err, d)
	}
}

func TestWatchDiagnosticsUnknownAndPolling(t *testing.T) {
	d := &watchDiagnostics{}
	for _, backend := range []string{"events", "poll", "snapshot"} {
		var out bytes.Buffer
		if err := d.write(&out, backend, socketOptions{}, testSocketEvents{}); err != nil {
			t.Fatal(err)
		}
		want := "notifications=not-applicable coalesced=not-applicable backend_drops=not-applicable"
		if backend == "events" {
			want = "notifications=unavailable coalesced=unavailable backend_drops=unavailable"
		}
		if !strings.Contains(out.String(), want) {
			t.Fatalf("%s: %s", backend, &out)
		}
	}
	if err := d.write(brokenWriter{}, "poll", socketOptions{}, nil); err == nil {
		t.Fatal("lost diagnostics output error")
	}
	d = &watchDiagnostics{}
	if err := watchSnapshots(context.Background(), io.Discard, socketOptions{json: true, diagnostics: d}, time.Millisecond, 3, func() ([]platform.Socket, error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	if d.baseline != 1 || d.polls != 2 || d.events != 0 {
		t.Fatalf("stats=%+v", d)
	}
}

func TestWatchEventIntervalValidation(t *testing.T) {
	for _, value := range []string{"bad", "0s", "99ms", "21s"} {
		var out, stderr bytes.Buffer
		code := watchCommandWithSources(context.Background(), []string{"--event-interval", value}, &out, &stderr, func(context.Context) (platform.SocketEvents, error) {
			t.Fatal("subscribed with invalid interval")
			return nil, nil
		}, func() ([]platform.Socket, error) { t.Fatal("collected with invalid interval"); return nil, nil })
		if code != 1 || stderr.Len() == 0 {
			t.Fatalf("%s: code=%d stderr=%s", value, code, &stderr)
		}
	}
}
