package terminal

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRedirectedStreamsArePlainByDefault(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	var buffer bytes.Buffer
	file, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	pipe, end, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pipe.Close()
	defer end.Close()
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	for _, out := range []*os.File{file, end, null} {
		if IsTerminal(out) || Width(Wrap(out, "auto")) != 0 || Style(Wrap(out, "auto"), Error, "ERROR") != "ERROR" {
			t.Fatal("redirected output treated as a console")
		}
	}
	if Style(Wrap(&buffer, "auto"), Success, "OK") != "OK" {
		t.Fatal("buffer colored automatically")
	}
	t.Setenv("NO_COLOR", "1")
	if !strings.Contains(Style(Wrap(&buffer, "always"), Warning, "WARN"), "\x1b[") {
		t.Fatal("explicit always should override environment")
	}
	if Style(Wrap(&buffer, "never"), Warning, "WARN") != "WARN" {
		t.Fatal("never emitted colors")
	}
}
