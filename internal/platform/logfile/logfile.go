package logfile

import (
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

const (
	name     = "game.log"
	prevName = "game-prev.log"
)

// Open makes dir, moves the last run's game.log to game-prev.log and opens
// a fresh game.log for writing.
func Open(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	cur := filepath.Join(dir, name)
	if err := os.Rename(cur, filepath.Join(dir, prevName)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return os.OpenFile(cur, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
}

// Dir is the per-user logs folder ("" when there is no config folder).
func Dir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "rimworld-go", "logs")
}

// Start tees the standard logger to a fresh game.log in Dir, beside
// stderr, and returns the func that closes the file. Without a config
// folder it does nothing; when the file can't be opened it logs why and
// the run logs to stderr only.
func Start() (stop func()) {
	dir := Dir()
	if dir == "" {
		return func() {}
	}
	f, err := Open(dir)
	if err != nil {
		log.Printf("log file off this run: %v", err)
		return func() {}
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	return func() { f.Close() }
}
