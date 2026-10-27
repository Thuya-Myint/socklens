package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/Thuya-Myint/socklens/internal/terminal"
	"github.com/Thuya-Myint/socklens/internal/update"
)

func updateCommand(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, current string, updater update.Updater) int {
	fs := newCommandFlags("update", errOut)
	dryRun := fs.Bool("dry-run", false, "check and show the update method without installing")
	showStatus := fs.Bool("status", false, "show the latest Windows background update result")
	timeout := fs.Duration("timeout", update.DefaultInstallTimeout, "total update deadline (1s to 30m)")
	if err := parseCommandFlags(fs, args, errOut); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 || *timeout < time.Second || *timeout > 30*time.Minute {
		return fail(errOut, fmt.Errorf("usage: socklens update [--dry-run | --status] [--timeout 5m]; timeout must be 1s to 30m"))
	}
	if *showStatus {
		extra := false
		fs.Visit(func(f *flag.Flag) {
			if f.Name != "status" {
				extra = true
			}
		})
		if extra {
			return fail(errOut, fmt.Errorf("--status cannot be combined with update options"))
		}
		job, err := updater.InstallStatus()
		if err != nil {
			return fail(errOut, err)
		}
		switch job.Status {
		case "not-started":
			return reportStatus(out, errOut, terminal.Info, "INFO", "No background update is recorded for this installation. macOS/Linux updates run in the foreground.")
		case "completed":
			return reportStatus(out, errOut, terminal.Success, "OK", fmt.Sprintf("The background update completed: SockLens %s.", job.Latest))
		case "failed":
			return fail(errOut, fmt.Errorf("background update failed: %s\nDetails: %s", job.Error, job.LogPath))
		default:
			return reportStatus(out, errOut, terminal.Info, "INFO", fmt.Sprintf("Update to %s is %s. Log: %s", job.Latest, job.Status, job.LogPath))
		}
	}
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	if err := status(out, terminal.Info, "INFO", "Checking for a newer stable SockLens release..."); err != nil {
		return fail(errOut, err)
	}
	plan, err := updater.Check(ctx, current)
	if err != nil {
		return installFailure(ctx, errOut, err)
	}
	if !plan.Result.UpdateAvailable {
		if plan.Result.Status == "ahead" {
			return reportStatus(out, errOut, terminal.Info, "INFO", fmt.Sprintf("Installed %s is newer than the latest stable release %s.", plan.Result.Current, plan.Result.Latest))
		}
		return reportStatus(out, errOut, terminal.Success, "OK", fmt.Sprintf("SockLens %s is up to date.", plan.Result.Current))
	}
	if *dryRun {
		if err := status(out, terminal.Info, "DRY RUN", fmt.Sprintf("Would update %s to %s using %s. No files changed.", plan.Result.Current, plan.Result.Latest, plan.Method)); err != nil {
			return fail(errOut, err)
		}
		var details string
		switch plan.Method {
		case "homebrew":
			details = "brew update\nbrew upgrade --formula " + plan.Package + "\n"
		case "scoop":
			details = "scoop update\nscoop update socklens"
			if plan.Global {
				details += " --global"
			}
			details += "\n"
		default:
			details = "Download the matching GitHub release, verify SHA-256 and executable version, then replace: " + plan.Target + "\n"
		}
		if _, err := fmt.Fprint(out, details); err != nil {
			return fail(errOut, err)
		}
		return 0
	}
	if err := status(out, terminal.Info, "INFO", fmt.Sprintf("Updating SockLens to %s using %s...", plan.Result.Latest, plan.Method)); err != nil {
		return fail(errOut, err)
	}
	result, err := updater.Apply(ctx, plan, update.Streams{In: in, Out: out, Err: errOut})
	if err != nil {
		return installFailure(ctx, errOut, err)
	}
	if result.Status == "started" || result.Status == "in-progress" {
		return reportStatus(out, errOut, terminal.Info, "INFO", fmt.Sprintf("Windows update is %s in the background. Run socklens update --status for the result. Log: %s", result.Status, result.LogPath))
	}
	return reportStatus(out, errOut, terminal.Success, "OK", fmt.Sprintf("SockLens %s is installed (%s).", result.Version, result.Method))
}
func installFailure(ctx context.Context, out io.Writer, err error) int {
	if errors.Is(ctx.Err(), context.Canceled) {
		_ = status(out, terminal.Info, "CANCELLED", "Update interrupted; check the installed version before retrying.")
		return 130
	}
	return fail(out, err)
}
func reportStatus(out, errOut io.Writer, role terminal.Role, label, message string) int {
	if err := status(out, role, label, message); err != nil {
		return fail(errOut, err)
	}
	return 0
}
