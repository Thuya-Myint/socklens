//go:build !windows

package platform

import "context"

// Windows service metadata is optional; other OSes retain process observations.
func CollectServices(ctx context.Context, rows []Socket) *ServiceSnapshot { return nil }
