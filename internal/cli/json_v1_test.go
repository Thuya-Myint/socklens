package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

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
	for _, tc := range []struct{ args []string; want bool }{
		{[]string{"list", "--format", "json-v1"}, true},
		{[]string{"inspect", "--format=json-v1", "8080"}, true},
		{[]string{"list", "--format", "table"}, false},
		{[]string{"list", "--json"}, false},
	} {
		if got := jsonV1Requested(tc.args); got != tc.want {
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
