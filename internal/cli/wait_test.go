package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

func TestWaitConditionsAndFilters(t *testing.T) {
	fixture := fixtureSocket()
	other := fixture
	other.Local = "127.0.0.1:9000"
	for _, until := range []string{"present", "absent"} {
		t.Run(until, func(t *testing.T) {
			calls := 0
			rows, err := waitForSockets(context.Background(), socketFilter{port: 8080, pid: 10, protocol: "tcp", listening: true}, until, time.Millisecond, func() ([]platform.Socket, error) {
				calls++
				present := (until == "present" && calls == 2) || (until == "absent" && calls == 1)
				if present {
					return []platform.Socket{other, fixture}, nil
				}
				return []platform.Socket{other}, nil
			})
			if err != nil || calls != 2 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
			if until == "present" && (len(rows) != 1 || rows[0].Local != fixture.Local) {
				t.Fatalf("bad matched rows: %+v", rows)
			}
			if until == "absent" && (rows == nil || len(rows) != 0) {
				t.Fatalf("expected empty array, got %+v", rows)
			}
		})
	}
}

func TestWaitImmediateCondition(t *testing.T) {
	for _, until := range []string{"present", "absent"} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		calls := 0
		_, err := waitForSockets(ctx, socketFilter{}, until, time.Hour, func() ([]platform.Socket, error) {
			calls++
			if until == "present" {
				return []platform.Socket{fixtureSocket()}, nil
			}
			return nil, nil
		})
		if err != nil || ctx.Err() != nil || calls != 1 {
			t.Fatalf("%s: err=%v calls=%d", until, err, calls)
		}
		cancel()
	}
}

func TestWaitTimeoutDuringSleep(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	calls := 0
	_, err := waitForSockets(ctx, socketFilter{}, "present", time.Hour, func() ([]platform.Socket, error) { calls++; return nil, nil })
	if !errors.Is(err, context.DeadlineExceeded) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestWaitRejectsLateSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := waitForSockets(ctx, socketFilter{}, "present", time.Hour, func() ([]platform.Socket, error) {
		<-ctx.Done()
		return []platform.Socket{fixtureSocket()}, nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("late success accepted: %v", err)
	}
}

func TestWaitCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := waitForSockets(ctx, socketFilter{}, "present", time.Hour, func() ([]platform.Socket, error) {
		t.Fatal("collected after cancellation")
		return nil, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	_, err = waitForSockets(ctx, socketFilter{}, "present", time.Hour, func() ([]platform.Socket, error) {
		cancel()
		return []platform.Socket{fixtureSocket()}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("accepted success after cancellation", err)
	}
}

func TestWaitCollectionErrors(t *testing.T) {
	want := errors.New("proc unreadable")
	_, err := waitForSockets(context.Background(), socketFilter{}, "absent", time.Millisecond, func() ([]platform.Socket, error) { return nil, want })
	if !errors.Is(err, want) {
		t.Fatalf("collector error lost: %v", err)
	}
	_, err = waitForSockets(context.Background(), socketFilter{}, "present", time.Millisecond, func() ([]platform.Socket, error) {
		return []platform.Socket{{Local: "invalid"}}, nil
	})
	if err == nil {
		t.Fatal("filter error lost")
	}
}

func TestWaitArguments(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		code    int
		message string
	}{
		{[]string{"wait", "--help"}, 0, "timeout"},
		{[]string{"wait"}, 1, "usage:"},
		{[]string{"wait", "8080", "--json"}, 1, "flags must precede"},
		{[]string{"wait", "0"}, 1, "port must"},
		{[]string{"wait", "65536"}, 1, "port must"},
		{[]string{"wait", "--port", "8080", "8080"}, 1, "usage:"},
		{[]string{"wait", "--until", "ready", "8080"}, 1, "present or absent"},
		{[]string{"wait", "--timeout", "-1s", "8080"}, 1, "zero or positive"},
		{[]string{"wait", "--timeout", "bad", "8080"}, 1, "invalid value"},
		{[]string{"wait", "--interval", "0s", "8080"}, 1, "at least 100ms"},
		{[]string{"wait", "--interval", "99ms", "8080"}, 1, "at least 100ms"},
		{[]string{"wait", "--pid", "0", "8080"}, 1, "PID must"},
		{[]string{"wait", "--protocol", "icmp", "8080"}, 1, "protocol must"},
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

func TestWaitContextExitCodes(t *testing.T) {
	for _, timeout := range []string{"30s", "0"} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var out, stderr bytes.Buffer
		code := RunContext(ctx, []string{"wait", "--timeout", timeout, "--json", "8080"}, strings.NewReader(""), &out, &stderr, "test")
		if code != 130 || out.Len() != 0 || !strings.Contains(stderr.String(), "interrupted") {
			t.Fatalf("code=%d output=%s%s", code, &out, &stderr)
		}
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	var out, stderr bytes.Buffer
	code := RunContext(ctx, []string{"wait", "--json", "8080"}, strings.NewReader(""), &out, &stderr, "test")
	if code != 2 || out.Len() != 0 || !strings.Contains(stderr.String(), "timed out") {
		t.Fatalf("code=%d output=%s%s", code, &out, &stderr)
	}
}

func TestWriteWaitResult(t *testing.T) {
	for _, condition := range []string{"present", "absent"} {
		rows := []platform.Socket{}
		if condition == "present" {
			rows = append(rows, fixtureSocket())
		}
		result := waitResult{Timestamp: time.Now().UTC(), Condition: condition, Port: 8080, Sockets: rows}
		var out bytes.Buffer
		if err := writeWaitResult(&out, result, true); err != nil {
			t.Fatal(err)
		}
		var decoded waitResult
		if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.Condition != condition || decoded.Port != 8080 || decoded.Sockets == nil {
			t.Fatalf("bad JSON: %s", &out)
		}
		out.Reset()
		if err := writeWaitResult(&out, result, false); err != nil || !strings.Contains(out.String(), condition) {
			t.Fatalf("%s %v", &out, err)
		}
		for _, asJSON := range []bool{false, true} {
			if err := writeWaitResult(brokenWriter{}, result, asJSON); err == nil {
				t.Fatal("output error lost")
			}
		}
		if condition == "present" {
			if err := writeWaitResult(&failAfterWriter{remaining: 1}, result, false); err == nil {
				t.Fatal("table output error lost")
			}
		}
	}
}
