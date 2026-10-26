package platform

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
)

// Decode documented MIB_*_OWNER_PID records by byte offsets, not Go layout.
func parseWindowsTable(data []byte, protocol string, ipv6 bool) ([]Socket, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("truncated Windows socket table")
	}
	stride := 24
	if protocol == "udp" {
		stride = 12
	}
	if ipv6 {
		stride = 56
		if protocol == "udp" {
			stride = 28
		}
	}
	count := uint64(binary.LittleEndian.Uint32(data[:4]))
	if count > uint64((len(data)-4)/stride) {
		return nil, fmt.Errorf("truncated Windows socket records")
	}
	rows := make([]Socket, 0, int(count))
	for i := 0; i < int(count); i++ {
		b := data[4+i*stride : 4+(i+1)*stride]
		row := Socket{Protocol: protocol, PIDs: []int{}, Processes: []Process{}}
		pid := 0
		if !ipv6 {
			row.Local = windowsEndpoint(b[4:8], b[8:10], 0)
			if protocol == "tcp" {
				row.State = windowsTCPState(binary.LittleEndian.Uint32(b[:4]))
				row.Remote = windowsEndpoint(b[12:16], b[16:18], 0)
				pid = int(binary.LittleEndian.Uint32(b[20:24]))
			} else {
				row.Local = windowsEndpoint(b[:4], b[4:6], 0)
				row.Remote = "0.0.0.0:0"
				row.State = "BOUND"
				pid = int(binary.LittleEndian.Uint32(b[8:12]))
			}
		} else {
			row.Local = windowsEndpoint(b[:16], b[20:22], binary.LittleEndian.Uint32(b[16:20]))
			if protocol == "tcp" {
				row.Remote = windowsEndpoint(b[24:40], b[44:46], binary.LittleEndian.Uint32(b[40:44]))
				row.State = windowsTCPState(binary.LittleEndian.Uint32(b[48:52]))
				pid = int(binary.LittleEndian.Uint32(b[52:56]))
			} else {
				row.Remote = "[::]:0"
				row.State = "BOUND"
				pid = int(binary.LittleEndian.Uint32(b[24:28]))
			}
		}
		if pid > 0 {
			row.PIDs = append(row.PIDs, pid)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func windowsEndpoint(ip, port []byte, scope uint32) string {
	host := net.IP(ip).String()
	if len(ip) == 16 && scope != 0 {
		host += "%" + strconv.FormatUint(uint64(scope), 10)
	}
	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(port))))
}

func windowsTCPState(state uint32) string {
	names := []string{"", "CLOSE", "LISTEN", "SYN_SENT", "SYN_RECV", "ESTABLISHED", "FIN_WAIT1", "FIN_WAIT2", "CLOSE_WAIT", "CLOSING", "LAST_ACK", "TIME_WAIT", "DELETE_TCB"}
	if state > 0 && int(state) < len(names) {
		return names[state]
	}
	return fmt.Sprintf("UNKNOWN(%d)", state)
}
