package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSemanticVersionComparison(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"v0.2.0", "v0.2.1", -1}, {"v0.9.0", "v0.10.0", -1}, {"1.2.3", "v1.2.3", 0},
		{"v1.2.3+abc", "v1.2.3+def", 0}, {"v1.2.3-rc.1", "v1.2.3", -1},
		{"v1.2.3-alpha.9", "v1.2.3-alpha.10", -1}, {"v1.2.3-1", "v1.2.3-beta", -1},
		{"v1.2.3-alpha", "v1.2.3-alpha.1", -1}, {"v2.0.0", "v1.99.99", 1},
		{"v9999999999999999999999999.0.0", "v2.0.0", 1},
	} {
		got, err := Compare(tc.a, tc.b)
		if err != nil || (got < 0) != (tc.want < 0) || (got > 0) != (tc.want > 0) {
			t.Fatalf("%s vs %s: %d %v", tc.a, tc.b, got, err)
		}
	}
	for _, value := range []string{"dev", "", "v1.2", "v01.2.3", "v1.2.3-01", "v1.2.3\nBAD", "../v1.2.3"} {
		if _, err := Compare(value, "v1.2.3"); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}
func testChecker(t *testing.T, handler http.HandlerFunc) Checker {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return Checker{Endpoint: server.URL, Client: server.Client()}
}
func TestCheckResultsAndRequestPrivacy(t *testing.T) {
	checker := testChecker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.Header.Get("User-Agent") != "socklens/v0.2.0" || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected request headers/method")
		}
		fmt.Fprint(w, `{"tag_name":"v0.3.0","html_url":"https://untrusted.invalid","draft":false,"prerelease":false}`)
	})
	result, err := checker.Check(context.Background(), "v0.2.0")
	if err != nil || !result.UpdateAvailable || result.Status != "update-available" || result.ReleaseURL != ReleasesURL+"/tag/v0.3.0" {
		t.Fatalf("%+v %v", result, err)
	}
	for _, tc := range []struct{ version, status string }{{"v0.3.0", "up-to-date"}, {"v0.4.0", "ahead"}} {
		checker := testChecker(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"tag_name":"v0.3.0"}`) })
		result, err := checker.Check(context.Background(), tc.version)
		if err != nil || result.Status != tc.status || result.UpdateAvailable {
			t.Fatalf("%+v %v", result, err)
		}
	}
}
func TestCheckRejectsBadResponses(t *testing.T) {
	for _, body := range []string{`bad`, `{}`, `{"tag_name":"v1.0.0","draft":true}`, `{"tag_name":"v1.0.0","prerelease":true}`, `{"tag_name":"v1.0.0-rc.1"}`, `{"tag_name":"v1.0.0\nINJECT"}`, strings.Repeat("x", (1<<20)+1)} {
		checker := testChecker(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
		if _, err := checker.Check(context.Background(), "v0.2.0"); err == nil {
			t.Fatal("accepted invalid response")
		}
	}
	for _, status := range []int{403, 429, 404, 500} {
		checker := testChecker(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) })
		if _, err := checker.Check(context.Background(), "v0.2.0"); err == nil {
			t.Fatalf("accepted HTTP %d", status)
		}
	}
}
func TestCheckTimeoutAndNoRequestForDev(t *testing.T) {
	var calls atomic.Int32
	checker := testChecker(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); <-r.Context().Done() })
	if _, err := checker.Check(context.Background(), "dev"); err == nil || calls.Load() != 0 {
		t.Fatal("development build made a request")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := checker.Check(ctx, "v0.2.0"); err == nil {
		t.Fatal("timeout ignored")
	}
}
func TestCheckRefusesRedirects(t *testing.T) {
	var reached atomic.Bool
	checker := testChecker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/other" {
			reached.Store(true)
			fmt.Fprint(w, `{"tag_name":"v2.0.0"}`)
			return
		}
		http.Redirect(w, r, "/other", http.StatusFound)
	})
	if _, err := checker.Check(context.Background(), "v0.2.0"); err == nil || reached.Load() {
		t.Fatal("followed an unexpected redirect")
	}
}
