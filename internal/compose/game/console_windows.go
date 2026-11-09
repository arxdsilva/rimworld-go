package game

import (
	"io"
	"os"
	"syscall"
)

// attachParent is ATTACH_PARENT_PROCESS: the console of the process that
// started this one.
const attachParent = ^uintptr(0)

// ConsoleOut is where -version and -selfcheck print. The Windows build is
// a GUI program (-H=windowsgui) with no console of its own: stdout works
// when the caller redirected it (a pipe or a file, as the installer's
// smoke test does); otherwise the report goes to the console of the
// terminal that started the game, when there is one.
func ConsoleOut(stdout *os.File) io.Writer {
	if t, err := syscall.GetFileType(syscall.Handle(stdout.Fd())); err == nil && t != syscall.FILE_TYPE_UNKNOWN {
		return stdout
	}
	attach := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole")
	if attach.Find() != nil {
		return stdout
	}
	if ok, _, _ := attach.Call(attachParent); ok == 0 {
		return stdout
	}
	con, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return stdout
	}
	return con
}
