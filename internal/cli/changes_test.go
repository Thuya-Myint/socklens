package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

func fixtureSocket() platform.Socket {
	return platform.Socket{Protocol: "tcp", Local: "127.0.0.1:8080", Remote: "0.0.0.0:0", Inode: "42", State: "LISTEN",
		PIDs: []int{10}, Processes: []platform.Process{{PID: 10, Name: "server"}}}
}

func TestDiffSocketChanges(t *testing.T) {
	original := fixtureSocket()
	for _, field := range []string{"state", "owner", "name"} {
		t.Run(field, func(t *testing.T) {
			changed := cloneSockets([]platform.Socket{original})[0]
			switch field {
			case "state":
				changed.State = "CLOSE"
			case "owner":
				changed.PIDs = []int{20}
				changed.Processes = []platform.Process{{PID: 20, Name: "server"}}
			case "name":
				changed.Processes[0].Name = "new-name"
			}
			got := diffSockets([]platform.Socket{original}, []platform.Socket{changed})
			if len(got.Updated) != 1 || len(got.Added) != 0 || len(got.Removed) != 0 {
				t.Fatalf("expected update only: %+v", got)
			}
			if !reflect.DeepEqual(got.Updated[0].Before, original) || !reflect.DeepEqual(got.Updated[0].After, changed) {
				t.Fatal("before/after details lost")
			}
		})
	}
	for _, field := range []string{"inode", "local", "remote", "protocol"} {
		t.Run(field, func(t *testing.T) {
			changed := original
			switch field {
			case "inode":
				changed.Inode = "43"
			case "local":
				changed.Local = "127.0.0.1:8081"
			case "remote":
				changed.Remote = "127.0.0.1:9000"
			case "protocol":
				changed.Protocol = "udp"
			}
			got := diffSockets([]platform.Socket{original}, []platform.Socket{changed})
			if len(got.Updated) != 0 || len(got.Added) != 1 || len(got.Removed) != 1 {
				t.Fatalf("expected replacement: %+v", got)
			}
		})
	}
}

func TestDiffIgnoresOrderingWithoutMutating(t *testing.T) {
	original := fixtureSocket()
	original.PIDs = []int{20, 10}
	original.Processes = []platform.Process{{PID: 20, Name: "b"}, {PID: 10, Name: "a"}}
	normalized := cloneSockets([]platform.Socket{original})[0]
	other := fixtureSocket()
	other.Inode = "99"
	got := diffSockets([]platform.Socket{original, other}, []platform.Socket{other, normalized})
	if got.hasChanges() {
		t.Fatalf("order caused false changes: %+v", got)
	}
	if original.PIDs[0] != 20 || original.Processes[0].PID != 20 {
		t.Fatal("source mutated")
	}
	empty := fixtureSocket()
	empty.PIDs = nil
	empty.Processes = nil
	normalizedEmpty := cloneSockets([]platform.Socket{empty})[0]
	if diffSockets([]platform.Socket{empty}, []platform.Socket{normalizedEmpty}).hasChanges() {
		t.Fatal("nil versus empty owners caused false change")
	}
}

func TestDiffDuplicateRows(t *testing.T) {
	a := fixtureSocket()
	a.Inode = "0"
	b := a
	b.State = "CLOSE"
	got := diffSockets([]platform.Socket{a, a}, []platform.Socket{a})
	if len(got.Removed) != 1 || len(got.Added) != 0 || len(got.Updated) != 0 {
		t.Fatalf("lost duplicate removal: %+v", got)
	}
	got = diffSockets([]platform.Socket{a}, []platform.Socket{a, a})
	if len(got.Added) != 1 {
		t.Fatalf("lost duplicate addition: %+v", got)
	}
	got = diffSockets([]platform.Socket{a, b}, []platform.Socket{b})
	if len(got.Removed) != 1 || got.Removed[0].State != "LISTEN" || len(got.Updated) != 0 {
		t.Fatalf("exact match should survive: %+v", got)
	}
	got = diffSockets([]platform.Socket{a, a}, []platform.Socket{a, b})
	if len(got.Updated) != 1 || len(got.Added) != 0 || len(got.Removed) != 0 {
		t.Fatalf("lost duplicate update: %+v", got)
	}
}

func TestDiffDeterministic(t *testing.T) {
	a, b := fixtureSocket(), fixtureSocket()
	b.Inode = "100"
	first, _ := json.Marshal(diffSockets(nil, []platform.Socket{a, b}))
	second, _ := json.Marshal(diffSockets(nil, []platform.Socket{b, a}))
	if !bytes.Equal(first, second) {
		t.Fatalf("unstable output: %s versus %s", first, second)
	}
}

func TestWatchChangesQuietPollsCount(t *testing.T) {
	calls := 0
	original := fixtureSocket()
	changed := original
	changed.State = "CLOSE"
	list := func() ([]platform.Socket, error) {
		calls++
		switch calls {
		case 1, 2:
			return []platform.Socket{original}, nil
		case 3:
			return []platform.Socket{changed}, nil
		default:
			return nil, nil
		}
	}
	var out bytes.Buffer
	err := watchSnapshots(context.Background(), &out, socketOptions{json: true, changes: true}, time.Millisecond, 5, list)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 5 {
		t.Fatalf("--count counted events instead of polls: %d", calls)
	}
	decoder := json.NewDecoder(&out)
	var baseline, update, removal changeSet
	for _, event := range []*changeSet{&baseline, &update, &removal} {
		if err := decoder.Decode(event); err != nil {
			t.Fatal(err)
		}
		if event.Timestamp.IsZero() || event.Added == nil || event.Removed == nil || event.Updated == nil {
			t.Fatalf("invalid event schema: %+v", event)
		}
	}
	if !baseline.Baseline || len(baseline.Added) != 1 {
		t.Fatalf("bad baseline: %+v", baseline)
	}
	if update.Baseline || len(update.Updated) != 1 {
		t.Fatalf("bad update: %+v", update)
	}
	if removal.Baseline || len(removal.Removed) != 1 {
		t.Fatalf("bad removal: %+v", removal)
	}
	var extra changeSet
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("unchanged polls should be silent: %v", err)
	}
}

func TestWatchChangesFilteredBaseline(t *testing.T) {
	var out bytes.Buffer
	calls := 0
	err := watchSnapshots(context.Background(), &out, socketOptions{json: true, changes: true, filter: socketFilter{port: 9000}}, time.Millisecond, 3, func() ([]platform.Socket, error) {
		calls++
		row := fixtureSocket()
		row.State = strings.Repeat("A", calls)
		return []platform.Socket{row}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var baseline changeSet
	if len(lines) != 1 {
		t.Fatalf("filtered-out changes emitted: %s", &out)
	}
	if err := json.Unmarshal([]byte(lines[0]), &baseline); err != nil {
		t.Fatal(err)
	}
	if !baseline.Baseline || baseline.hasChanges() {
		t.Fatalf("expected empty baseline: %+v", baseline)
	}
}

type failAfterWriter struct{ remaining int }

func (w *failAfterWriter) Write(p []byte) (int, error) {
	if w.remaining == 0 {
		return 0, errors.New("output failed")
	}
	w.remaining--
	return len(p), nil
}

func TestWriteChanges(t *testing.T) {
	a := fixtureSocket()
	b := a
	b.State = "CLOSE"
	event := changeSet{Timestamp: time.Now().UTC(), Added: []platform.Socket{a}, Removed: []platform.Socket{a}, Updated: []socketUpdate{{Before: a, After: b}}}
	var out bytes.Buffer
	if err := writeChanges(&out, event, false); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Changes ", "ADDED", "REMOVED", "UPDATED BEFORE", "UPDATED AFTER", "LISTEN", "CLOSE"} {
		if !strings.Contains(out.String(), label) {
			t.Fatalf("missing %s in %s", label, &out)
		}
	}
	for n := 0; n < 9; n++ {
		if err := writeChanges(&failAfterWriter{remaining: n}, event, false); err == nil {
			t.Fatalf("write failure %d lost", n)
		}
	}
	if err := writeChanges(brokenWriter{}, event, true); err == nil {
		t.Fatal("JSON write failure lost")
	}
	event.Baseline = true
	for n := 0; n < 2; n++ {
		if err := writeChanges(&failAfterWriter{remaining: n}, event, false); err == nil {
			t.Fatalf("baseline failure %d lost", n)
		}
	}
	out.Reset()
	if err := writeChanges(&out, event, false); err != nil || !strings.Contains(out.String(), "Baseline ") {
		t.Fatalf("%s %v", &out, err)
	}
}

func TestWatchChangesErrorsAndCancellation(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		err := watchSnapshots(context.Background(), brokenWriter{}, socketOptions{json: asJSON, changes: true}, time.Millisecond, 1, func() ([]platform.Socket, error) {
			return nil, nil
		})
		if err == nil {
			t.Fatal("changes mode lost output error")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out := &cancelWriter{cancel: cancel}
	err := watchSnapshots(ctx, out, socketOptions{json: true, changes: true}, time.Hour, 0, func() ([]platform.Socket, error) { return nil, nil })
	if err != nil || out.calls != 1 {
		t.Fatalf("cancellation failed: %v", err)
	}
}
