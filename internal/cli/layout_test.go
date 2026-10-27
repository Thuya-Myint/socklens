package cli

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/Thuya-Myint/socklens/internal/platform"
	"github.com/Thuya-Myint/socklens/internal/terminal"
)

func TestResponsiveSocketDetailsStayComplete(t *testing.T) {
	row := platform.Socket{Protocol: "tcp", Local: "[2001:db8:1234:5678:9abc:def0:1111:2222]:65535", Remote: "[2001:db8:aaaa:bbbb:cccc:dddd:eeee:ffff]:12345", State: "ESTABLISHED", PIDs: []int{12345, 67890}, Processes: []platform.Process{{PID: 12345, Name: strings.Repeat("worker", 20) + "\x1b[31m"}, {PID: 67890, Name: "second"}}}
	spaces := regexp.MustCompile(`\s+`)
	for _, width := range []int{1, 8, 20, 40, 60, 80, 120} {
		var plain, colored bytes.Buffer
		for _, target := range []struct {
			out   *bytes.Buffer
			color string
		}{{&plain, "never"}, {&colored, "always"}} {
			if err := writeSocketTableWidth(terminal.Wrap(target.out, target.color), []platform.Socket{row}, width); err != nil {
				t.Fatal(err)
			}
		}
		clean := ansi.ReplaceAllString(colored.String(), "")
		if clean != plain.String() {
			t.Fatalf("width %d: colors changed wrapping", width)
		}
		for _, line := range strings.Split(clean, "\n") {
			if len(line) > width {
				t.Fatalf("width %d: overflowing line %q", width, line)
			}
		}
		joined := spaces.ReplaceAllString(clean, "")
		for _, value := range []string{row.Local, row.Remote, "12345,67890", processName(row.Processes[0].Name), "67890:second"} {
			if !strings.Contains(joined, spaces.ReplaceAllString(value, "")) {
				t.Fatalf("width %d: lost detail %q", width, value)
			}
		}
	}
}

func TestTableLayoutOnlyChangesWhenNeeded(t *testing.T) {
	rows := []platform.Socket{fixtureSocket()}
	for _, width := range []int{0, 300} {
		var out bytes.Buffer
		if err := writeSocketTableWidth(&out, rows, width); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "PROCESSES") || strings.Contains(out.String(), "Local:") {
			t.Fatal("full table changed when it fits or output is redirected")
		}
	}
	var out bytes.Buffer
	if err := writeSocketTableWidth(&out, rows, 32); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "PROCESSES") || !strings.Contains(out.String(), "Local:") {
		t.Fatal("narrow terminal did not use stacked records")
	}
	for _, width := range []int{8, 20, 40} {
		out.Reset()
		if err := writeSocketTableWidth(&out, nil, width); err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(out.String(), "\n") {
			if len(line) > width {
				t.Fatal("empty result overflowed")
			}
		}
	}
}

func TestResponsiveLayoutPreservesOutputErrors(t *testing.T) {
	rows := []platform.Socket{fixtureSocket()}
	for _, remaining := range []int{0, 1, 2, 6} {
		if err := writeSocketTableWidth(&failAfterWriter{remaining: remaining}, rows, 20); err == nil {
			t.Fatal("responsive write failure lost")
		}
	}
}
