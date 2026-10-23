// Package scenariofile keeps custom scenarios as JSON files in a folder,
// one file per scenario, written atomically (temp file + rename) so a
// crash never leaves a half-written scenario behind. The file content is
// opaque here: the scenario domain package encodes and decodes it.
package scenariofile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Ext is the scenario-file extension.
const Ext = ".json"

// maxName bounds a file name (without the extension).
const maxName = 64

// Store is a folder of scenario files.
type Store struct{ Dir string }

// File describes one scenario file.
type File struct {
	Name     string // file name without the extension
	Modified time.Time
}

// Default is the per-user scenarios folder (e.g. ~/Library/Application
// Support/rimworld-go/scenarios on macOS).
func Default() (Store, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return Store{}, fmt.Errorf("find config dir: %w", err)
	}
	return Store{Dir: filepath.Join(base, "rimworld-go", "scenarios")}, nil
}

// ValidName reports name is a plain slug: letters, digits, '-' and '_',
// at most 64 characters. Anything else (a path separator, "..") is refused
// so a name can never leave the folder.
func ValidName(name string) bool {
	if name == "" || len(name) > maxName {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func (s Store) path(name string) (string, error) {
	if !ValidName(name) {
		return "", fmt.Errorf("scenario file name %q is not allowed", name)
	}
	return filepath.Join(s.Dir, name+Ext), nil
}

// List returns the scenario files by name. A missing folder is an empty
// list.
func (s Store) List() ([]File, error) {
	ents, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read scenarios folder: %w", err)
	}
	var out []File
	for _, e := range ents {
		name, ok := strings.CutSuffix(e.Name(), Ext)
		if e.IsDir() || !ok || !ValidName(name) {
			continue
		}
		f := File{Name: name}
		if fi, err := e.Info(); err == nil {
			f.Modified = fi.ModTime()
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Load reads a scenario file.
func (s Store) Load(name string) ([]byte, error) {
	p, err := s.path(name)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read scenario: %w", err)
	}
	return b, nil
}

// Save writes data to the file name, replacing it.
func (s Store) Save(name string, data []byte) error {
	p, err := s.path(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("create scenarios folder: %w", err)
	}
	tmp, err := os.CreateTemp(s.Dir, name+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create scenario: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("write scenario: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("write scenario: %w", err)
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("store scenario: %w", err)
	}
	return nil
}

// Delete removes a scenario file; a file that is already gone is not an
// error.
func (s Store) Delete(name string) error {
	p, err := s.path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete scenario: %w", err)
	}
	return nil
}
