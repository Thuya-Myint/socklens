package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Thuya-Myint/socklens/internal/update"
)

func TestStoreNoticeDisabled(t *testing.T) {
	if !update.StoreManaged() {
		t.Skip("run with Store distribution linker flag")
	}
	if shouldAutoNotify([]string{"list"}, true) {
		t.Fatal("Store build enabled GitHub update notices")
	}
}

func TestStoreBuildUpdateOwnership(t *testing.T) {
	dir := t.TempDir()
	name := "socklens-store"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(dir, name)
	build := exec.Command("go", "build", "-ldflags", "-X github.com/Thuya-Myint/socklens/internal/update.distribution=microsoft-store -X main.version=v0.12.1", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	for _, args := range [][]string{{"update"}, {"update", "--dry-run"}, {"update-check", "--json"}, {"updates", "--enable"}, {"__install-update", filepath.Join(dir, "fake-plan")}, {"__update-refresh", "fake-nonce"}} {
		command := exec.Command(binary, args...)
		command.Env = append(os.Environ(), "XDG_CONFIG_HOME="+filepath.Join(dir, "config"), "APPDATA="+filepath.Join(dir, "config"))
		output, err := command.CombinedOutput()
		if err == nil || !strings.Contains(string(output), "Microsoft Store manages updates") {
			t.Fatalf("%v: %v %s", args, err, output)
		}
	}
	for _, args := range [][]string{{"version"}, {"list", "--help"}, {"completion", "bash"}, {"update", "--help"}, {"help"}} {
		output, err := exec.Command(binary, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v %s", args, err, output)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "config", "socklens")); !os.IsNotExist(err) {
		t.Fatalf("Store commands created update state: %v", err)
	}
	test := exec.Command("go", "test", "-ldflags", "-X github.com/Thuya-Myint/socklens/internal/update.distribution=microsoft-store", "-run", "^TestStoreNoticeDisabled$", ".")
	if output, err := test.CombinedOutput(); err != nil {
		t.Fatalf("Store notice check: %v\n%s", err, output)
	}
}
