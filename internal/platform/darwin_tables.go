package platform

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
)

// Decode Apple's packed, length-tagged pcblist_n records. Unknown record kinds
// are skipped; malformed/truncated known records fail rather than imply absence.
func parseDarwinPCBList(data []byte, protocol string) ([]Socket, error) {
	le := binary.LittleEndian
	if len(data) < 48 || le.Uint32(data[:4]) != 24 {
		return nil, fmt.Errorf("unsupported or truncated macOS PCB header")
	}
	rows := make([]Socket, 0)
	var row *Socket
	haveSocket, haveTCP := false, false
	flush := func() error {
		if row == nil {
			return nil
		}
		if !haveSocket || (protocol == "tcp" && !haveTCP) {
			return fmt.Errorf("incomplete macOS socket record")
		}
		rows = append(rows, *row)
		return nil
	}
	for offset := 24; offset < len(data); {
		if len(data)-offset < 8 {
			return nil, fmt.Errorf("truncated macOS PCB record")
		}
		length := int(le.Uint32(data[offset : offset+4]))
		if length == 24 && offset+24 == len(data) {
			if err := flush(); err != nil {
				return nil, err
			}
			return rows, nil
		}
		if length < 8 || length > len(data)-offset {
			return nil, fmt.Errorf("invalid macOS PCB record length")
		}
		record := data[offset : offset+length]
		switch le.Uint32(record[4:8]) {
		case 0x10: // XSO_INPCB.
			if err := flush(); err != nil {
				return nil, err
			}
			if length < 104 {
				return nil, fmt.Errorf("short macOS Internet PCB")
			}
			ipv6 := record[44]&2 != 0
			local := record[64:80]
			remote := record[48:64]
			if !ipv6 {
				if record[44]&1 == 0 {
					return nil, fmt.Errorf("unknown macOS socket address family")
				}
				local = record[76:80]
				remote = record[60:64]
			}
			state := "UNCONN"
			if binary.BigEndian.Uint16(record[16:18]) != 0 {
				state = "CONNECTED"
			}
			row = &Socket{Protocol: protocol, Local: darwinEndpoint(local, record[18:20]), Remote: darwinEndpoint(remote, record[16:18]),
				State: state, Inode: fmt.Sprintf("%x-%x", le.Uint64(record[8:16]), le.Uint64(record[28:36])), PIDs: []int{}, Processes: []Process{}}
			haveSocket, haveTCP = false, false
		case 0x1: // XSO_SOCKET.
			if row == nil || length < 84 {
				return nil, fmt.Errorf("short or orphan macOS socket")
			}
			pid := int(int32(le.Uint32(record[68:72])))
			if pid > 0 {
				row.PIDs = []int{pid}
			}
			haveSocket = true
		case 0x20: // XSO_TCPCB; four externally fixed timer slots.
			if row == nil || length < 40 {
				return nil, fmt.Errorf("short or orphan macOS TCP control block")
			}
			row.State = darwinTCPState(le.Uint32(record[36:40]))
			haveTCP = true
		}
		step := (length + 7) &^ 7
		if step > len(data)-offset {
			return nil, fmt.Errorf("truncated macOS PCB alignment")
		}
		offset += step
	}
	return nil, fmt.Errorf("missing macOS PCB trailer")
}

func darwinEndpoint(raw, port []byte) string {
	ip := append(net.IP{}, raw...)
	zone := ""
	if len(ip) == 16 && ip.IsLinkLocalUnicast() {
		scope := binary.BigEndian.Uint16(ip[2:4])
		if scope != 0 {
			zone = "%" + strconv.Itoa(int(scope))
			ip[2], ip[3] = 0, 0
		}
	}
	return net.JoinHostPort(ip.String()+zone, strconv.Itoa(int(binary.BigEndian.Uint16(port))))
}

func darwinTCPState(state uint32) string {
	names := []string{"CLOSE", "LISTEN", "SYN_SENT", "SYN_RECV", "ESTABLISHED", "CLOSE_WAIT", "FIN_WAIT1", "CLOSING", "LAST_ACK", "FIN_WAIT2", "TIME_WAIT"}
	if int(state) < len(names) {
		return names[state]
	}
	return fmt.Sprintf("UNKNOWN(%d)", state)
}
