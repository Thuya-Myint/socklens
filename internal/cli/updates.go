package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Thuya-Myint/socklens/internal/terminal"
	"github.com/Thuya-Myint/socklens/internal/update"
)

func updateCheckCommand(ctx context.Context, args []string, out, errOut io.Writer, version string, checker update.Checker) int {
	fs := newCommandFlags("update-check", errOut)
	asJSON := fs.Bool("json", false, "JSON update result")
	timeout := fs.Duration("timeout", 3*time.Second, "network timeout (100ms to 30s)")
	if err := parseCommandFlags(fs, args, errOut); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 {
		return fail(errOut, fmt.Errorf("update-check takes no positional arguments"))
	}
	if *timeout < 100*time.Millisecond || *timeout > 30*time.Second {
		return fail(errOut, fmt.Errorf("timeout must be between 100ms and 30s"))
	}
	client := http.Client{Timeout: *timeout}
	if checker.Client != nil {
		client = *checker.Client
		client.Timeout = *timeout
	}
	checker.Client = &client
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	result, err := checker.Check(ctx, version)
	if err != nil {
		return fail(errOut, err)
	}
	if *asJSON {
		if err := json.NewEncoder(out).Encode(result); err != nil {
			return fail(errOut, err)
		}
		return 0
	}
	var message string
	switch result.Status {
	case "update-available":
		message = terminal.Style(out, terminal.Warning, "UPDATE") + " " + update.Notice(result)
	case "ahead":
		message = terminal.Style(out, terminal.Info, "INFO") + " " + fmt.Sprintf("Installed %s is newer than the latest stable release %s.\n", result.Current, result.Latest)
	default:
		message = terminal.Style(out, terminal.Success, "OK") + " " + fmt.Sprintf("SockLens %s is up to date.\n", result.Current)
	}
	if _, err := fmt.Fprint(out, message); err != nil {
		return fail(errOut, err)
	}
	return 0
}

func updatesCommand(args []string, out, errOut io.Writer, service *update.Service) int {
	fs := newCommandFlags("updates", errOut)
	enable := fs.Bool("enable", false, "enable daily background update notices")
	disable := fs.Bool("disable", false, "disable background update notices")
	fs.Bool("status", false, "show whether daily notices are enabled")
	if err := parseCommandFlags(fs, args, errOut); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	selected := 0
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "enable" || f.Name == "disable" || f.Name == "status" {
			selected++
		}
	})
	if selected > 1 || fs.NArg() != 0 {
		return fail(errOut, fmt.Errorf("usage: socklens updates [--enable | --disable | --status]"))
	}
	if *enable || *disable {
		if err := service.SetEnabled(*enable); err != nil {
			return fail(errOut, err)
		}
	}
	state, err := service.Read()
	if err != nil {
		return fail(errOut, err)
	}
	message := "Daily update notices: disabled.\n"
	if state.Enabled {
		message = "Daily update notices: enabled (at most once every 24 hours).\n"
	}
	if err := status(out, terminal.Info, "INFO", strings.TrimSuffix(message, "\n")); err != nil {
		return fail(errOut, err)
	}
	return 0
}
