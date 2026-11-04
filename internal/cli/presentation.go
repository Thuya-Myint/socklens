package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Thuya-Myint/socklens/internal/terminal"
)

// ParseColorArgs accepts presentation flags before the command or among its
// options. It preserves flag values, --, and the positional-argument boundary.
// Main also uses the normalized args for cancellation and update eligibility.
func ParseColorArgs(args []string) ([]string, string, error) {
	result := make([]string, 0, len(args))
	mode := "auto"
	command := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || (command && !strings.HasPrefix(arg, "-")) {
			result = append(result, args[i:]...)
			break
		}
		name, value, equals := strings.Cut(arg, "=")
		if name == "--color" {
			if !equals {
				i++
				if i == len(args) {
					return nil, "", fmt.Errorf("--color requires auto, always, or never")
				}
				value = args[i]
			}
			if value != "auto" && value != "always" && value != "never" {
				return nil, "", fmt.Errorf("--color must be auto, always, or never")
			}
			mode = value
			continue
		}
		result = append(result, arg)
		if !command {
			command = true
			continue
		}
		if !equals && takesValue(name) && i+1 < len(args) {
			i++
			result = append(result, args[i])
		}
	}
	return result, mode, nil
}
func takesValue(name string) bool {
	switch strings.TrimLeft(name, "-") {
	case "port", "pid", "protocol", "interval", "count", "until", "timeout", "peer", "firewall-timeout", "rule-limit", "host", "ports", "family", "connect-timeout", "concurrency":
		return true
	}
	return false
}
func jsonRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--json" || arg == "-json" || strings.HasPrefix(arg, "--json=") || strings.HasPrefix(arg, "-json=") {
			return true
		}
	}
	return false
}

func status(out io.Writer, role terminal.Role, label, message string) error {
	_, err := fmt.Fprintf(out, "%s %s\n", terminal.Style(out, role, label), message)
	return err
}

func newCommandFlags(command string, out io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(out)
	fs.Usage = func() {
		suffix := "[options]"
		if command == "inspect" || command == "wait" {
			suffix += " PORT"
		}
		if command == "terminate" {
			suffix = "--pid PID [options]"
		}
		fmt.Fprintf(out, "%s socklens %s %s\n\n", terminal.Style(out, terminal.Heading, "Usage:"), command, suffix)
		// Positional-port commands share parsing with list/watch, but do not
		// accept --port. Keep their help limited to usable options.
		visible := flag.NewFlagSet(command, flag.ContinueOnError)
		visible.SetOutput(out)
		fs.VisitAll(func(option *flag.Flag) {
			if (command == "inspect" || command == "wait") && option.Name == "port" {
				return
			}
			visible.Var(option.Value, option.Name, option.Usage)
			visible.Lookup(option.Name).DefValue = option.DefValue
		})
		visible.PrintDefaults()
		fmt.Fprintln(out, "  --color auto|always|never\n        terminal colors (default auto)")
		if command == "probe" {
			fmt.Fprintln(out, `
Explicit TCP connection checks. One host is resolved once; each selected numeric
IP/port pair is tested separately. No application payloads or UDP probes.
Limits: 64 distinct ports, 16 selected addresses, 256 total attempts.
--connect-timeout bounds each attempt; --timeout includes DNS and queued work.

Examples:
  socklens probe --host 127.0.0.1 --ports 8080
  socklens probe --host localhost --ports 80,443 --family ipv4
  socklens probe --host 192.168.1.20 --ports 80,443 --connect-timeout 2s --timeout 10s
  socklens probe --host ::1 --ports 8080 --json

Results: connected, refused, timed-out, network-error, cancelled, not-attempted.
A timeout does not establish a closed port or identify a firewall.
Ctrl+C returns a partial report. No background update check is scheduled.
Exit: 0 all connected; 2 unsuccessful/incomplete connections or total deadline;
1 invalid input, DNS/target planning, or output failure; 130 interrupted.`)
		}
		if command == "audit" {
			fmt.Fprintln(out, `
Explains local TCP listeners and bound/unconnected UDP sockets.
Filters combine: --port PORT, --protocol tcp|udp, --pid PID.
Read-only firewall context is included by default; --firewall=false skips it.
Concise text shows inbound protocol/binding scope and outbound peer visibility.
Default rule evidence is endpoint/process-related; --details includes broad and
identity-unresolved entries, full evidence and restrictions.
Windows shows hosted service names when available; a service host can be shared.
JSON always includes the complete structured report, regardless of --details.
--peer filters candidates by numeric IP; it does not send traffic.
No probes, elevation, firewall changes, or background update checks run during audit.
Review items are configuration questions, not confirmed vulnerabilities.
Reading order: non-loopback service-port hints, other review items,
other specific bindings, then loopback. Hints are unverified, not a severity score.

Examples:
  socklens audit
  socklens audit --port 8080
  socklens audit --protocol tcp --json
  socklens audit --pid 12345
  socklens audit --port 5432 --peer 192.168.1.20
  socklens audit --rule-limit 0
  socklens audit --port 8080 --details

Exit 0 means the audit completed, including when review items exist.
External reachability, authentication, TLS, and vulnerabilities are not tested.`)
		}
		if command == "wait" {
			fmt.Fprintln(out, `
Conditions:
  present  Succeed when at least one socket matches (default).
  absent   Succeed when no matching sockets remain.
  Both check immediately and succeed if the condition already holds.
  Otherwise, wait until it matches, the timeout expires, or you press Ctrl+C.

Examples:
  socklens wait --until present --listening --protocol tcp --timeout 30s 8080
  socklens wait --until absent --listening --protocol tcp --timeout 30s 8080

With --listening --protocol tcp, present means starts listening;
absent means stops listening. Put all options before the final PORT.`)
		}
	}
	return fs
}
func parseCommandFlags(fs *flag.FlagSet, args []string, errOut io.Writer) error {
	usage := fs.Usage
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	err := fs.Parse(args)
	fs.SetOutput(errOut)
	fs.Usage = usage
	if err == flag.ErrHelp {
		fs.Usage()
	} else if err != nil {
		fail(errOut, fmt.Errorf("%v\nTry: socklens %s --help", err, fs.Name()))
	}
	return err
}

func writeHelp(out io.Writer) error {
	for _, line := range strings.SplitAfter(help, "\n") {
		value := strings.TrimSuffix(line, "\n")
		if strings.HasPrefix(value, "SockLens") || strings.HasSuffix(value, ":") {
			value = terminal.Style(out, terminal.Heading, value)
		}
		newline := ""
		if strings.HasSuffix(line, "\n") {
			newline = "\n"
		}
		if _, err := fmt.Fprint(out, value+newline); err != nil {
			return err
		}
	}
	return nil
}
