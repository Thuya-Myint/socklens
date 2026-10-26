package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Thuya-Myint/socklens/internal/update"
)

func TestUpdateCheckCLIJSONAndErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"tag_name":"v0.3.0"}`) }))
	defer server.Close()
	checker := update.Checker{Endpoint: server.URL, Client: server.Client()}
	var out, stderr bytes.Buffer
	code := updateCheckCommand(context.Background(), []string{"--json"}, &out, &stderr, "v0.2.0", checker)
	var result update.Result
	if code != 0 || stderr.Len() != 0 || json.Unmarshal(out.Bytes(), &result) != nil || !result.UpdateAvailable {
		t.Fatalf("%d %s %s", code, &out, &stderr)
	}
	for _, args := range [][]string{{"--timeout", "0s"}, {"--timeout", "31s"}, {"extra"}} {
		out.Reset()
		stderr.Reset()
		if code := updateCheckCommand(context.Background(), args, &out, &stderr, "v0.2.0", checker); code != 1 {
			t.Fatal("invalid arguments accepted", args)
		}
	}
	stderr.Reset()
	if code := updateCheckCommand(context.Background(), nil, brokenWriter{}, &stderr, "v0.2.0", checker); code != 1 {
		t.Fatal("output error lost")
	}
}
func TestUpdatePreferenceCLI(t *testing.T) {
	service := &update.Service{Path: filepath.Join(t.TempDir(), "updates.json")}
	for _, tc := range []struct {
		args    []string
		message string
		code    int
	}{
		{nil, "disabled", 0}, {[]string{"--enable"}, "enabled", 0}, {[]string{"--status"}, "enabled", 0},
		{[]string{"--disable"}, "disabled", 0}, {[]string{"--enable", "--disable"}, "usage:", 1},
	} {
		var out, stderr bytes.Buffer
		code := updatesCommand(tc.args, &out, &stderr, service)
		if code != tc.code || !strings.Contains(out.String()+stderr.String(), tc.message) {
			t.Fatalf("%v: %d %s%s", tc.args, code, &out, &stderr)
		}
	}
}
