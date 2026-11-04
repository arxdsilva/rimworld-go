// Package cardpresetfile keeps the colonist card's presets as one JSON file
// each in a folder beside the settings file.
package cardpresetfile

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"rimworld-go/internal/platform/atomicfile"
)

// Limits. MaxBytes matches sheet.MaxPresetBytes; MaxName is a file stem.
const (
	MaxBytes = 256 << 10
	MaxName  = 64
	ext      = ".json"
)

// ErrName is returned for a name that cannot be a file in the folder.
var ErrName = errors.New("cardpresetfile: bad preset name")

// Store is a folder of preset files.
type Store struct{ Dir string }

// Default is <user config dir>/rimworld-go/dossier.
func Default() (Store, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return Store{}, fmt.Errorf("find config dir: %w", err)
	}
	return Store{Dir: filepath.Join(base, "rimworld-go", "dossier")}, nil
}

// checkName accepts only letters, digits, '-' and '_' (1..MaxName): no path
// separator, dot, space or empty name can name a file outside Dir.
func checkName(name string) error {
	if name == "" || len(name) > MaxName {
		return fmt.Errorf("%w: use 1 to %d characters", ErrName, MaxName)
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return fmt.Errorf("%w: %q", ErrName, name)
		}
	}
	return nil
}

func (s Store) path(name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", err
	}
	return filepath.Join(s.Dir, name+ext), nil
}

// List names the presets in the folder, sorted. A missing folder is empty.
func (s Store) List() ([]string, error) {
	es, err := os.ReadDir(s.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list presets: %w", err)
	}
	var out []string
	for _, e := range es {
		stem, ok := strings.CutSuffix(e.Name(), ext)
		if !ok || e.IsDir() || checkName(stem) != nil {
			continue
		}
		out = append(out, stem)
	}
	sort.Strings(out)
	return out, nil
}

// Load reads a preset file's bytes; a file over MaxBytes is refused.
func (s Store) Load(name string) ([]byte, error) {
	p, err := s.path(name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("open preset: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read preset: %w", err)
	}
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("read preset: %w", fs.ErrInvalid)
	}
	return data, nil
}

// Save writes a preset atomically, creating the folder.
func (s Store) Save(name string, data []byte) error {
	p, err := s.path(name)
	if err != nil {
		return err
	}
	if len(data) > MaxBytes {
		return fmt.Errorf("store preset: too big: %w", fs.ErrInvalid)
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("store preset: %w", err)
	}
	if err := atomicfile.WriteBytes(p, 0o600, data); err != nil {
		return fmt.Errorf("store preset: %w", err)
	}
	return nil
}

// Delete removes a preset file; a missing one is not an error.
func (s Store) Delete(name string) error {
	p, err := s.path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete preset: %w", err)
	}
	return nil
}
