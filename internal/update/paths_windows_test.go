//go:build windows

package update

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func makeInstallJunction(t *testing.T, link, target string) {
	t.Helper()
	command := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "$ErrorActionPreference = 'Stop'; New-Item -ItemType Junction -Path $env:SOCKLENS_TEST_JUNCTION_LINK -Target $env:SOCKLENS_TEST_JUNCTION_TARGET | Out-Null")
	command.Env = append(os.Environ(), "SOCKLENS_TEST_JUNCTION_LINK="+link, "SOCKLENS_TEST_JUNCTION_TARGET="+target)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create fixture junction: %v: %s", err, output)
	}
}

func TestScoopJunctionDetectionAndDelegation(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "Scoop fixture ü")
	target := filepath.Join(root, "apps", "socklens", "0.3.0", "socklens.exe")
	writeFixture(t, target, "managed executable")
	writeFixture(t, filepath.Join(filepath.Dir(target), "scoop-install.json"), `{"bucket":"socklens","architecture":"64bit"}`)
	writeFixture(t, filepath.Join(root, "apps", "scoop", "current", "bin", "scoop.ps1"), "# fixture manager")
	writeFixture(t, filepath.Join(root, "buckets", "socklens", "bucket", "socklens.json"), `{"version":"0.5.0"}`)
	current := filepath.Join(root, "apps", "socklens", "current")
	makeInstallJunction(t, current, filepath.Dir(target))
	alias := filepath.Join(parent, "custom Scoop root")
	makeInstallJunction(t, alias, root)
	t.Setenv("SCOOP", alias)
	t.Setenv("LOCALAPPDATA", filepath.Join(parent, "private cache"))
	runner := &installRunner{}
	runner.output = func(Command) ([]byte, error) {
		if len(runner.commands) >= 2 {
			return []byte("socklens v0.5.0"), nil
		}
		return []byte("socklens v0.3.0"), nil
	}
	updater := Updater{Executable: filepath.Join(alias, "apps", "socklens", "current", "socklens.exe"), Checker: managerChecker(t), Runner: runner, Lookup: func(string) (string, error) { return "powershell.exe", nil }, foreground: true}
	plan, err := updater.Check(context.Background(), "v0.3.0")
	if err != nil {
		t.Fatal("resolve real Scoop junctions:", err)
	}
	if plan.Method != "scoop" || plan.Global {
		t.Fatalf("junction install misidentified: %+v", plan)
	}
	actual, err := os.Stat(plan.Target)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(target)
	if err != nil || !os.SameFile(actual, original) {
		t.Fatal("resolved executable identity differs", err)
	}
	outcome, err := updater.Apply(context.Background(), plan, Streams{Out: io.Discard, Err: io.Discard})
	if err != nil || outcome.Version != "v0.5.0" || len(runner.commands) != 2 {
		t.Fatalf("junction upgrade: %+v %v", outcome, err)
	}
	job, err := updater.InstallStatus()
	if err != nil || job.Status != "not-started" {
		t.Fatalf("junction status lookup: %+v %v", job, err)
	}
}

func TestInstalledScoopJunctionDetection(t *testing.T) {
	path := os.Getenv("SOCKLENS_TEST_SCOOP_EXECUTABLE")
	if path == "" {
		t.Skip("optional read-only check against a real Scoop installation")
	}
	updater := DefaultUpdater()
	updater.Executable = path
	plan, err := updater.Detect()
	if err != nil || plan.Method != "scoop" || plan.Script == "" || plan.BucketPath == "" {
		t.Fatalf("actual Scoop install: %+v %v", plan, err)
	}
}
