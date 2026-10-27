package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/Thuya-Myint/socklens/internal/terminal"

	"github.com/Thuya-Myint/socklens/internal/platform"
	"github.com/Thuya-Myint/socklens/internal/update"
)

const help = `SockLens — inspect local TCP/UDP ports

Usage:
  socklens [--color auto|always|never] COMMAND [options]

Discover:
  list           List local TCP/UDP sockets
  inspect PORT   Inspect sockets on a local port
  check          Check socket discovery on this machine

Monitor:
  watch          Repeat snapshots; --changes shows only changes
  wait PORT      Wait for sockets to appear or disappear

Manage:
  terminate      Preview or confirm process termination with --pid PID
  update-check   Check GitHub for a newer stable release
  updates        Enable, disable, or view daily update notices

About:
  version        Show the installed version
  help           Show this overview

Examples:
  socklens list --listening
  socklens inspect --protocol tcp 8080
  socklens watch --changes --port 8080
  socklens wait --until absent --timeout 10s 8080
  socklens terminate --pid 12345 --dry-run

Options:
  --color MODE   auto (default), always, or never; NO_COLOR disables auto color
  --json         Structured output for list, inspect, watch, wait, update-check
  -h, --help     Command options, for example: socklens list --help

Put options before PORT for inspect and wait. JSON output never includes colors.
Windows termination requires --force and typed PID confirmation.
Linux/macOS use identity-checked SIGTERM with typed PID confirmation.
`

func Run(args []string, in io.Reader, out, errOut io.Writer, version string) int {
	return RunContext(context.Background(), args, in, out, errOut, version)
}

// RunContext allows long-running commands to stop when their caller cancels.
func RunContext(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, version string) int {
	var mode string
	var err error
	args, mode, err = ParseColorArgs(args)
	if err != nil {
		return fail(errOut, err)
	}
	if jsonRequested(args) {
		mode = "never"
	}
	out, errOut = terminal.Wrap(out, mode), terminal.Wrap(errOut, mode)
	if len(args) == 0 {
		if err := writeHelp(out); err != nil {
			return fail(errOut, err)
		}
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		if len(args) != 1 {
			return fail(errOut, fmt.Errorf("help takes no arguments"))
		}
		if err := writeHelp(out); err != nil {
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
		if err := status(out, terminal.Success, "OK", fmt.Sprintf("Socket discovery is available on %s/%s (%d sockets).", runtime.GOOS, runtime.GOARCH, len(rows))); err != nil {
			return fail(errOut, err)
		}
		return 0
	case "list", "inspect":
		return sockets(args[0], args[1:], out, errOut)
	case "watch":
		return watchCommand(ctx, args[1:], out, errOut)
	case "wait":
		return waitCommand(ctx, args[1:], out, errOut)
	case "update-check":
		return updateCheckCommand(ctx, args[1:], out, errOut, version, update.DefaultChecker())
	case "updates":
		service, err := update.DefaultService()
		if err != nil {
			return fail(errOut, err)
		}
		return updatesCommand(args[1:], out, errOut, service)
	case "terminate":
		return terminate(args[1:], in, out, errOut)
	default:
		return fail(errOut, fmt.Errorf("unknown command %q; run socklens help", args[0]))
	}
}

func fail(w io.Writer, err error) int {
	_ = status(w, terminal.Error, "ERROR", "socklens: "+err.Error())
	return 1
}

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
	fs := newCommandFlags(command, errOut)
	opts := addSocketFlags(fs)
	if err := parseCommandFlags(fs, args, errOut); err != nil {
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
			(row.Protocol == "udp" && (row.State == "UNCONN" || row.State == "BOUND"))) {
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
	return writeSocketTable(out, rows)
}

func terminate(args []string, in io.Reader, out, errOut io.Writer) int {
	fs := newCommandFlags("terminate", errOut)
	pid := fs.Int("pid", 0, "process ID to terminate")
	dryRun := fs.Bool("dry-run", false, "preview the target without sending a signal")
	force := fs.Bool("force", false, "Windows only: allow immediate process termination")
	if err := parseCommandFlags(fs, args, errOut); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 || *pid <= 1 {
		return fail(errOut, fmt.Errorf("usage: socklens terminate --pid PID [--dry-run] [--force] (PID must exceed 1)"))
	}
	if *force && runtime.GOOS != "windows" {
		return fail(errOut, fmt.Errorf("--force is only supported on Windows"))
	}
	if runtime.GOOS == "windows" && !*dryRun && !*force {
		return fail(errOut, fmt.Errorf("Windows termination is immediate, not graceful; use --force to explicitly allow it"))
	}
	action := platform.TerminationAction()
	if *dryRun {
		id, err := platform.Identify(*pid)
		if err != nil {
			return fail(errOut, processError(*pid, err))
		}
		if err := status(out, terminal.Info, "DRY RUN", fmt.Sprintf("Would send %s to %q (PID %d). No signal sent.", action, id.Name, id.PID)); err != nil {
			return fail(errOut, err)
		}
		return 0
	}
	target, err := platform.OpenTarget(*pid)
	if err != nil {
		return fail(errOut, processError(*pid, err))
	}
	defer target.Close()
	id := target.Identity()
	if err := status(out, terminal.Warning, "CONFIRM", fmt.Sprintf("Send %s to %q (PID %d).", action, id.Name, id.PID)); err != nil {
		return fail(errOut, err)
	}
	if runtime.GOOS == "windows" {
		if err := status(out, terminal.Warning, "WARN", "Termination is immediate; unsaved work may be lost."); err != nil {
			return fail(errOut, err)
		}
	}
	if _, err := fmt.Fprintf(out, "Type %s to confirm, or press Enter to cancel: ", terminal.Style(out, terminal.Warning, strconv.Itoa(id.PID))); err != nil {
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
	if err := status(out, terminal.Success, "OK", fmt.Sprintf("%s sent to PID %d. The process may take time to exit.", action, id.PID)); err != nil {
		return fail(errOut, err)
	}
	return 0
}

func processError(pid int, err error) error {
	if os.IsNotExist(err) {
		return fmt.Errorf("process %d is no longer running; find a current PID with socklens list --listening", pid)
	}
	return err
}
