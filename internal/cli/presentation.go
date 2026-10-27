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
	case "port", "pid", "protocol", "interval", "count", "until", "timeout":
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
		fs.PrintDefaults()
		fmt.Fprintln(out, "  --color auto|always|never\n        terminal colors (default auto)")
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
