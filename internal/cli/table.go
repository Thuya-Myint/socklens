package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Thuya-Myint/socklens/internal/platform"
	"github.com/Thuya-Myint/socklens/internal/terminal"
)

func processName(name string) string {
	// Keep common names readable; quote everything else to neutralize controls,
	// terminal escape sequences, ambiguous delimiters, and wide Unicode cells.
	if name != "" && strings.IndexFunc(name, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._-", r))
	}) == -1 {
		return name
	}
	return strconv.QuoteToASCII(name)
}

func stateRole(state string) terminal.Role {
	switch state {
	case "LISTEN", "BOUND", "UNCONN":
		return terminal.Success
	case "ESTABLISHED", "CONNECTED":
		return terminal.Info
	case "CLOSE", "CLOSED", "CLOSE_WAIT", "CLOSING", "TIME_WAIT", "FIN_WAIT1", "FIN_WAIT2", "LAST_ACK":
		return terminal.Warning
	default:
		return terminal.Muted
	}
}

func writeSocketTable(out io.Writer, rows []platform.Socket) error {
	return writeSocketTableWidth(out, rows, terminal.Width(out))
}

func writeSocketTableWidth(out io.Writer, rows []platform.Socket, columns int) error {
	if len(rows) == 0 {
		return writeFolded(out, terminal.Info, "INFO No matching sockets.", columns)
	}
	header := [6]string{"PROTO", "LOCAL", "REMOTE", "STATE", "PIDS", "PROCESSES"}
	widths := [6]int{}
	for i, cell := range header {
		widths[i] = len(cell)
	}
	cells := make([][6]string, 0, len(rows))
	listening, connected := 0, 0
	missing := false
	for _, row := range rows {
		pids := make([]string, len(row.PIDs))
		for i, pid := range row.PIDs {
			pids[i] = strconv.Itoa(pid)
		}
		names := make([]string, 0, len(row.Processes))
		for _, process := range row.Processes {
			if process.Name != "" {
				name := processName(process.Name)
				if len(row.PIDs) > 1 || len(row.Processes) > 1 {
					name = fmt.Sprintf("%d:%s", process.PID, name)
				}
				names = append(names, name)
			}
		}
		owner, detail := strings.Join(pids, ","), strings.Join(names, ", ")
		if owner == "" {
			owner = "-"
		}
		if detail == "" {
			detail = "-"
		}
		if owner == "-" || len(names) < len(row.PIDs) {
			missing = true
		}
		state := row.State
		if state == "" {
			state = "-"
		}
		remote := row.Remote
		if remote == "" {
			remote = "-"
		}
		line := [6]string{row.Protocol, row.Local, remote, state, owner, detail}
		cells = append(cells, line)
		for i, cell := range line {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
		switch row.State {
		case "LISTEN", "BOUND", "UNCONN":
			listening++
		case "ESTABLISHED", "CONNECTED":
			connected++
		}
	}
	writeRow := func(line [6]string, heading bool) error {
		var text strings.Builder
		for i, cell := range line {
			role := terminal.Role("")
			if heading {
				role = terminal.Heading
			} else if i == 0 {
				role = terminal.Info
			} else if i == 3 {
				role = stateRole(cell)
			} else if cell == "-" {
				role = terminal.Muted
			}
			if role != "" {
				text.WriteString(terminal.Style(out, role, cell))
			} else {
				text.WriteString(cell)
			}
			// Padding is based on plain ASCII cells, never the ANSI escape byte count.
			if i < len(line)-1 {
				text.WriteString(strings.Repeat(" ", widths[i]-len(cell)+2))
			}
		}
		text.WriteByte('\n')
		_, err := io.WriteString(out, text.String())
		return err
	}
	tableWidth := 2 * (len(header) - 1)
	for _, width := range widths {
		tableWidth += width
	}
	if columns > 0 && tableWidth > columns {
		if err := writeStackedSockets(out, cells, columns); err != nil {
			return err
		}
	} else {
		if err := writeRow(header, true); err != nil {
			return err
		}
		for _, line := range cells {
			if err := writeRow(line, false); err != nil {
				return err
			}
		}
	}
	summary := fmt.Sprintf("%s | %d listening/bound | %d connected", socketCount(len(rows)), listening, connected)
	summaries := []string{summary}
	if columns > 0 && len(summary) > columns {
		summaries = []string{socketCount(len(rows)), fmt.Sprintf("%d listening/bound", listening), fmt.Sprintf("%d connected", connected)}
	}
	for _, text := range summaries {
		if err := writeFolded(out, terminal.Muted, text, columns); err != nil {
			return err
		}
	}
	if missing {
		return writeFolded(out, terminal.Info, "INFO Some process details are unavailable; permissions may limit visibility.", columns)
	}
	return nil
}

func socketCount(count int) string {
	if count == 1 {
		return "1 socket"
	}
	return fmt.Sprintf("%d sockets", count)
}
