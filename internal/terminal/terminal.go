// Package terminal provides optional ANSI styling without external dependencies.
package terminal

import (
	"io"
	"os"
)

type Role string

const (
	Heading Role = "1;36"
	Success Role = "32"
	Warning Role = "33"
	Error   Role = "31"
	Info    Role = "36"
	Muted   Role = "90"
)

type writer struct {
	io.Writer
	color bool
}

// Wrap chooses color separately for each output stream. Explicit always overrides
// NO_COLOR and terminal detection; JSON callers must select never.
func Wrap(out io.Writer, mode string) io.Writer {
	enabled := mode == "always"
	if enabled {
		if file, ok := out.(*os.File); ok {
			_ = enableColor(file)
		}
	}
	if mode == "auto" && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" {
		if file, ok := out.(*os.File); ok {
			enabled = enableColor(file)
		}
	}
	return &writer{Writer: out, color: enabled}
}

func Style(out io.Writer, role Role, value string) string {
	if w, ok := out.(*writer); ok && w.color && value != "" {
		return "\x1b[" + string(role) + "m" + value + "\x1b[0m"
	}
	return value
}

func IsTerminal(out io.Writer) bool {
	file, ok := out.(*os.File)
	return ok && isTerminal(file)
}

// Width reports the visible terminal's columns. It is read on each render so
// watch output adapts after resizing. Redirected or unknown streams return zero.
func Width(out io.Writer) int {
	for {
		styled, ok := out.(*writer)
		if !ok {
			break
		}
		out = styled.Writer
	}
	file, ok := out.(*os.File)
	if !ok {
		return 0
	}
	return terminalWidth(file)
}
