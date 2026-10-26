package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

const help = `SockLens — inspect local TCP/UDP ports

Usage:
  socklens help
  socklens version
  socklens check
  socklens list [--port PORT] [--protocol tcp|udp] [--pid PID] [--listening] [--json]
  socklens inspect [--protocol tcp|udp] [--pid PID] [--listening] [--json] PORT
  socklens watch [--interval 2s] [--count N] [--changes] [list filters] [--json]
  socklens wait [--until present|absent] [--timeout 30s] [--interval 500ms] [filters] PORT
  socklens terminate --pid PID [--dry-run]

list includes listening, connected, and bound sockets in this network namespace.
watch repeats snapshots until Ctrl+C; --count limits the number of snapshots.
watch --changes reports additions, removals, and updates; unchanged polls are silent.
--listening selects TCP LISTEN and UDP UNCONN sockets.
Process names are best-effort; unavailable owners are shown as "-".
wait polls until a local socket condition matches; --timeout 0 waits indefinitely.
Flags must precede the port argument for inspect and wait.
terminate asks you to type the PID, then sends SIGTERM through a stable process handle.
terminate --dry-run previews the target without prompting or sending a signal.
`

func Run(args []string, in io.Reader, out, errOut io.Writer, version string) int {
	return RunContext(context.Background(), args, in, out, errOut, version)
}

// RunContext allows long-running commands to stop when their caller cancels.
func RunContext(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, version string) int {
	if len(args) == 0 {
		if _, err := fmt.Fprint(out, help); err != nil {
			return fail(errOut, err)
		}
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		if len(args) != 1 {
			return fail(errOut, fmt.Errorf("help takes no arguments"))
		}
		if _, err := fmt.Fprint(out, help); err != nil {
			return fail(errOut, err)
		}
		return 0
	case "version", "--version":
		if len(args) != 1 {
			return fail(errOut, fmt.Errorf("version takes no arguments"))
		}
		if _, err := fmt.Fprintln(out, "socklens "+version); err != nil {
			return fail(errOut, err)
		}
		return 0
	case "check":
		if len(args) != 1 {
			return fail(errOut, fmt.Errorf("check takes no arguments"))
		}
		rows, err := platform.List()
		if err != nil {
			return fail(errOut, err)
		}
		if _, err := fmt.Fprintf(out, "OK: %s/%s; proc socket tables readable (%d sockets). Process ownership may be hidden by permissions.\n", runtime.GOOS, runtime.GOARCH, len(rows)); err != nil {
			return fail(errOut, err)
		}
		return 0
	case "list", "inspect":
		return sockets(args[0], args[1:], out, errOut)
	case "watch":
		return watchCommand(ctx, args[1:], out, errOut)
	case "wait":
		return waitCommand(ctx, args[1:], out, errOut)
	case "terminate":
		return terminate(args[1:], in, out, errOut)
	default:
		return fail(errOut, fmt.Errorf("unknown command %q; run socklens help", args[0]))
	}
}

func fail(w io.Writer, err error) int { fmt.Fprintln(w, "socklens:", err); return 1 }

type socketOptions struct {
	changes bool // watch only
	filter  socketFilter
	json    bool
}

func addSocketFlags(fs *flag.FlagSet) *socketOptions {
	opts := &socketOptions{}
	fs.IntVar(&opts.filter.port, "port", 0, "local port (1-65535)")
	fs.StringVar(&opts.filter.protocol, "protocol", "", "tcp or udp")
	fs.IntVar(&opts.filter.pid, "pid", 0, "owning process ID (positive integer)")
	fs.BoolVar(&opts.filter.listening, "listening", false, "TCP listeners and unconnected UDP sockets")
	fs.BoolVar(&opts.json, "json", false, "JSON output (watch: newline-delimited records)")
	return opts
}

func (opts *socketOptions) validate(fs *flag.FlagSet) error {
	supplied := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { supplied[f.Name] = true })
	if supplied["port"] && (opts.filter.port < 1 || opts.filter.port > 65535) {
		return fmt.Errorf("port must be 1-65535")
	}
	if supplied["pid"] && opts.filter.pid < 1 {
		return fmt.Errorf("PID must be a positive integer")
	}
	if fs.Name() == "inspect" || fs.Name() == "wait" {
		if fs.NArg() != 1 || supplied["port"] {
			return fmt.Errorf("usage: socklens %s [flags] PORT (flags must precede PORT)", fs.Name())
		}
		n, err := strconv.Atoi(fs.Arg(0))
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("port must be 1-65535")
		}
		opts.filter.port = n
	} else if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	if opts.filter.protocol != "" && opts.filter.protocol != "tcp" && opts.filter.protocol != "udp" {
		return fmt.Errorf("protocol must be tcp or udp")
	}
	return nil
}

func sockets(command string, args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	opts := addSocketFlags(fs)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if err := opts.validate(fs); err != nil {
		return fail(errOut, err)
	}
	rows, err := platform.List()
	if err != nil {
		return fail(errOut, err)
	}
	filtered, err := filterSockets(rows, opts.filter)
	if err != nil {
		return fail(errOut, err)
	}
	if err := writeSockets(out, filtered, opts.json); err != nil {
		return fail(errOut, err)
	}
	return 0
}

type socketFilter struct {
	port      int
	protocol  string
	pid       int
	listening bool
}

func filterSockets(rows []platform.Socket, filter socketFilter) ([]platform.Socket, error) {
	filtered := make([]platform.Socket, 0)
	for _, row := range rows {
		_, port, err := net.SplitHostPort(row.Local)
		if err != nil {
			return nil, err
		}
		if filter.port != 0 && port != strconv.Itoa(filter.port) {
			continue
		}
		if filter.protocol != "" && row.Protocol != filter.protocol {
			continue
		}
		if filter.listening && !((row.Protocol == "tcp" && row.State == "LISTEN") ||
			(row.Protocol == "udp" && row.State == "UNCONN")) {
			continue
		}
		if filter.pid != 0 {
			found := false
			for _, pid := range row.PIDs {
				if pid == filter.pid {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		filtered = append(filtered, row)
	}
	return filtered, nil
}

func writeSockets(out io.Writer, rows []platform.Socket, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "PROTO	LOCAL	REMOTE	STATE	PIDS	PROCESSES")
	for _, row := range rows {
		pids := make([]string, len(row.PIDs))
		for i, pid := range row.PIDs {
			pids[i] = strconv.Itoa(pid)
		}
		owner := strings.Join(pids, ",")
		if owner == "" {
			owner = "-"
		}
		names := make([]string, 0, len(row.Processes))
		for _, process := range row.Processes {
			if process.Name != "" {
				// Kernel process names can contain tabs, newlines, or terminal escapes.
				names = append(names, fmt.Sprintf("%d:%s", process.PID, strconv.QuoteToASCII(process.Name)))
			}
		}
		detail := strings.Join(names, ",")
		if detail == "" {
			detail = "-"
		}
		fmt.Fprintf(w, "%s	%s	%s	%s	%s	%s\n", row.Protocol, row.Local, row.Remote, row.State, owner, detail)
	}
	return w.Flush()
}

func terminate(args []string, in io.Reader, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("terminate", flag.ContinueOnError)
	fs.SetOutput(errOut)
	pid := fs.Int("pid", 0, "process ID to terminate")
	dryRun := fs.Bool("dry-run", false, "preview the target without sending a signal")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 || *pid <= 1 {
		return fail(errOut, fmt.Errorf("usage: socklens terminate --pid PID [--dry-run] (PID must exceed 1)"))
	}
	if *dryRun {
		id, err := platform.Identify(*pid)
		if err != nil {
			return fail(errOut, err)
		}
		if _, err := fmt.Fprintf(out, "Dry run: would send SIGTERM to %q (PID %d). No signal sent.\n", id.Name, id.PID); err != nil {
			return fail(errOut, err)
		}
		return 0
	}
	target, err := platform.OpenTarget(*pid)
	if err != nil {
		return fail(errOut, err)
	}
	defer target.Close()
	id := target.Identity()
	if _, err := fmt.Fprintf(out, "Send SIGTERM to %q (PID %d)? Type %d to confirm: ", id.Name, id.PID, id.PID); err != nil {
		return fail(errOut, err)
	}
	line, err := bufio.NewReader(io.LimitReader(in, 64)).ReadString('\n')
	if err != nil {
		return fail(errOut, fmt.Errorf("confirmation requires a complete line; cancelled"))
	}
	if strings.TrimSpace(line) != strconv.Itoa(id.PID) {
		return fail(errOut, fmt.Errorf("confirmation did not match; cancelled"))
	}
	if err := target.Terminate(); err != nil {
		return fail(errOut, err)
	}
	if _, err := fmt.Fprintf(out, "SIGTERM sent to PID %d. The process may take time to exit.\n", id.PID); err != nil {
		return fail(errOut, err)
	}
	return 0
}
