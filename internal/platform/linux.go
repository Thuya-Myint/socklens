//go:build linux

package platform

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

func endpoint(raw string) (string, error) {
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid endpoint %q", raw)
	}
	ip, err := hex.DecodeString(parts[0])
	if err != nil || (len(ip) != 4 && len(ip) != 16) {
		return "", fmt.Errorf("invalid address %q", raw)
	}
	// proc uses host-endian 32-bit words, including each word of IPv6.
	for i := 0; i < len(ip); i += 4 {
		n := binary.NativeEndian.Uint32(ip[i : i+4])
		binary.BigEndian.PutUint32(ip[i:i+4], n)
	}
	port, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(net.IP(ip).String(), strconv.FormatUint(port, 10)), nil
}

func parseTable(text, protocol string) ([]Socket, error) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("empty proc socket table")
	}
	var result []Socket
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		if len(fields) < 10 {
			return nil, fmt.Errorf("malformed %s socket row", protocol)
		}
		local, err := endpoint(fields[1])
		if err != nil {
			return nil, err
		}
		remote, err := endpoint(fields[2])
		if err != nil {
			return nil, err
		}
		state := fields[3]
		if protocol == "tcp" {
			names := map[string]string{"01": "ESTABLISHED", "02": "SYN_SENT", "03": "SYN_RECV", "04": "FIN_WAIT1", "05": "FIN_WAIT2", "06": "TIME_WAIT", "07": "CLOSE", "08": "CLOSE_WAIT", "09": "LAST_ACK", "0A": "LISTEN", "0B": "CLOSING"}
			if name, ok := names[state]; ok {
				state = name
			}
		} else if state == "07" {
			state = "UNCONN"
		} else if state == "01" {
			state = "CONNECTED"
		}
		result = append(result, Socket{Protocol: protocol, Local: local, Remote: remote, State: state, Inode: fields[9]})
	}
	return result, scanner.Err()
}

func List() ([]Socket, error) {
	var result []Socket
	for _, table := range []string{"tcp", "tcp6", "udp", "udp6"} {
		data, err := os.ReadFile("/proc/net/" + table)
		if os.IsNotExist(err) && strings.HasSuffix(table, "6") {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", table, err)
		}
		protocol := strings.TrimSuffix(table, "6")
		rows, err := parseTable(string(data), protocol)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", table, err)
		}
		result = append(result, rows...)
	}
	owners := map[string][]int{}
	names := map[int]string{}
	processes, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	for _, process := range processes {
		pid, err := strconv.Atoi(process.Name())
		if err != nil {
			continue
		}
		fds, err := os.ReadDir(filepath.Join("/proc", process.Name(), "fd"))
		if err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join("/proc", process.Name(), "fd", fd.Name()))
			if err != nil || !strings.HasPrefix(link, "socket:[") || !strings.HasSuffix(link, "]") {
				continue
			}
			inode := strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
			if !seen[inode] {
				owners[inode] = append(owners[inode], pid)
				seen[inode] = true
			}
		}
		if len(seen) > 0 {
			if name, err := os.ReadFile(filepath.Join("/proc", process.Name(), "comm")); err == nil {
				names[pid] = strings.TrimSuffix(string(name), "\n")
			}
		}
	}
	for i := range result {
		result[i].PIDs = owners[result[i].Inode]
		sort.Ints(result[i].PIDs)
		result[i].Processes = make([]Process, 0, len(result[i].PIDs))
		for _, pid := range result[i].PIDs {
			result[i].Processes = append(result[i].Processes, Process{PID: pid, Name: names[pid]})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Protocol != result[j].Protocol {
			return result[i].Protocol < result[j].Protocol
		}
		if result[i].Local != result[j].Local {
			return result[i].Local < result[j].Local
		}
		return result[i].Inode < result[j].Inode
	})
	return result, nil
}

func parseIdentity(pid int, data string) (Identity, error) {
	left, right := strings.Index(data, "("), strings.LastIndex(data, ")")
	if left < 0 || right <= left {
		return Identity{}, fmt.Errorf("invalid process stat")
	}
	fields := strings.Fields(data[right+1:])
	if len(fields) < 20 {
		return Identity{}, fmt.Errorf("truncated process stat")
	}
	return Identity{PID: pid, Name: data[left+1 : right], Start: fields[19]}, nil
}

func Identify(pid int) (Identity, error) {
	if pid <= 1 || pid == os.Getpid() {
		return Identity{}, fmt.Errorf("refusing protected PID %d", pid)
	}
	info, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	if err != nil {
		return Identity{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return Identity{}, fmt.Errorf("PID %d is not owned by the current user", pid)
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return Identity{}, err
	}
	return parseIdentity(pid, string(data))
}

// OpenTarget captures the kernel handle before the user confirms termination.
func OpenTarget(pid int) (Target, error) {
	initial, err := Identify(pid)
	if err != nil {
		return nil, err
	}
	fd, err := pidfdOpen(pid)
	if err != nil {
		return nil, err
	}
	current, err := Identify(pid)
	if err != nil || current.Start != initial.Start {
		syscall.Close(fd)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("process identity changed; refusing PID %d", pid)
	}
	return &linuxTarget{fd: fd, id: current}, nil
}

type linuxTarget struct {
	fd int
	id Identity
}

func (t *linuxTarget) Identity() Identity { return t.id }

func (t *linuxTarget) Close() error {
	if t.fd < 0 {
		return nil
	}
	fd := t.fd
	t.fd = -1
	return syscall.Close(fd)
}

func (t *linuxTarget) Terminate() error {
	if t.fd < 0 {
		return fmt.Errorf("process handle is closed")
	}
	current, err := Identify(t.id.PID)
	if err != nil {
		return err
	}
	if current.Start != t.id.Start {
		return fmt.Errorf("process identity changed; refusing to signal PID %d", t.id.PID)
	}
	// The signal targets the captured kernel object, never a recycled PID.
	return pidfdSignal(t.fd, syscall.SIGTERM)
}

// Terminate validates a previously observed identity and signals via a handle.
func Terminate(id Identity) error {
	target, err := OpenTarget(id.PID)
	if err != nil {
		return err
	}
	defer target.Close()
	if target.Identity().Start != id.Start {
		return fmt.Errorf("process identity changed; refusing to signal PID %d", id.PID)
	}
	return target.Terminate()
}
