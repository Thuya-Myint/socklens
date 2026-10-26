//go:build !windows

package update

import "os"

func openStateFile(path string) (*os.File, error) { return os.Open(path) }
func lockContention(err error) bool               { return false }

func replaceStateFile(oldpath, newpath string) error { return os.Rename(oldpath, newpath) }
