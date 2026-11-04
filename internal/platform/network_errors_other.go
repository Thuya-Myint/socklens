//go:build !windows

package platform

import (
	"errors"
	"syscall"
)

// IsConnectionRefused recognizes wrapped native TCP connection refusal errors.
func IsConnectionRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED)
}
