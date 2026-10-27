//go:build windows

package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func protocolUpdater(target string) (Updater, func()) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"tag_name":"v0.5.0"}`) }))
	runner := &installRunner{}
	runner.run = func(c Command) error {
		file, err := os.OpenFile(os.Getenv("SOCKLENS_INSTALL_TEST_MARKER"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = fmt.Fprintln(file, strings.Join(c.Args, " "))
		return err
	}
	runner.output = func(c Command) ([]byte, error) {
		if len(c.Args) > 0 && c.Args[0] == "--prefix" {
			return []byte(filepath.Dir(filepath.Dir(target))), nil
		}
		return []byte("socklens v0.5.0"), nil
	}
	updater := Updater{Checker: Checker{Endpoint: server.URL, Client: server.Client()}, Executable: target, Runner: runner, Lookup: func(string) (string, error) { return "brew", nil }}
	return updater, server.Close
}
func TestMain(m *testing.M) {
	if os.Getenv("SOCKLENS_INSTALL_TEST_HELPER") == "1" && len(os.Args) == 3 {
		switch os.Args[1] {
		case "__test-launch-install":
			updater, closeServer := protocolUpdater(os.Args[2])
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			plan, err := updater.Check(ctx, "v0.3.0")
			if err == nil {
				_, err = updater.Apply(ctx, plan, Streams{Out: os.Stdout, Err: os.Stderr})
			}
			cancel()
			closeServer()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			os.Exit(0)
		case "__install-update":
			err := runInstallWorker(context.Background(), os.Args[2], func(target string) Updater { updater, _ := protocolUpdater(target); return updater })
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(3)
			}
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}
func TestWindowsInstallHandoffWaitsForParentAndReportsStatus(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "cache"))
	t.Setenv("SOCKLENS_INSTALL_TEST_HELPER", "1")
	marker := filepath.Join(root, "manager-commands.txt")
	t.Setenv("SOCKLENS_INSTALL_TEST_MARKER", marker)
	target := filepath.Join(root, "Cellar", "socklens", "0.3.0", "bin", "socklens")
	writeFixture(t, target, "managed executable")
	writeFixture(t, filepath.Join(filepath.Dir(filepath.Dir(target)), "INSTALL_RECEIPT.json"), `{"source":{"tap":"thuya-myint/socklens"}}`)
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	launcher := exec.Command(source, "__test-launch-install", target)
	if output, err := launcher.CombinedOutput(); err != nil {
		t.Fatalf("launcher failed: %v %s", err, output)
	}
	updater := Updater{Executable: target, Lookup: func(string) (string, error) { return "brew", nil }}
	deadline := time.Now().Add(20 * time.Second)
	var job InstallJob
	for time.Now().Before(deadline) {
		job, err = updater.InstallStatus()
		if err == nil && (job.Status == "completed" || job.Status == "failed") {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil || job.Status != "completed" || job.Latest != "v0.5.0" {
		t.Fatalf("worker status: %+v %v", job, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := waitForInstallerParent(ctx, job.PID); err != nil {
		t.Fatal("worker did not release its executable", err)
	}
	data, err := os.ReadFile(marker)
	if err != nil || !strings.Contains(string(data), "upgrade --formula thuya-myint/socklens/socklens") {
		t.Fatal("package manager was not run after handoff", string(data), err)
	}
	if _, err := loadInstallJob(filepath.Dir(filepath.Dir(job.LogPath)), "../escape"); err == nil {
		t.Fatal("worker accepted an invalid nonce")
	}
}
