package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Thuya-Myint/socklens/internal/cli"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func versionInfo() string {
	return fmt.Sprintf("%s (commit %s, built %s)", version, commit, buildDate)
}

func main() { os.Exit(run()) }

func run() int {
	ctx := context.Background()
	if len(os.Args) > 1 && (os.Args[1] == "watch" || os.Args[1] == "wait") {
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer stop()
	}
	return cli.RunContext(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, versionInfo())
}
