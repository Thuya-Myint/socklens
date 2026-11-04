//go:build windows

package platform

import (
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
)

func TestWindowsConnectionRefusalCodes(t *testing.T) {
	for _, code := range []syscall.Errno{10061, syscall.ECONNREFUSED} {
		err := &net.OpError{Op: "dial", Net: "tcp4", Err: os.NewSyscallError("connectex", code)}
		if !IsConnectionRefused(err) {
			t.Fatalf("native refusal lost through network/syscall wrappers: %v", err)
		}
	}
	for _, err := range []error{nil, syscall.Errno(10060), syscall.EACCES, errors.New("connection refused")} {
		if IsConnectionRefused(err) {
			t.Fatalf("non-refusal was classified from unrelated code or text: %v", err)
		}
	}
}
