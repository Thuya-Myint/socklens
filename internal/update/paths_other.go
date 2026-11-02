//go:build !windows

package update

import "path/filepath"

func resolveInstallPath(path string) (string, error) { return filepath.EvalSymlinks(path) }
