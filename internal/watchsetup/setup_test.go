package watchsetup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

type testEvents struct{}

func (testEvents) Wait(context.Context) error { return nil }
func (testEvents) Close() error               { return nil }

func TestReadyTracingNeedsNoSetup(t *testing.T) {
	s := setup{interactive: true, subscribe: func(context.Context) (platform.SocketEvents, error) { return testEvents{}, nil },
		installPlan: func() ([]command, error) { t.Fatal("installation offered when tracing works"); return nil, nil },
		canElevate:  func() (bool, error) { t.Fatal("elevated when tracing works"); return true, nil },
	}
	var output bytes.Buffer
	source, err := s.subscribeWithSetup(context.Background(), nil, strings.NewReader(""), &output)
	if err != nil || source == nil || output.Len() != 0 {
		t.Fatalf("source=%v err=%v out=%s", source, err, &output)
	}
}

func TestInstallRequiresCompleteConsent(t *testing.T) {
	for _, answer := range []string{"", "\n", "n\n", "yes", strings.Repeat("y", 65) + "\n"} {
		t.Run(answer, func(t *testing.T) {
			s := setup{interactive: true, subscribe: func(context.Context) (platform.SocketEvents, error) { return nil, platform.ErrEventHelperMissing },
				installPlan: func() ([]command, error) {
					return []command{{path: "package-manager", args: []string{"install", "bpftrace"}}}, nil
				},
				run: func(context.Context, command) error { t.Fatal("installed without consent"); return nil },
			}
			var out bytes.Buffer
			if _, err := s.subscribeWithSetup(context.Background(), nil, strings.NewReader(answer), &out); !errors.Is(err, platform.ErrEventHelperMissing) {
				t.Fatalf("err=%v", err)
			}
			if !strings.Contains(out.String(), "package-manager install bpftrace") || !strings.Contains(out.String(), "[y/N]") {
				t.Fatalf("prompt=%s", &out)
			}
		})
	}
}

func TestInstallPromptCancellation(t *testing.T) {
	in, end := io.Pipe()
	defer in.Close()
	defer end.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	s := setup{interactive: true, subscribe: func(context.Context) (platform.SocketEvents, error) { return nil, platform.ErrEventHelperMissing },
		installPlan: func() ([]command, error) { return []command{{path: "package-manager"}}, nil },
		run:         func(context.Context, command) error { t.Fatal("installed after cancellation"); return nil },
	}
	if _, err := s.subscribeWithSetup(ctx, nil, in, io.Discard); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
}

func TestInstallRetriesWatch(t *testing.T) {
	calls := 0
	var ran []command
	plan := []command{{path: "package-manager", args: []string{"refresh"}}, {path: "package-manager", args: []string{"install", "bpftrace"}}}
	s := setup{interactive: true,
		subscribe: func(context.Context) (platform.SocketEvents, error) {
			calls++
			if calls == 1 {
				return nil, platform.ErrEventHelperMissing
			}
			return testEvents{}, nil
		},
		installPlan: func() ([]command, error) { return plan, nil },
		run:         func(_ context.Context, c command) error { ran = append(ran, c); return nil },
	}
	source, err := s.subscribeWithSetup(context.Background(), nil, strings.NewReader("yes\n"), io.Discard)
	if err != nil || source == nil || calls != 2 || !reflect.DeepEqual(ran, plan) {
		t.Fatalf("source=%v err=%v calls=%d ran=%v", source, err, calls, ran)
	}
}

func TestInstallFailureStopsWatch(t *testing.T) {
	want := errors.New("package unavailable")
	calls := 0
	s := setup{interactive: true,
		subscribe: func(context.Context) (platform.SocketEvents, error) {
			calls++
			return nil, platform.ErrEventHelperMissing
		},
		installPlan: func() ([]command, error) { return []command{{path: "package-manager"}}, nil },
		run:         func(context.Context, command) error { return want },
	}
	_, err := s.subscribeWithSetup(context.Background(), nil, strings.NewReader("y\n"), io.Discard)
	if !errors.Is(err, want) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestNonInteractiveNeverInstallsOrElevates(t *testing.T) {
	for _, want := range []error{platform.ErrEventHelperMissing, platform.ErrEventPrivileges} {
		s := setup{subscribe: func(context.Context) (platform.SocketEvents, error) { return nil, want },
			installPlan: func() ([]command, error) { t.Fatal("planned installation without a terminal"); return nil, nil },
			canElevate:  func() (bool, error) { t.Fatal("elevation attempted without a terminal"); return true, nil },
		}
		if _, err := s.subscribeWithSetup(context.Background(), nil, strings.NewReader("yes\n"), io.Discard); !errors.Is(err, want) {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestElevationPreservesArguments(t *testing.T) {
	args := []string{"--port", "8080", "--pid", "42"}
	s := setup{interactive: true, subscribe: func(context.Context) (platform.SocketEvents, error) { return nil, platform.ErrEventPrivileges },
		canElevate: func() (bool, error) { return true, nil },
		elevate: func(_ context.Context, got []string) error {
			if !reflect.DeepEqual(got, args) {
				t.Fatalf("args=%v", got)
			}
			return nil
		},
	}
	_, err := s.subscribeWithSetup(context.Background(), args, strings.NewReader(""), io.Discard)
	if !errors.Is(err, ErrRelaunched) {
		t.Fatalf("err=%v", err)
	}
}

func TestAlreadyElevatedDoesNotLoop(t *testing.T) {
	s := setup{interactive: true, subscribe: func(context.Context) (platform.SocketEvents, error) { return nil, platform.ErrEventPrivileges },
		canElevate: func() (bool, error) { return false, nil },
		elevate:    func(context.Context, []string) error { t.Fatal("repeated elevation"); return nil },
	}
	if _, err := s.subscribeWithSetup(context.Background(), nil, strings.NewReader(""), io.Discard); !errors.Is(err, platform.ErrEventPrivileges) {
		t.Fatalf("err=%v", err)
	}
}

func TestElevationDenied(t *testing.T) {
	want := errors.New("UAC cancelled")
	s := setup{interactive: true, subscribe: func(context.Context) (platform.SocketEvents, error) { return nil, platform.ErrEventPrivileges },
		canElevate: func() (bool, error) { return true, nil }, elevate: func(context.Context, []string) error { return want },
	}
	if _, err := s.subscribeWithSetup(context.Background(), nil, strings.NewReader(""), io.Discard); !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}

func TestUnsupportedTracingDoesNotElevate(t *testing.T) {
	want := errors.New("SIP restricts this probe")
	s := setup{interactive: true, subscribe: func(context.Context) (platform.SocketEvents, error) { return nil, want },
		canElevate: func() (bool, error) { t.Fatal("elevated for an unrelated error"); return true, nil },
	}
	if _, err := s.subscribeWithSetup(context.Background(), nil, strings.NewReader(""), io.Discard); !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}
