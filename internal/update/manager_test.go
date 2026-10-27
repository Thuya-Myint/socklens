package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func managerChecker(t *testing.T) Checker {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"tag_name":"v0.5.0"}`) }))
	t.Cleanup(server.Close)
	return Checker{Endpoint: server.URL, Client: server.Client()}
}
func writeFixture(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0755); err != nil {
		t.Fatal(err)
	}
}
func TestHomebrewDelegationAndPostUpgradeVerification(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "Cellar", "socklens", "0.3.0", "bin", "socklens")
	writeFixture(t, target, "managed executable")
	writeFixture(t, filepath.Join(filepath.Dir(filepath.Dir(target)), "INSTALL_RECEIPT.json"), `{"source":{"tap":"thuya-myint/socklens"}}`)
	prefix := filepath.Join(root, "opt", "socklens")
	runner := &installRunner{output: func(c Command) ([]byte, error) {
		if len(c.Args) > 0 && c.Args[0] == "--prefix" {
			return []byte(prefix + "\n"), nil
		}
		return []byte("socklens v0.5.0\n"), nil
	}}
	updater := Updater{Checker: managerChecker(t), Executable: target, Runner: runner, Lookup: func(name string) (string, error) { return "brew", nil }, foreground: true}
	plan, err := updater.Check(context.Background(), "v0.3.0")
	if err != nil || plan.Method != "homebrew" {
		t.Fatal(plan, err)
	}
	result, err := updater.Apply(context.Background(), plan, Streams{Out: io.Discard, Err: io.Discard})
	if err != nil || result.Version != "v0.5.0" {
		t.Fatal(result, err)
	}
	if len(runner.commands) != 2 || strings.Join(runner.commands[0].Args, " ") != "update" || strings.Join(runner.commands[1].Args, " ") != "upgrade --formula thuya-myint/socklens/socklens" {
		t.Fatal("wrong Homebrew operations", runner.commands)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "managed executable" {
		t.Fatal("updater overwrote a Homebrew-owned file")
	}
	runner.output = func(c Command) ([]byte, error) {
		if c.Args[0] == "--prefix" {
			return []byte(prefix), nil
		}
		return []byte("socklens v0.4.0"), nil
	}
	if _, err := updater.Apply(context.Background(), plan, Streams{Out: io.Discard, Err: io.Discard}); err == nil {
		t.Fatal("stale formula was reported as updated")
	}
}
func TestScoopDelegationGuardsCatalogAndInstalledVersions(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SCOOP", root)
	target := filepath.Join(root, "apps", "socklens", "0.3.0", "socklens.exe")
	writeFixture(t, target, "managed executable")
	writeFixture(t, filepath.Join(filepath.Dir(target), "scoop-install.json"), `{"bucket":"socklens","architecture":"64bit"}`)
	writeFixture(t, filepath.Join(root, "apps", "scoop", "current", "bin", "scoop.ps1"), "# fixture")
	manifest := filepath.Join(root, "buckets", "socklens", "bucket", "socklens.json")
	writeFixture(t, manifest, `{"version":"0.5.0"}`)
	runner := &installRunner{}
	runner.output = func(c Command) ([]byte, error) {
		if len(runner.commands) >= 2 {
			return []byte("socklens v0.5.0"), nil
		}
		return []byte("socklens v0.3.0"), nil
	}
	updater := Updater{OS: "windows", Executable: target, Checker: managerChecker(t), Runner: runner, Lookup: func(string) (string, error) { return "pwsh", nil }, foreground: true}
	plan, err := updater.Check(context.Background(), "v0.3.0")
	if err != nil || plan.Method != "scoop" || plan.Global {
		t.Fatal(plan, err)
	}
	result, err := updater.Apply(context.Background(), plan, Streams{Out: io.Discard, Err: io.Discard})
	if err != nil || result.Version != "v0.5.0" {
		t.Fatal(result, err)
	}
	if len(runner.commands) != 2 || runner.commands[1].Args[len(runner.commands[1].Args)-1] != "socklens" {
		t.Fatal("wrong Scoop operations")
	}
	for _, c := range runner.commands {
		if strings.Contains(strings.Join(c.Args, " "), "Bypass") || strings.Contains(strings.Join(c.Args, " "), "--force") {
			t.Fatal("unsafe package-manager override")
		}
	}
	writeFixture(t, manifest, `{"version":"0.4.0"}`)
	runner.commands = nil
	if _, err := updater.Apply(context.Background(), plan, Streams{Out: io.Discard, Err: io.Discard}); err == nil || len(runner.commands) != 1 {
		t.Fatal("unpublished Scoop catalog was applied", err)
	}
	runner.commands = nil
	runner.output = func(Command) ([]byte, error) { return []byte("socklens v0.6.0"), nil }
	result, err = updater.Apply(context.Background(), plan, Streams{Out: io.Discard, Err: io.Discard})
	if err != nil || result.Version != "v0.6.0" || len(runner.commands) != 1 {
		t.Fatal("newer installed Scoop version was not preserved", result, err)
	}
	writeFixture(t, filepath.Join(filepath.Dir(target), "scoop-install.json"), `{"url":"https://example.invalid/pinned.json"}`)
	if _, err := updater.Detect(); err == nil {
		t.Fatal("pinned Scoop installation was treated as standalone")
	}
}
func TestInstallerDetectionRejectsUntrustedReceiptAndMissingManager(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "Cellar", "socklens", "0.3.0", "bin", "socklens")
	writeFixture(t, target, "executable")
	receipt := filepath.Join(filepath.Dir(filepath.Dir(target)), "INSTALL_RECEIPT.json")
	writeFixture(t, receipt, `{"source":{"tap":"--bad/owner"}}`)
	updater := Updater{Executable: target, Lookup: func(string) (string, error) { return "", fmt.Errorf("not found") }}
	if _, err := updater.Detect(); err == nil {
		t.Fatal("invalid tap accepted")
	}
	writeFixture(t, receipt, `{"source":{"tap":"thuya-myint/socklens"}}`)
	if _, err := updater.Detect(); err == nil {
		t.Fatal("managed install without brew fell back to self replacement")
	}
}

func TestInstalledHomebrewDetection(t *testing.T) {
	path := os.Getenv("SOCKLENS_TEST_HOMEBREW_EXECUTABLE")
	if path == "" {
		t.Skip("set by native Homebrew installation CI")
	}
	updater := DefaultUpdater()
	updater.Executable = path
	plan, err := updater.Detect()
	if err != nil || plan.Method != "homebrew" || !strings.HasSuffix(plan.Package, "/socklens") || plan.Program == "" {
		t.Fatalf("installed Homebrew receipt not recognized: %+v %v", plan, err)
	}
}
