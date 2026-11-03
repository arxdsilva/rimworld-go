// Package settingsfile stores the player's settings as one JSON file next
// to the saves folder, written atomically (temp file + rename).
package settingsfile

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"rimworld-go/internal/platform/atomicfile"
	"rimworld-go/internal/platform/config"
)

// Store is a settings file.
type Store struct{ Path string }

// Default is the per-user settings file (e.g. ~/Library/Application
// Support/rimworld-go/settings.json on macOS).
func Default() (Store, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return Store{}, fmt.Errorf("find config dir: %w", err)
	}
	return Store{Path: filepath.Join(base, "rimworld-go", "settings.json")}, nil
}

// Load reads the settings. A missing file is a fresh install (defaults,
// no error). An unreadable file is moved aside to <path>.bad so it is
// never silently overwritten, and the defaults come back with the error.
func (s Store) Load() (config.Settings, error) {
	f, err := os.Open(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return config.Defaults(), nil
	}
	if err != nil {
		return config.Defaults(), fmt.Errorf("open settings: %w", err)
	}
	set, err := config.Decode(f)
	f.Close()
	if err != nil {
		if rerr := os.Rename(s.Path, s.Path+".bad"); rerr != nil {
			return config.Defaults(), fmt.Errorf("%w (and could not set it aside: %v)", err, rerr)
		}
		return config.Defaults(), fmt.Errorf("%w; reset to defaults, old file kept as %s.bad", err, filepath.Base(s.Path))
	}
	return set, nil
}

// Save writes the settings, replacing the file atomically.
func (s Store) Save(set config.Settings) error {
	err := atomicfile.Write(s.Path, 0o600, func(w io.Writer) error { return config.Encode(w, set) })
	if err != nil {
		return fmt.Errorf("store settings: %w", err)
	}
	return nil
}
