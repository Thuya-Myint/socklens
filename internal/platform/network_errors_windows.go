//go:build windows

package platform

import (
	"errors"
	"syscall"
)

// Winsock WSAECONNREFUSED differs from Go's compatibility ECONNREFUSED value.
// https://learn.microsoft.com/en-us/windows/win32/winsock/windows-sockets-error-codes-2
const wsaConnectionRefused syscall.Errno = 10061

// IsConnectionRefused recognizes wrapped native TCP connection refusal errors.
func IsConnectionRefused(err error) bool {
	return errors.Is(err, wsaConnectionRefused) || errors.Is(err, syscall.ECONNREFUSED)
}
