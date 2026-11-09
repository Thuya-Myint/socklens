//go:build !linux && !darwin && !windows

package watchsetup

import (
	"context"
	"fmt"
)

func canElevate() (bool, error) { return false, nil }
func elevateWatch(context.Context, []string) error {
	return fmt.Errorf("watch elevation is unsupported")
}
func installPlan() ([]command, error) {
	return nil, fmt.Errorf("automatic tracing setup is unsupported")
}
