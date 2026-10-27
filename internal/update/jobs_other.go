//go:build !windows

package update

import (
	"context"
	"fmt"
)

func queueInstall(ctx context.Context, u Updater, plan Plan) (InstallResult, error) {
	return InstallResult{}, fmt.Errorf("background installer handoff is only used on Windows")
}
func RunInstallWorker(ctx context.Context, nonce string) error {
	return fmt.Errorf("background installer handoff is only used on Windows")
}
