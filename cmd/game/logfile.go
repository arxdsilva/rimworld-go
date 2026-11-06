package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Log file (OPT-003): the run's log is copied to <config>/rimworld-go/logs,
// the folder Options › General › Open log folder shows; the last run's log
// stays beside it as game-prev.log.

const (
	logName     = "game.log"
	logPrevName = "game-prev.log"
)

// openLogFile makes dir, moves the last run's game.log to game-prev.log and
// opens a fresh game.log for writing.
func openLogFile(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	cur := filepath.Join(dir, logName)
	if err := os.Rename(cur, filepath.Join(dir, logPrevName)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return os.OpenFile(cur, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
}

// logDir is the per-user logs folder ("" when there is no config folder).
func logDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "rimworld-go", "logs")
}
