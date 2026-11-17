package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

func TestJSONV1Schema(t *testing.T) {
	rows := []platform.Socket{{
		Protocol: "tcp", Local: "[::1]:8080", Remote: "127.0.0.1:50000",
		State: "ESTABLISHED", PIDs: []int{42},
		Processes: []platform.Process{{PID: 42, Name: "node"}},
	}}
	var out bytes.Buffer
	if err := writeSocketsV1(&out, "list", rows); err != nil {
		t.Fatal(err)
	}
	var got socketResponseJSONV1
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != "1.0" || got.Command != "list" || got.Count != 1 {
		t.Fatalf("unexpected envelope: %+v", got)
	}
	if got.Sockets[0].Local.Address != "::1" || got.Sockets[0].Local.Port != 8080 || got.Sockets[0].Processes[0].Name != "node" {
		t.Fatalf("unexpected socket: %+v", got.Sockets[0])
	}
}

func TestJSONV1EmptyAndErrors(t *testing.T) {
	var out bytes.Buffer
	if err := writeSocketsV1(&out, "inspect", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"sockets": []`) {
		t.Fatal("expected empty array:", out.String())
	}
	if err := writeSocketsV1(&out, "list", []platform.Socket{{Local: "invalid"}}); err == nil {
		t.Fatal("expected invalid endpoint error")
	}
	if err := writeSocketsV1(brokenWriter{}, "list", nil); err == nil {
		t.Fatal("expected output error")
	}
}

func TestJSONV1Flags(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{[]string{"list", "--format", "json-v1"}, true},
		{[]string{"inspect", "--format=json-v1", "8080"}, true},
		{[]string{"list", "--format", "table"}, false},
		{[]string{"list", "--json"}, false},
		{nil, false},
		{[]string{"list", "-format", "json-v1"}, true},
		{[]string{"inspect", "-format=json-v1", "8080"}, true},
		{[]string{"inspect", "8080", "--format=json-v1"}, false},
		{[]string{"list", "--", "--format=json-v1"}, false},
		{[]string{"list", "--process", "--format=json-v1"}, false},
		{[]string{"watch", "--format=json-v1"}, false},
		{[]string{"list", "--format=json-v1", "--format=table"}, false},
		{[]string{"list", "--format=table", "--format=json-v1"}, true},
	} {
		if got := JSONV1Requested(tc.args); got != tc.want {
			t.Fatalf("%v: got %v want %v", tc.args, got, tc.want)
		}
	}
	var out, errs bytes.Buffer
	if code := Run([]string{"list", "--format", "invalid"}, strings.NewReader(""), &out, &errs, "test"); code == 0 {
		t.Fatal("accepted invalid format")
	}
	out.Reset()
	errs.Reset()
	if code := Run([]string{"list", "--json", "--format", "json-v1"}, strings.NewReader(""), &out, &errs, "test"); code == 0 {
		t.Fatal("accepted conflicting format flags")
	}
}

func TestJSONV1EndpointBoundaries(t *testing.T) {
	for _, tc := range []struct {
		raw, address string
		port         int
	}{
		{"127.0.0.1:1", "127.0.0.1", 1},
		{"0.0.0.0:0", "0.0.0.0", 0},
		{"[::]:65535", "::", 65535},
		{"[fe80::1%12]:5353", "fe80::1%12", 5353},
		{"[::ffff:127.0.0.1]:80", "::ffff:127.0.0.1", 80},
	} {
		got, err := endpointV1(tc.raw)
		if err != nil || got.Address != tc.address || got.Port != tc.port {
			t.Fatalf("%s: %+v, %v", tc.raw, got, err)
		}
	}
	for _, raw := range []string{"invalid", "::1:80", "127.0.0.1:-1", "127.0.0.1:65536", "127.0.0.1:http"} {
		if _, err := endpointV1(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestJSONV1MissingMetadataAndEncoding(t *testing.T) {
	rows := []platform.Socket{
		{Protocol: "udp", Local: "[::]:5353", State: "BOUND"},
		{Protocol: "tcp", Local: "127.0.0.1:80", Remote: "0.0.0.0:0", Inode: "123", PIDs: []int{42, 43}, Processes: []platform.Process{{PID: 42, Name: "worker\n\x1b[31m\"雪"}}},
	}
	var out bytes.Buffer
	if err := writeSocketsV1(&out, "list", rows); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Fatal("raw terminal escape in JSON")
	}
	var got socketResponseJSONV1
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.GeneratedAt.IsZero() || got.GeneratedAt.Location() != time.UTC {
		t.Fatal("missing UTC observation time")
	}
	if got.Sockets[0].PIDs == nil || got.Sockets[0].Processes == nil || got.Sockets[0].Remote != (endpointJSONV1{}) {
		t.Fatalf("missing metadata: %+v", got.Sockets[0])
	}
	if got.Sockets[1].Processes[0].Name != rows[1].Processes[0].Name || !reflect.DeepEqual(got.Sockets[1].PIDs, rows[1].PIDs) {
		t.Fatal("process data changed")
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope) != 5 {
		t.Fatalf("unexpected envelope fields: %v", envelope)
	}
	for _, name := range []string{"schema_version", "command", "generated_at", "count", "sockets"} {
		if _, ok := envelope[name]; !ok {
			t.Fatalf("missing %s", name)
		}
	}
	var sockets []map[string]json.RawMessage
	if err := json.Unmarshal(envelope["sockets"], &sockets); err != nil {
		t.Fatal(err)
	}
	if _, ok := sockets[0]["inode"]; ok {
		t.Fatal("unavailable inode should be omitted")
	}
	if string(sockets[1]["inode"]) != `"123"` {
		t.Fatal("inode changed")
	}
}

func TestJSONV1CommandFilteringAndLegacyCompatibility(t *testing.T) {
	rows := []platform.Socket{
		{Protocol: "tcp", Local: "127.0.0.1:8080", Remote: "0.0.0.0:0", State: "LISTEN", PIDs: []int{42}, Processes: []platform.Process{{PID: 42, Name: "node"}}},
		{Protocol: "udp", Local: "[::]:5353", Remote: "[::]:0", State: "BOUND"},
	}
	for _, command := range []string{"list", "inspect"} {
		args := []string{"--format", "json-v1", "--protocol", "tcp", "--process", "NODE", "--pid", "42", "--listening"}
		if command == "inspect" {
			args = append(args, "8080")
		}
		var out, stderr bytes.Buffer
		code := socketsWithSource(command, args, &out, &stderr, func() ([]platform.Socket, error) { return rows, nil })
		var got socketResponseJSONV1
		if code != 0 || json.Unmarshal(out.Bytes(), &got) != nil || got.Command != command || got.Count != 1 || got.Sockets[0].Local.Port != 8080 {
			t.Fatalf("%s: code=%d %s %s", command, code, &out, &stderr)
		}
	}
	for _, args := range [][]string{{"--json"}, {"--format", "table", "--json"}} {
		var out, stderr bytes.Buffer
		if code := socketsWithSource("list", args, &out, &stderr, func() ([]platform.Socket, error) { return rows, nil }); code != 0 {
			t.Fatal(&stderr)
		}
		var got []platform.Socket
		if err := json.Unmarshal(out.Bytes(), &got); err != nil || !reflect.DeepEqual(got, rows) {
			t.Fatalf("legacy changed: %s %v", &out, err)
		}
		if !strings.Contains(out.String(), `"Local": "127.0.0.1:8080"`) || strings.Contains(out.String(), `"schema_version"`) {
			t.Fatalf("legacy field names changed: %s", &out)
		}
	}
	var out, stderr bytes.Buffer
	if code := socketsWithSource("list", []string{"--format=json-v1", "--process=not-present"}, &out, &stderr, func() ([]platform.Socket, error) { return rows, nil }); code != 0 {
		t.Fatal(&stderr)
	}
	var empty socketResponseJSONV1
	if err := json.Unmarshal(out.Bytes(), &empty); err != nil || empty.Count != 0 || empty.Sockets == nil || len(empty.Sockets) != 0 {
		t.Fatalf("empty response: %s %v", &out, err)
	}
}

func TestJSONV1FailuresDoNotEmitPartialResponse(t *testing.T) {
	for _, tc := range []struct {
		rows []platform.Socket
		err  error
	}{
		{err: errors.New("discovery failed")},
		{rows: []platform.Socket{{Local: "127.0.0.1:80", Remote: "invalid"}}},
	} {
		var out, stderr bytes.Buffer
		code := socketsWithSource("list", []string{"--format=json-v1"}, &out, &stderr, func() ([]platform.Socket, error) { return tc.rows, tc.err })
		if code != 1 || out.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, &out, &stderr)
		}
	}
	for _, command := range []string{"watch", "wait", "audit", "snapshot"} {
		var out, stderr bytes.Buffer
		if code := Run([]string{command, "--format=json-v1"}, strings.NewReader(""), &out, &stderr, "test"); code != 1 {
			t.Fatalf("%s accepted unsupported format", command)
		}
	}
}

func TestJSONV1Presentation(t *testing.T) {
	for _, format := range [][]string{{"--format", "json-v1"}, {"--format=json-v1"}, {"-format", "json-v1"}, {"-format=json-v1"}} {
		args := append([]string{"list"}, format...)
		args = append(args, "--color=always", "--protocol=invalid")
		var out, stderr bytes.Buffer
		if code := Run(args, strings.NewReader(""), &out, &stderr, "test"); code != 1 || strings.Contains(stderr.String(), "\x1b[") || !strings.Contains(stderr.String(), "protocol must") {
			t.Fatalf("%v: code=%d %q", args, code, &stderr)
		}
	}
}

func TestJSONV1InvalidOptionsSkipDiscovery(t *testing.T) {
	for _, tc := range []struct {
		command string
		args    []string
	}{
		{"list", []string{"--format="}},
		{"list", []string{"--format=invalid"}},
		{"list", []string{"--format=json-v1", "--json"}},
		{"inspect", []string{"8080", "--format=json-v1"}},
		{"inspect", []string{"--format=json-v1", "0"}},
	} {
		var out, stderr bytes.Buffer
		code := socketsWithSource(tc.command, tc.args, &out, &stderr, func() ([]platform.Socket, error) {
			t.Fatalf("invalid options reached discovery: %v", tc.args)
			return nil, nil
		})
		if code != 1 || out.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("%v: code=%d %q %q", tc.args, code, &out, &stderr)
		}
	}
}

func TestJSONV1LiveListener(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	for _, command := range []string{"list", "inspect"} {
		args := []string{command, "--format", "json-v1", "--color", "always", "--protocol", "tcp", "--listening"}
		if command == "list" {
			args = append(args, "--port", strconv.Itoa(port))
		} else {
			args = append(args, strconv.Itoa(port))
		}
		var out, stderr bytes.Buffer
		code := Run(args, strings.NewReader(""), &out, &stderr, "test")
		var got socketResponseJSONV1
		if code != 0 || json.Unmarshal(out.Bytes(), &got) != nil || got.Command != command || got.Count < 1 {
			t.Fatalf("%s: code=%d %s %s", command, code, &out, &stderr)
		}
		for _, socket := range got.Sockets {
			if socket.Local.Port != port || socket.Protocol != "tcp" || socket.State != "LISTEN" {
				t.Fatalf("unexpected socket: %+v", socket)
			}
		}
		if strings.Contains(out.String()+stderr.String(), "\x1b[") {
			t.Fatal("JSON command emitted terminal colors")
		}
	}
}
