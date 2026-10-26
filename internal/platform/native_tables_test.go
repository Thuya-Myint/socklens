package platform

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestWindowsTableLayouts(t *testing.T) {
	le := binary.LittleEndian
	for _, tc := range []struct {
		protocol string
		v6       bool
		size     int
	}{{"tcp", false, 24}, {"udp", false, 12}, {"tcp", true, 56}, {"udp", true, 28}} {
		data := make([]byte, 4+tc.size)
		le.PutUint32(data, 1)
		row := data[4:]
		if tc.v6 {
			copy(row, net.ParseIP("fe80::1").To16())
			le.PutUint32(row[16:20], 3)
			binary.BigEndian.PutUint16(row[20:22], 8080)
			if tc.protocol == "tcp" {
				le.PutUint32(row[48:52], 2)
				le.PutUint32(row[52:56], 42)
			} else {
				le.PutUint32(row[24:28], 42)
			}
		} else if tc.protocol == "tcp" {
			le.PutUint32(row, 2)
			copy(row[4:8], net.ParseIP("127.0.0.1").To4())
			binary.BigEndian.PutUint16(row[8:10], 8080)
			le.PutUint32(row[20:24], 42)
		} else {
			copy(row[:4], net.ParseIP("127.0.0.1").To4())
			binary.BigEndian.PutUint16(row[4:6], 8080)
			le.PutUint32(row[8:12], 42)
		}
		rows, err := parseWindowsTable(data, tc.protocol, tc.v6)
		if err != nil || len(rows) != 1 {
			t.Fatalf("%+v %v", tc, err)
		}
		want := "127.0.0.1:8080"
		if tc.v6 {
			want = "[fe80::1%3]:8080"
		}
		if rows[0].Local != want || len(rows[0].PIDs) != 1 || rows[0].PIDs[0] != 42 {
			t.Fatalf("%+v", rows[0])
		}
		state := "LISTEN"
		if tc.protocol == "udp" {
			state = "BOUND"
		}
		if rows[0].State != state {
			t.Fatal(rows[0].State)
		}
		if _, err := parseWindowsTable(data[:len(data)-1], tc.protocol, tc.v6); err == nil {
			t.Fatal("accepted truncated table")
		}
	}
	if _, err := parseWindowsTable(nil, "tcp", false); err == nil {
		t.Fatal("accepted missing header")
	}
}

func darwinFixture(protocol string, v6 bool) []byte {
	le := binary.LittleEndian
	header := make([]byte, 24)
	le.PutUint32(header, 24)
	pcb := make([]byte, 104)
	le.PutUint32(pcb, 104)
	le.PutUint32(pcb[4:], 0x10)
	le.PutUint64(pcb[8:], 123)
	le.PutUint64(pcb[28:], 456)
	binary.BigEndian.PutUint16(pcb[18:], 8080)
	if v6 {
		pcb[44] = 2
		copy(pcb[64:80], net.ParseIP("::1").To16())
	} else {
		pcb[44] = 1
		copy(pcb[76:80], net.ParseIP("127.0.0.1").To4())
	}
	socket := make([]byte, 104)
	le.PutUint32(socket, 104)
	le.PutUint32(socket[4:], 1)
	le.PutUint32(socket[68:], 42)
	data := append(append(header, pcb...), socket...)
	if protocol == "tcp" {
		tcp := make([]byte, 40)
		le.PutUint32(tcp, 40)
		le.PutUint32(tcp[4:], 0x20)
		le.PutUint32(tcp[36:], 1)
		data = append(data, tcp...)
	}
	footer := make([]byte, 24)
	le.PutUint32(footer, 24)
	return append(data, footer...)
}
func TestDarwinPCBLayouts(t *testing.T) {
	for _, protocol := range []string{"tcp", "udp"} {
		for _, v6 := range []bool{false, true} {
			data := darwinFixture(protocol, v6)
			rows, err := parseDarwinPCBList(data, protocol)
			if err != nil || len(rows) != 1 {
				t.Fatalf("%s v6=%v: %+v %v", protocol, v6, rows, err)
			}
			want := "127.0.0.1:8080"
			if v6 {
				want = "[::1]:8080"
			}
			if rows[0].Local != want || rows[0].PIDs[0] != 42 || rows[0].Inode != "7b-1c8" {
				t.Fatalf("%+v", rows[0])
			}
			state := "UNCONN"
			if protocol == "tcp" {
				state = "LISTEN"
			}
			if rows[0].State != state {
				t.Fatal(rows[0].State)
			}
			for _, length := range []int{0, 23, len(data) - 1, len(data) - 24} {
				if _, err := parseDarwinPCBList(data[:length], protocol); err == nil {
					t.Fatalf("accepted truncated Darwin data length %d", length)
				}
			}
		}
	}
	bad := darwinFixture("tcp", false)
	binary.LittleEndian.PutUint32(bad[24:], 4)
	if _, err := parseDarwinPCBList(bad, "tcp"); err == nil {
		t.Fatal("accepted invalid record length")
	}
}
