package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

// JSON v1 is opt-in. Legacy --json output remains unchanged for compatibility.
type socketJSONV1 struct {
	Protocol  string             `json:"protocol"`
	Local     endpointJSONV1     `json:"local"`
	Remote    endpointJSONV1     `json:"remote"`
	State     string             `json:"state"`
	Inode     string             `json:"inode,omitempty"`
	PIDs      []int              `json:"pids"`
	Processes []processJSONV1    `json:"processes"`
}

type endpointJSONV1 struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type processJSONV1 struct {
	PID  int    `json:"pid"`
	Name string `json:"name"`
}

type socketResponseJSONV1 struct {
	SchemaVersion string         `json:"schema_version"`
	Command       string         `json:"command"`
	GeneratedAt   time.Time      `json:"generated_at"`
	Count         int            `json:"count"`
	Sockets       []socketJSONV1 `json:"sockets"`
}

func endpointV1(raw string) (endpointJSONV1, error) {
	host, port, err := net.SplitHostPort(raw)
	if err != nil {
		return endpointJSONV1{}, fmt.Errorf("invalid endpoint %q: %w", raw, err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return endpointJSONV1{}, fmt.Errorf("invalid endpoint port %q", port)
	}
	return endpointJSONV1{Address: host, Port: n}, nil
}

func writeSocketsV1(out io.Writer, command string, rows []platform.Socket) error {
	result := socketResponseJSONV1{
		SchemaVersion: "1.0",
		Command: command,
		GeneratedAt: time.Now().UTC(),
		Count: len(rows),
		Sockets: make([]socketJSONV1, 0, len(rows)),
	}
	for _, row := range rows {
		local, err := endpointV1(row.Local)
		if err != nil {
			return err
		}
		remote, err := endpointV1(row.Remote)
		if err != nil {
			// Some OS backends report an empty remote for unconnected sockets.
			if strings.TrimSpace(row.Remote) != "" {
				return err
			}
			remote = endpointJSONV1{}
		}
		item := socketJSONV1{
			Protocol: row.Protocol, Local: local, Remote: remote,
			State: row.State, Inode: row.Inode,
			PIDs: append([]int{}, row.PIDs...),
			Processes: make([]processJSONV1, 0, len(row.Processes)),
		}
		for _, process := range row.Processes {
			item.Processes = append(item.Processes, processJSONV1{PID: process.PID, Name: process.Name})
		}
		result.Sockets = append(result.Sockets, item)
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
