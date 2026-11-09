// Package watchsetup guides interactive watch users through tracing prerequisites.
package watchsetup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
	"github.com/Thuya-Myint/socklens/internal/terminal"
)

// ErrRelaunched means Windows opened the watch in an administrator window.
var ErrRelaunched = errors.New("watch started in administrator window")

// ErrSetupStopped prevents an explicitly declined/failed interactive action
// from being treated as permission to start polling by the auto backend.
var ErrSetupStopped = errors.New("watch setup stopped")

type command struct {
	path string
	args []string
}
type setup struct {
	subscribe   func(context.Context) (platform.SocketEvents, error)
	installPlan func() ([]command, error)
	run         func(context.Context, command) error
	canElevate  func() (bool, error)
	elevate     func(context.Context, []string) error
	interactive bool
}

func Subscribe(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) (platform.SocketEvents, error) {
	input, ok := in.(*os.File)
	interactive := ok && terminal.IsTerminal(input) && terminal.IsTerminal(out) && terminal.IsTerminal(errOut)
	for _, arg := range args {
		if name, _, _ := strings.Cut(arg, "="); name == "--json" || name == "-json" {
			interactive = false
		}
	}
	s := setup{
		subscribe: platform.WatchSocketEvents, installPlan: installPlan, canElevate: canElevate, elevate: elevateWatch,
		interactive: interactive,
		run: func(ctx context.Context, step command) error {
			cmd := exec.CommandContext(ctx, step.path, step.args...)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = in, errOut, errOut
			// Let sudo forward interruption and the package manager release its
			// locks instead of killing only the sudo monitor immediately.
			cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
			cmd.WaitDelay = 5 * time.Second
			return cmd.Run()
		},
	}
	return s.subscribeWithSetup(ctx, args, in, errOut)
}

func (s setup) subscribeWithSetup(ctx context.Context, args []string, in io.Reader, errOut io.Writer) (platform.SocketEvents, error) {
	source, err := s.subscribe(ctx)
	if err == nil || ctx.Err() != nil {
		return source, err
	}
	if errors.Is(err, platform.ErrEventHelperMissing) {
		if !s.interactive {
			return nil, fmt.Errorf("%w; install the bpftrace package in an interactive terminal before watching", err)
		}
		plan, planErr := s.installPlan()
		if planErr != nil {
			return nil, fmt.Errorf("%w; %v", err, planErr)
		}
		if _, writeErr := fmt.Fprintln(errOut, "Watch needs bpftrace. Installation will run:"); writeErr != nil {
			return nil, writeErr
		}
		for _, step := range plan {
			if _, writeErr := fmt.Fprintln(errOut, " ", step.path, strings.Join(step.args, " ")); writeErr != nil {
				return nil, writeErr
			}
		}
		if _, writeErr := fmt.Fprint(errOut, "Install bpftrace now? [y/N] "); writeErr != nil {
			return nil, writeErr
		}
		if !confirmContext(ctx, in) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("%w: %w; installation cancelled", ErrSetupStopped, platform.ErrEventHelperMissing)
		}
		for _, step := range plan {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if installErr := s.run(ctx, step); installErr != nil {
				return nil, fmt.Errorf("%w: install bpftrace: %w", ErrSetupStopped, installErr)
			}
		}
		source, err = s.subscribe(ctx)
		if err == nil || ctx.Err() != nil {
			return source, err
		}
	}
	if !errors.Is(err, platform.ErrEventPrivileges) {
		return nil, err
	}
	if !s.interactive {
		return nil, fmt.Errorf("%w; use an interactive terminal to approve administrator access, or run from an already elevated terminal", err)
	}
	can, checkErr := s.canElevate()
	if checkErr != nil {
		return nil, fmt.Errorf("check tracing privileges: %w", checkErr)
	}
	if !can {
		return nil, err
	} // Already elevated: do not loop on SIP or tracing policy failures.
	message := "Watch needs administrator access. Your system will ask for permission."
	if runtime.GOOS == "windows" {
		message += " Watch will open in a new administrator window."
	}
	if _, writeErr := fmt.Fprintln(errOut, message); writeErr != nil {
		return nil, writeErr
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if elevateErr := s.elevate(ctx, args); elevateErr != nil {
		return nil, fmt.Errorf("%w: request administrator access: %w", ErrSetupStopped, elevateErr)
	}
	return nil, ErrRelaunched
}

func confirmContext(ctx context.Context, in io.Reader) bool {
	if ctx.Err() != nil {
		return false
	}
	answer := make(chan bool, 1)
	go func() { answer <- confirm(in) }()
	select {
	case <-ctx.Done():
		return false
	case yes := <-answer:
		return yes
	}
}

// Read only the confirmation line: never prefetch a future password or answer.
func confirm(in io.Reader) bool {
	var line []byte
	var b [1]byte
	for len(line) < 64 {
		if _, err := io.ReadFull(in, b[:]); err != nil {
			return false
		}
		if b[0] == '\n' {
			answer := strings.TrimSpace(strings.ToLower(string(line)))
			return answer == "y" || answer == "yes"
		}
		line = append(line, b[0])
	}
	return false
}
