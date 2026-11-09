//go:build !windows

package game

import (
	"io"
	"os"
)

// ConsoleOut is where -version and -selfcheck print: stdout (only the
// Windows GUI build needs to find a console, console_windows.go).
func ConsoleOut(stdout *os.File) io.Writer { return stdout }
