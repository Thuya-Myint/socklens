package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Thuya-Myint/socklens/internal/cli"
	"github.com/Thuya-Myint/socklens/internal/terminal"
	"github.com/Thuya-Myint/socklens/internal/update"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func versionInfo() string {
	return version
}

func main() { os.Exit(run()) }

func run() int {
	ctx := context.Background()
	if len(os.Args) > 1 && os.Args[1] == "__install-update" {
		if len(os.Args) != 3 {
			return 1
		}
		if err := update.RunInstallWorker(ctx, os.Args[2]); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "socklens update:", err)
			return 1
		}
		return 0
	}
	if len(os.Args) > 1 && os.Args[1] == "__update-refresh" {
		if len(os.Args) != 3 {
			return 0
		}
		if service, err := update.DefaultService(); err == nil {
			_ = service.RunBackground(ctx, versionInfo(), os.Args[2])
		}
		return 0
	}
	args, mode, parseErr := cli.ParseColorArgs(os.Args[1:])
	if parseErr != nil {
		return cli.RunContext(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, versionInfo())
	}
	if shouldAutoNotify(args, isTerminal(os.Stdout) && isTerminal(os.Stderr)) {
		if service, err := update.DefaultService(); err == nil {
			message, nonce, err := service.PrepareAutomatic(versionInfo())
			if err == nil {
				if message != "" {
					_, _ = fmt.Fprint(os.Stderr, terminal.Style(terminal.Wrap(os.Stderr, mode), terminal.Warning, "UPDATE")+" "+message)
				}
				if nonce != "" {
					executable, err := os.Executable()
					if err == nil {
						err = startUpdateWorker(executable, nonce)
					}
					if err != nil {
						service.CancelPending(nonce)
					}
				}
			}
		}
	}
	if len(args) > 0 && (args[0] == "watch" || args[0] == "wait" || args[0] == "update") {
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer stop()
	}
	return cli.RunContext(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, versionInfo())
}

func isTerminal(file *os.File) bool {
	return terminal.IsTerminal(file)
}
func shouldAutoNotify(args []string, terminal bool) bool {
	if !terminal || len(args) == 0 {
		return false
	}
	switch args[0] {
	case "list", "inspect", "check", "watch", "wait":
	default:
		return false
	}
	for _, arg := range args[1:] {
		if arg == "--json" || arg == "-json" || strings.HasPrefix(arg, "--json=") || strings.HasPrefix(arg, "-json=") || arg == "--help" || arg == "-h" {
			return false
		}
	}
	return true
}
func startUpdateWorker(executable, nonce string) error {
	command := exec.Command(executable, "__update-refresh", nonce)
	configureUpdateWorker(command)
	if err := command.Start(); err != nil {
		return err
	}
	go func() { _ = command.Wait() }()
	return nil
}
