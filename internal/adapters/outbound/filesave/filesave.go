// Package filesave stores saved games as files in a saves folder: one
// file per slot, written atomically (temp file + rename) so a crash never
// leaves a half-written save behind.
package filesave

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"rimworld-go/internal/application/game"
)

// Ext is the save-file extension.
const Ext = ".rwsave"

// Store is a folder of saves.
type Store struct{ Dir string }

// Default is the per-user saves folder (e.g. ~/Library/Application
// Support/rimworld-go/saves on macOS).
func Default() (Store, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return Store{}, fmt.Errorf("find config dir: %w", err)
	}
	return Store{Dir: filepath.Join(base, "rimworld-go", "saves")}, nil
}

// Slot describes one save file.
type Slot struct {
	Name     string // slot name (file name without extension)
	Path     string
	Modified time.Time
	Info     game.SaveInfo
	Err      error // unreadable / incompatible: listed but not loadable
}

// sanitize makes a slot name safe as a file name.
func sanitize(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "colony"
	}
	return b.String()
}

// Save writes g to the slot (replacing it) and returns the file path.
func (s Store) Save(g *game.Game, slot string) (string, error) {
	slot = sanitize(slot)
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return "", fmt.Errorf("create saves folder: %w", err)
	}
	path := filepath.Join(s.Dir, slot+Ext)
	tmp, err := os.CreateTemp(s.Dir, slot+"-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create save: %w", err)
	}
	if err := g.Save(tmp, slot); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("write save: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("store save: %w", err)
	}
	return path, nil
}

// List returns every save, newest first. Unreadable files are listed with
// Err set.
func (s Store) List() ([]Slot, error) {
	ents, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read saves folder: %w", err)
	}
	var out []Slot
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), Ext) {
			continue
		}
		sl := Slot{Name: strings.TrimSuffix(e.Name(), Ext), Path: filepath.Join(s.Dir, e.Name())}
		if fi, err := e.Info(); err == nil {
			sl.Modified = fi.ModTime()
		}
		f, err := os.Open(sl.Path)
		if err != nil {
			sl.Err = err
		} else {
			sl.Info, sl.Err = game.ReadSaveInfo(f)
			f.Close()
			if sl.Err == nil && sl.Info.Version != game.SaveVersion {
				sl.Err = game.ErrSaveVersion
			}
		}
		out = append(out, sl)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
	return out, nil
}

// Load reads a save file.
func (s Store) Load(path string) (*game.Game, game.SaveInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, game.SaveInfo{}, fmt.Errorf("open save: %w", err)
	}
	defer f.Close()
	return game.Load(f)
}
