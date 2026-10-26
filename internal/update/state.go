package update

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const DailyInterval = 24 * time.Hour

var ErrBusy = errors.New("update settings are busy")

type State struct {
	Enabled      bool      `json:"enabled"`
	LastAttempt  time.Time `json:"last_attempt,omitempty"`
	Latest       string    `json:"latest,omitempty"`
	LastNotified string    `json:"last_notified,omitempty"`
	NotifiedAt   time.Time `json:"notified_at,omitempty"`
	Pending      string    `json:"pending,omitempty"`
}
type Service struct {
	Path    string
	Checker Checker
	Now     func() time.Time
}

func DefaultService() (*Service, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &Service{Path: filepath.Join(directory, "socklens", "updates.json"), Checker: DefaultChecker()}, nil
}
func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func (s *Service) Read() (State, error) {
	file, err := os.Open(s.Path)
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	defer file.Close()
	var state State
	info, err := file.Stat()
	if err != nil {
		return State{}, err
	}
	if info.Size() > 64<<10 {
		return State{}, fmt.Errorf("update settings are too large")
	}
	decoder := json.NewDecoder(io.LimitReader(file, (64<<10)+1))
	if err := decoder.Decode(&state); err != nil {
		return State{}, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return State{}, fmt.Errorf("invalid trailing update settings data")
	}
	return state, nil
}
func (s *Service) lock() (func(), error) {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return nil, err
	}
	path := s.Path + ".lock"
	if err := os.Mkdir(path, 0700); err != nil {
		if !os.IsExist(err) {
			return nil, err
		}
		if info, e := os.Stat(path); e == nil && time.Since(info.ModTime()) > time.Minute {
			if e := os.Remove(path); e == nil {
				if e := os.Mkdir(path, 0700); e == nil {
					return func() { os.Remove(path) }, nil
				}
			}
		}
		return nil, ErrBusy
	}
	return func() { os.Remove(path) }, nil
}
func (s *Service) save(state State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(s.Path), ".updates-")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(name, s.Path)
}
func (s *Service) SetEnabled(enabled bool) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	state, err := s.Read()
	if err != nil {
		return err
	}
	previous := state.Enabled
	state.Enabled = enabled
	if !enabled {
		state.Pending = ""
	}
	if enabled && !previous {
		state.Pending = ""
		state.LastAttempt = time.Time{}
	}
	return s.save(state)
}
func due(now, previous time.Time) bool {
	return previous.IsZero() || now.Sub(previous) >= DailyInterval || previous.After(now.Add(5*time.Minute))
}

// PrepareAutomatic does local I/O only. It reserves one background attempt and
// returns at most one cached notice per version/day. No network blocks the CLI.
func (s *Service) PrepareAutomatic(current string) (message, nonce string, err error) {
	if _, err := parseVersion(current); err != nil {
		return "", "", nil
	}
	initial, err := s.Read()
	if err != nil || !initial.Enabled {
		return "", "", err
	}
	unlock, err := s.lock()
	if err != nil {
		return "", "", err
	}
	defer unlock()
	state, err := s.Read()
	if err != nil || !state.Enabled {
		return "", "", err
	}
	now := s.now()
	if comparison, e := Compare(current, state.Latest); e == nil && comparison < 0 && (state.LastNotified != state.Latest || due(now, state.NotifiedAt)) {
		message = Notice(Result{Current: current, Latest: state.Latest, UpdateAvailable: true, ReleaseURL: ReleasesURL + "/tag/" + state.Latest})
		state.LastNotified = state.Latest
		state.NotifiedAt = now
	}
	if due(now, state.LastAttempt) {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", "", err
		}
		nonce = hex.EncodeToString(random[:])
		state.Pending = nonce
		state.LastAttempt = now
	}
	if message == "" && nonce == "" {
		return "", "", nil
	}
	if err := s.save(state); err != nil {
		return "", "", err
	}
	return message, nonce, nil
}

// RunBackground claims a reserved attempt once, fetches outside the file lock,
// and rechecks opt-in before updating the cache.
func (s *Service) RunBackground(ctx context.Context, current, nonce string) error {
	if !validNonce(nonce) {
		return nil
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	state, err := s.Read()
	if err != nil || !state.Enabled || state.Pending != nonce {
		unlock()
		return err
	}
	state.Pending = ""
	err = s.save(state)
	unlock()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result, err := s.Checker.Check(ctx, current)
	if err != nil {
		return err
	}
	unlock, err = s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	state, err = s.Read()
	if err != nil || !state.Enabled {
		return err
	}
	state.Latest = result.Latest
	return s.save(state)
}
func (s *Service) CancelPending(nonce string) {
	unlock, err := s.lock()
	if err != nil {
		return
	}
	defer unlock()
	state, err := s.Read()
	if err != nil || state.Pending != nonce {
		return
	}
	state.Pending = ""
	state.LastAttempt = time.Time{}
	_ = s.save(state)
}
