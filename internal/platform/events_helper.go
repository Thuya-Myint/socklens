//go:build linux || darwin

package platform

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

type helperSocketEvents struct {
	cancel  context.CancelFunc
	changes chan struct{}
	ready   chan struct{}
	done    chan struct{}
	err     error // published by closing done
	once    sync.Once
}

func startEventHelper(ctx context.Context, name, path string, args ...string) (SocketEvents, error) {
	// Attachment must finish before the initial snapshot to avoid a startup gap.
	childCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(childCtx, path, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	// Keep diagnostics bounded even if the helper is noisy.
	diagnostics := &limitedDiagnostics{}
	cmd.Stderr = diagnostics
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	source := &helperSocketEvents{cancel: cancel, changes: make(chan struct{}, 1), ready: make(chan struct{}), done: make(chan struct{})}
	go func() {
		scanner := bufio.NewScanner(stdout)
		ready := false
		for scanner.Scan() {
			switch scanner.Text() {
			case "READY":
				if !ready {
					close(source.ready)
					ready = true
				}
			case "CHANGE":
				select {
				case source.changes <- struct{}{}:
				default:
				}
			}
		}
		scanErr := scanner.Err()
		if scanErr != nil {
			cancel()
		}
		waitErr := cmd.Wait()
		source.err = fmt.Errorf("%s socket event helper stopped (read: %v, exit: %v): %s", name, scanErr, waitErr, diagnostics.text)
		close(source.done)
	}()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case <-source.ready:
		return source, nil
	case <-source.done:
		cancel()
		return nil, source.err
	case <-ctx.Done():
		source.Close()
		return nil, ctx.Err()
	case <-timer.C:
		source.Close()
		return nil, fmt.Errorf("%s did not attach within 10s; check probe availability and tracing privileges", name)
	}
}

func (s *helperSocketEvents) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return s.err
	case <-s.changes:
		return nil
	}
}

func (s *helperSocketEvents) Close() error {
	s.once.Do(s.cancel)
	<-s.done
	return nil
}

type limitedDiagnostics struct{ text string }

func (d *limitedDiagnostics) Write(p []byte) (int, error) {
	n := len(p)
	if remaining := 4096 - len(d.text); remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		d.text += string(p)
	}
	return n, nil
}
