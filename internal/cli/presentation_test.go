package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
	"github.com/Thuya-Myint/socklens/internal/terminal"
)

func TestPresentationArgumentBoundaries(t *testing.T) {
	for _, tc := range []struct {
		args, want []string
		mode       string
		bad        bool
	}{
		{[]string{"help"}, []string{"help"}, "auto", false},
		{[]string{"list"}, []string{"list"}, "auto", false},
		{[]string{"--color", "always", "watch", "--changes"}, []string{"watch", "--changes"}, "always", false},
		{[]string{"inspect", "--color=never", "--protocol", "tcp", "8080"}, []string{"inspect", "--protocol", "tcp", "8080"}, "never", false},
		{[]string{"list", "--protocol", "--color=always"}, []string{"list", "--protocol", "--color=always"}, "auto", false},
		{[]string{"inspect", "8080", "--color=always"}, []string{"inspect", "8080", "--color=always"}, "auto", false},
		{[]string{"inspect", "--", "--color=always"}, []string{"inspect", "--", "--color=always"}, "auto", false},
		{[]string{"list", "--pid", "42", "--color", "always"}, []string{"list", "--pid", "42"}, "always", false},
		{[]string{"list", "--color"}, nil, "", true},
		{[]string{"list", "--color=rainbow"}, nil, "", true},
	} {
		got, mode, err := ParseColorArgs(tc.args)
		if (err != nil) != tc.bad || !reflect.DeepEqual(got, tc.want) || mode != tc.mode {
			t.Fatalf("%v: %v %s %v", tc.args, got, mode, err)
		}
	}
}

func TestPresentationCLIContracts(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		code  int
		color bool
		want  string
	}{
		{[]string{"--color=always", "help"}, 0, true, "Usage:"},
		{[]string{"help", "--color=never"}, 0, false, "Usage:"},
		{[]string{"list", "--color=always", "--help"}, 0, true, "--color"},
		{[]string{"list", "--color=always", "--bad-option"}, 1, true, "Try: socklens list --help"},
		{[]string{"list", "--color=always", "--json", "--protocol", "icmp"}, 1, false, "protocol must"},
		{[]string{"list", "--color=always", "--json", "--bad-option"}, 1, false, "flag provided"},
		{[]string{"version", "--color=always"}, 0, false, "socklens v1.2.3\n"},
		{[]string{"list", "--color=wrong"}, 1, false, "--color must"},
	} {
		var out, stderr bytes.Buffer
		code := RunContext(context.Background(), tc.args, strings.NewReader(""), &out, &stderr, "v1.2.3")
		text := out.String() + stderr.String()
		if code != tc.code || strings.Contains(text, "\x1b[") != tc.color || !strings.Contains(text, tc.want) {
			t.Fatalf("%v: %d %q", tc.args, code, text)
		}
	}
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestColoredTableKeepsColumnsAndEscapesProcessNames(t *testing.T) {
	rows := []platform.Socket{
		{Protocol: "tcp", Local: "127.0.0.1:80", Remote: "0.0.0.0:0", State: "LISTEN", PIDs: []int{42}, Processes: []platform.Process{{PID: 42, Name: "nginx"}}},
		{Protocol: "tcp", Local: "[::1]:65535", Remote: "[::1]:1234", State: "ESTABLISHED", PIDs: []int{50, 51}, Processes: []platform.Process{{PID: 50, Name: "a\x1b[31m\t\n"}, {PID: 51, Name: "worker"}}},
	}
	var plain, colored bytes.Buffer
	if err := writeSockets(&plain, rows, false); err != nil {
		t.Fatal(err)
	}
	if err := writeSockets(terminal.Wrap(&colored, "always"), rows, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(colored.String(), "\x1b[") || ansi.ReplaceAllString(colored.String(), "") != plain.String() {
		t.Fatal("color changed table content or alignment")
	}
	if strings.Contains(plain.String(), "\x1b") || !strings.Contains(plain.String(), `50:"a\x1b[31m\t\n"`) {
		t.Fatalf("unsafe process name: %q", plain.String())
	}
	if !strings.Contains(plain.String(), "2 sockets | 1 listening/bound | 1 connected") || !strings.Contains(plain.String(), "nginx") {
		t.Fatal("missing socket summary or process name")
	}
	lines := strings.Split(plain.String(), "\n")
	// Every row begins its STATE field in the same column, with and without ANSI.
	column := strings.Index(lines[0], "STATE")
	if strings.Index(lines[1], "LISTEN") != column || strings.Index(lines[2], "ESTABLISHED") != column {
		t.Fatal("STATE columns do not align")
	}
}

func TestPresentationJSONAndEmptyResults(t *testing.T) {
	rows := []platform.Socket{fixtureSocket()}
	var out bytes.Buffer
	styled := terminal.Wrap(&out, "always")
	if err := writeSockets(styled, rows, true); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") || !json.Valid(out.Bytes()) {
		t.Fatal("colors leaked into socket JSON")
	}
	out.Reset()
	if err := writeSockets(styled, []platform.Socket{}, true); err != nil || strings.TrimSpace(out.String()) != "[]" {
		t.Fatal("empty JSON changed")
	}
	out.Reset()
	if err := writeSockets(styled, nil, false); err != nil || !strings.Contains(out.String(), "No matching sockets.") {
		t.Fatal("missing empty-result message")
	}
	out.Reset()
	if err := watchSnapshots(context.Background(), styled, socketOptions{json: true, changes: true}, time.Millisecond, 1, func() ([]platform.Socket, error) { return rows, nil }); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") || !json.Valid(out.Bytes()) {
		t.Fatal("colors leaked into change JSON")
	}
	out.Reset()
	if err := writeWaitResult(styled, waitResult{Port: 8080, Condition: "present", Sockets: rows}, true); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") || !json.Valid(out.Bytes()) {
		t.Fatal("colors leaked into wait JSON")
	}
}
