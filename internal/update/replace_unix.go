//go:build !windows

package update

import (
	"io"
	"os"
)

func replaceExecutable(target, candidate, backup string) error {
	// Keep a rollback copy while the final rename atomically replaces the path.
	if err := os.Link(target, backup); err != nil {
		source, err := os.Open(target)
		if err != nil {
			return err
		}
		defer source.Close()
		info, err := source.Stat()
		if err != nil {
			return err
		}
		old, err := os.OpenFile(backup+".copy", os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		_, err = io.Copy(old, source)
		closeErr := old.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if err := os.Rename(backup+".copy", backup); err != nil {
			return err
		}
	}
	return os.Rename(candidate, target)
}
func restoreExecutable(target, backup string) error { return os.Rename(backup, target) }
