package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

func TestDoctorReportAndFailures(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		args := []string{}
		if asJSON {
			args = append(args, "--json")
		}
		for _, collectErr := range []error{nil, errors.New("discovery unavailable")} {
			var out, stderr bytes.Buffer
			list := func() ([]platform.Socket, error) { return []platform.Socket{{Protocol: "tcp"}}, collectErr }
			code := doctorCommandWithSource(context.Background(), args, &out, &stderr, list)
			if (code != 0) != (collectErr != nil) {
				t.Fatalf("code=%d err=%v", code, collectErr)
			}
			if asJSON {
				var report map[string]any
				if err := json.Unmarshal(out.Bytes(), &report); err != nil || report["event_backend"] == "" {
					t.Fatalf("invalid report: %s (%v)", &out, err)
				}
			} else if !strings.Contains(out.String(), "Socket discovery:") {
				t.Fatal(out.String())
			}
			if code := doctorCommandWithSource(context.Background(), args, brokenWriter{}, &stderr, list); code != 1 {
				t.Fatalf("write failure returned %d", code)
			}
		}
	}
}

func TestDoctorCancellationAndArguments(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, stderr bytes.Buffer
	if code := doctorCommandWithSource(ctx, nil, &out, &stderr, func() ([]platform.Socket, error) {
		t.Fatal("collected after cancellation")
		return nil, nil
	}); code != 0 || out.Len() != 0 {
		t.Fatalf("code=%d out=%s", code, &out)
	}
	for _, args := range [][]string{{"extra"}, {"--unknown"}} {
		if code := doctorCommandWithSource(context.Background(), args, &out, &stderr, func() ([]platform.Socket, error) {
			t.Fatal("collected with invalid arguments")
			return nil, nil
		}); code != 1 {
			t.Fatalf("code=%d", code)
		}
	}
}
