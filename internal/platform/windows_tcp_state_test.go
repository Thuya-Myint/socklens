package platform

import (
	"encoding/binary"
	"testing"
)

func TestWindowsListenerQueryProvidesStateEvidence(t *testing.T) {
	for _, ipv6 := range []bool{false, true} {
		stride, stateOffset := 24, 0
		if ipv6 {
			stride, stateOffset = 56, 48
		}
		data := make([]byte, 4+stride)
		binary.LittleEndian.PutUint32(data[:4], 1)
		rows, err := parseWindowsTCPTable(data, ipv6, true)
		if err != nil || len(rows) != 1 || rows[0].State != "LISTEN" || !rows[0].IsListener() {
			t.Fatalf("listener query: %+v %v", rows, err)
		}
		rows, err = parseWindowsTCPTable(data, ipv6, false)
		if err != nil || rows[0].State != "UNKNOWN(0)" || rows[0].IsListener() {
			t.Fatalf("unknown connection state guessed: %+v %v", rows, err)
		}
		binary.LittleEndian.PutUint32(data[4+stateOffset:4+stateOffset+4], 5)
		rows, err = parseWindowsTCPTable(data, ipv6, false)
		if err != nil || rows[0].State != "ESTABLISHED" || rows[0].IsListener() {
			t.Fatalf("connection state lost: %+v %v", rows, err)
		}
	}
	if _, err := parseWindowsTCPTable([]byte{1}, false, true); err == nil {
		t.Fatal("truncated listener table accepted")
	}
}
