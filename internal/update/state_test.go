package update

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newService(t *testing.T) (*Service, *time.Time, *atomic.Int32) {
	t.Helper()
	now := time.Now().UTC()
	calls := &atomic.Int32{}
	checker := testChecker(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); fmt.Fprint(w, `{"tag_name":"v0.3.0"}`) })
	return &Service{Path: filepath.Join(t.TempDir(), "config", "updates.json"), Checker: checker, Now: func() time.Time { return now }}, &now, calls
}
func TestDisabledByDefaultCreatesNothing(t *testing.T) {
	service, _, calls := newService(t)
	message, nonce, err := service.PrepareAutomatic("v0.2.0")
	if err != nil || message != "" || nonce != "" || calls.Load() != 0 {
		t.Fatalf("%q %q %v", message, nonce, err)
	}
	if _, err := os.Stat(filepath.Dir(service.Path)); !os.IsNotExist(err) {
		t.Fatal("opt-out created settings directory")
	}
}
func TestDailyBackgroundAndNoticeThrottling(t *testing.T) {
	service, now, calls := newService(t)
	if err := service.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	message, nonce, err := service.PrepareAutomatic("v0.2.0")
	if err != nil || message != "" || nonce == "" || calls.Load() != 0 {
		t.Fatalf("foreground fetched or invalid reservation: %q %q %v", message, nonce, err)
	}
	if err := service.RunBackground(context.Background(), "v0.2.0", nonce); err != nil {
		t.Fatal(err)
	}
	if err := service.RunBackground(context.Background(), "v0.2.0", nonce); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("nonce caused duplicate checks")
	}
	message, nonce, err = service.PrepareAutomatic("v0.2.0")
	if err != nil || message == "" || nonce != "" {
		t.Fatalf("missing cached notice: %q %q %v", message, nonce, err)
	}
	message, nonce, err = service.PrepareAutomatic("v0.2.0")
	if err != nil || message != "" || nonce != "" {
		t.Fatal("repeated notice within the day")
	}
	if err := service.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	_, nonce, err = service.PrepareAutomatic("v0.2.0")
	if err != nil || nonce != "" {
		t.Fatal("idempotent opt-in reset the throttle")
	}
	*now = now.Add(DailyInterval)
	message, nonce, err = service.PrepareAutomatic("v0.2.0")
	if err != nil || message == "" || nonce == "" {
		t.Fatal("next-day check/notice missing")
	}
}
func TestOnlyOneConcurrentReservation(t *testing.T) {
	service, _, _ := newService(t)
	if err := service.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	var count atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, nonce, err := service.PrepareAutomatic("v0.2.0")
			if err != nil && !errors.Is(err, ErrBusy) {
				t.Error(err)
			}
			if nonce != "" {
				count.Add(1)
			}
		}()
	}
	wg.Wait()
	if count.Load() != 1 {
		t.Fatalf("reserved %d requests", count.Load())
	}
}
func TestDisableAndFailuresAreQuiet(t *testing.T) {
	service, _, calls := newService(t)
	service.SetEnabled(true)
	_, nonce, _ := service.PrepareAutomatic("v0.2.0")
	if err := service.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	if err := service.RunBackground(context.Background(), "v0.2.0", nonce); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("opt-out still fetched")
	}
	service.SetEnabled(true)
	service.Checker = testChecker(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) })
	_, nonce, _ = service.PrepareAutomatic("v0.2.0")
	if err := service.RunBackground(context.Background(), "v0.2.0", nonce); err == nil {
		t.Fatal("expected rate limit")
	}
	message, next, err := service.PrepareAutomatic("v0.2.0")
	if err != nil || message != "" || next != "" {
		t.Fatal("failure triggered repeat attempt or notice")
	}
}
func TestOptOutDuringFetchIsPreserved(t *testing.T) {
	service, _, _ := newService(t)
	service.SetEnabled(true)
	entered, release := make(chan struct{}), make(chan struct{})
	service.Checker = testChecker(t, func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		fmt.Fprint(w, `{"tag_name":"v0.3.0"}`)
	})
	_, nonce, _ := service.PrepareAutomatic("v0.2.0")
	done := make(chan error, 1)
	go func() { done <- service.RunBackground(context.Background(), "v0.2.0", nonce) }()
	<-entered
	err := service.SetEnabled(false)
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	state, err := service.Read()
	if err != nil || state.Enabled || state.Latest != "" {
		t.Fatalf("opt-out overwritten: %+v %v", state, err)
	}
}
func TestCacheCorruptionAndLaunchFailure(t *testing.T) {
	service, _, _ := newService(t)
	service.SetEnabled(true)
	_, nonce, _ := service.PrepareAutomatic("v0.2.0")
	service.CancelPending(nonce)
	_, next, err := service.PrepareAutomatic("v0.2.0")
	if err != nil || next == "" {
		t.Fatal("could not retry after worker launch failure")
	}
	os.WriteFile(service.Path, []byte("invalid"), 0600)
	message, nonce, err := service.PrepareAutomatic("v0.2.0")
	if err == nil || message != "" || nonce != "" {
		t.Fatal("corrupt config produced a notice or request")
	}
}
func TestNoNoticeWhenUpToDateOrNewer(t *testing.T) {
	for _, version := range []string{"v0.3.0", "v0.4.0"} {
		service, _, _ := newService(t)
		service.SetEnabled(true)
		_, nonce, _ := service.PrepareAutomatic(version)
		if err := service.RunBackground(context.Background(), version, nonce); err != nil {
			t.Fatal(err)
		}
		message, _, err := service.PrepareAutomatic(version)
		if err != nil || message != "" {
			t.Fatal("notified an up-to-date/newer install")
		}
	}
}

func TestTrailingSettingsDataIsRejected(t *testing.T) {
	service, _, calls := newService(t)
	service.SetEnabled(true)
	if err := os.WriteFile(service.Path, []byte(`{"enabled":true} {"enabled":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	message, nonce, err := service.PrepareAutomatic("v0.2.0")
	if err == nil || message != "" || nonce != "" || calls.Load() != 0 {
		t.Fatal("accepted corrupt trailing settings")
	}
}
