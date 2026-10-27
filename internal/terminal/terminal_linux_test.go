//go:build linux

package terminal

import (
	"os"
	"strings"
	"testing"
)

func TestAutomaticColorAndEnvironmentOnTerminal(t *testing.T) {
	tty, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("pseudo-terminal unavailable: %v", err)
	}
	defer tty.Close()
	if !IsTerminal(tty) {
		t.Fatal("pseudo-terminal not recognized")
	}
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm")
	if !strings.Contains(Style(Wrap(tty, "auto"), Success, "OK"), "\x1b[") {
		t.Fatal("terminal did not get automatic colors")
	}
	t.Setenv("NO_COLOR", "1")
	if Style(Wrap(tty, "auto"), Success, "OK") != "OK" {
		t.Fatal("NO_COLOR ignored")
	}
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	if Style(Wrap(tty, "auto"), Success, "OK") != "OK" {
		t.Fatal("TERM=dumb ignored")
	}
}
