//go:build windows

package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsRollbackRestoresMissingTarget(t *testing.T) {
	root := t.TempDir()
	target, backup := filepath.Join(root, "socklens.exe"), filepath.Join(root, "previous.exe")
	if err := os.WriteFile(backup, []byte("previous executable"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := restoreExecutable(target, backup); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "previous executable" {
		t.Fatal("original executable was not recovered", err)
	}
}
