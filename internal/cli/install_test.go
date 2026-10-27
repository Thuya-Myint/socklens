package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Thuya-Myint/socklens/internal/update"
)

type updateCLIRunner struct{ runs int }

func (r *updateCLIRunner) Run(context.Context, update.Command, update.Streams) error {
	r.runs++
	return nil
}
func (r *updateCLIRunner) Output(context.Context, update.Command) ([]byte, error) {
	return []byte("socklens v0.3.0"), nil
}

func TestUpdateCLIHelpArgumentsAndDryRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"tag_name":"v0.5.0"}`) }))
	defer server.Close()
	root := t.TempDir()
	target := filepath.Join(root, "Cellar", "socklens", "0.3.0", "bin", "socklens")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("managed"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(target)), "INSTALL_RECEIPT.json"), []byte(`{"source":{"tap":"thuya-myint/socklens"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	runner := &updateCLIRunner{}
	updater := update.Updater{OS: "linux", Checker: update.Checker{Endpoint: server.URL, Client: server.Client()}, Executable: target, Runner: runner, Lookup: func(string) (string, error) { return "brew", nil }}
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"--help"}, 0, "dry-run"},
		{[]string{"--dry-run"}, 0, "brew upgrade --formula thuya-myint/socklens/socklens"},
		{[]string{"--timeout", "0s"}, 1, "timeout must"},
		{[]string{"--timeout", "31m"}, 1, "timeout must"},
		{[]string{"--status", "--dry-run"}, 1, "cannot be combined"},
		{[]string{"extra"}, 1, "usage:"},
	} {
		var out, stderr bytes.Buffer
		code := updateCommand(context.Background(), tc.args, strings.NewReader(""), &out, &stderr, "v0.3.0", updater)
		if code != tc.code || !strings.Contains(out.String()+stderr.String(), tc.want) {
			t.Fatalf("%v: %d %s%s", tc.args, code, &out, &stderr)
		}
	}
	if runner.runs != 0 {
		t.Fatal("dry run/help/invalid arguments started installation")
	}
	var out, stderr bytes.Buffer
	if code := updateCommand(context.Background(), nil, strings.NewReader(""), &out, &stderr, "v0.5.0", updater); code != 0 || !strings.Contains(out.String(), "up to date") || runner.runs != 0 {
		t.Fatal("current version attempted to update", code, &out, &stderr)
	}
}
func TestUpdateCLICancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, stderr bytes.Buffer
	if code := updateCommand(ctx, nil, strings.NewReader(""), &out, &stderr, "v0.3.0", update.DefaultUpdater()); code != 130 {
		t.Fatalf("cancelled update returned %d: %s", code, &stderr)
	}
}
