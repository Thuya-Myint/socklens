package cli

import (
	"bytes"
	"errors"
	"github.com/Thuya-Myint/socklens/internal/platform"
	"strings"
	"testing"
)

func TestCommands(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{nil, 0, "Usage:"},
		{[]string{"help"}, 0, "inspect"},
		{[]string{"version"}, 0, "socklens test"},
		{[]string{"wat"}, 1, "unknown command"},
		{[]string{"list", "--pid", "0"}, 1, "PID must"},
		{[]string{"list", "--pid", "-1"}, 1, "PID must"},
		{[]string{"list", "--port", "0"}, 1, "port must"},
		{[]string{"list", "--port", "65536"}, 1, "port must"},
		{[]string{"inspect", "8080", "--listening"}, 1, "flags must precede"},
		{[]string{"inspect", "0"}, 1, "port must"},
		{[]string{"list", "--protocol", "icmp"}, 1, "protocol must"},
		{[]string{"terminate", "--pid", "1"}, 1, "PID must"},
		{[]string{"terminate", "--pid", "-5"}, 1, "PID must"},
	} {
		var out, stderr bytes.Buffer
		code := Run(tc.args, strings.NewReader(""), &out, &stderr, "test")
		if code != tc.code || !strings.Contains(out.String()+stderr.String(), tc.want) {
			t.Errorf("%v: code=%d output=%s%s", tc.args, code, &out, &stderr)
		}
	}
}

func TestSocketFilters(t *testing.T) {
	rows := []platform.Socket{
		{Protocol: "tcp", Local: "127.0.0.1:8080", State: "LISTEN", PIDs: []int{10, 20}},
		{Protocol: "tcp", Local: "[::1]:8080", State: "ESTABLISHED", PIDs: []int{20}},
		{Protocol: "udp", Local: "[::]:5353", State: "UNCONN", PIDs: []int{30}},
		{Protocol: "udp", Local: "127.0.0.1:5353", State: "CONNECTED", PIDs: []int{30}},
		{Protocol: "tcp", Local: "0.0.0.0:80", State: "LISTEN"},
	}
	for _, tc := range []struct {
		name   string
		filter socketFilter
		want   int
	}{
		{"all", socketFilter{}, 5},
		{"listeners", socketFilter{listening: true}, 3},
		{"tcp listeners", socketFilter{listening: true, protocol: "tcp"}, 2},
		{"udp bound", socketFilter{listening: true, protocol: "udp"}, 1},
		{"shared owner", socketFilter{pid: 20}, 2},
		{"combined", socketFilter{port: 8080, pid: 20, listening: true, protocol: "tcp"}, 1},
		{"unknown owner excluded", socketFilter{pid: 99}, 0},
		{"IPv6", socketFilter{port: 5353}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := filterSockets(rows, tc.filter)
			if err != nil || len(got) != tc.want {
				t.Fatalf("got %d rows, %v; want %d", len(got), err, tc.want)
			}
		})
	}
	if _, err := filterSockets([]platform.Socket{{Local: "invalid"}}, socketFilter{}); err == nil {
		t.Fatal("accepted invalid address")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write(p []byte) (int, error) { return 0, errors.New("output unavailable") }

func TestSocketOutput(t *testing.T) {
	var out bytes.Buffer
	rows := []platform.Socket{{Protocol: "tcp", Local: "127.0.0.1:80", PIDs: []int{42}, Processes: []platform.Process{{PID: 42, Name: "name\t\x1b\n"}}}}
	// Use actual control bytes to ensure terminal output cannot inject them.
	rows[0].Processes[0].Name = "name" + string([]byte{9, 27, 10})
	if err := writeSockets(&out, rows, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), string(byte(27))) || !strings.Contains(out.String(), `name\t\x1b\n`) || !strings.Contains(out.String(), "PROCESSES") {
		t.Fatal("process names were not rendered safely:", out.String())
	}
	if err := writeSockets(brokenWriter{}, rows, false); err == nil {
		t.Fatal("table write error lost")
	}
	if err := writeSockets(brokenWriter{}, rows, true); err == nil {
		t.Fatal("JSON write error lost")
	}
	out.Reset()
	if err := writeSockets(&out, []platform.Socket{}, true); err != nil || strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("empty JSON: %q, %v", out.String(), err)
	}
}

func TestMetadataCommandsRejectExtraArguments(t *testing.T) {
	for _, command := range []string{"help", "version"} {
		var out, stderr bytes.Buffer
		if code := Run([]string{command, "unexpected"}, strings.NewReader(""), &out, &stderr, "test"); code != 1 {
			t.Fatalf("%s ignored extra arguments", command)
		}
	}
}

func TestHelpAndVersionOutputFailures(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"version"}} {
		var stderr bytes.Buffer
		if code := Run(args, strings.NewReader(""), brokenWriter{}, &stderr, "test"); code != 1 {
			t.Fatalf("%v ignored output error", args)
		}
	}
}
