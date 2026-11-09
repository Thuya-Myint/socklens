//go:build darwin

package watchsetup

import "fmt"

func installPlan() ([]command, error) {
	return nil, fmt.Errorf("DTrace is a macOS system tool; no helper installation is offered")
}
