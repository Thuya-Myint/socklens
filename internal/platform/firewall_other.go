//go:build !linux && !windows && !darwin

package platform

import (
	"context"
	"github.com/Thuya-Myint/socklens/internal/firewall"
)

func CollectFirewall(ctx context.Context, rows []Socket) firewall.Snapshot {
	s := policyBase()
	s.Notes = append(s.Notes, "Firewall collection is unsupported on this operating system.")
	return s
}
