// Package filesave stores saved games as files in a saves folder: one
// file per slot, written atomically (temp file, fsync, rename) so a crash
// never leaves a half-written save behind.
package filesave

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
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
	return s.write(slot, func(w io.Writer) error { return g.Save(w, slot) })
}

// write replaces the slot's file atomically: the save goes to a temp file
// in the same folder, is flushed to disk (fsync), then renamed over the
// old file, and the folder is synced so the rename itself survives a
// crash. Any failure removes the temp file and leaves the old save as it
// was — a commitment colony's only save is never half-written (NG-1 §2.13).
func (s Store) write(slot string, encode func(io.Writer) error) (string, error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return "", fmt.Errorf("create saves folder: %w", err)
	}
	path := filepath.Join(s.Dir, slot+Ext)
	tmp, err := os.CreateTemp(s.Dir, slot+Ext+"-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create save: %w", err)
	}
	fail := func(err error) (string, error) {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := encode(tmp); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(fmt.Errorf("flush save: %w", err))
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("write save: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("store save: %w", err)
	}
	syncDir(s.Dir)
	return path, nil
}

// syncDir flushes a folder's entries (the rename) to disk. Best effort:
// some systems cannot sync a directory, and the save is complete anyway.
func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
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
			if sl.Err == nil && !game.SupportedSaveVersion(sl.Info.Version) {
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

// ViewExt is the extension of the camera sidecar written beside a save.
const ViewExt = ".rwview"

// ViewFile is the camera as the sidecar stores it: the world point at the
// middle of the screen in tiles, and the zoom (never pixel offsets, so it
// survives another window size or screen density).
type ViewFile struct {
	CX, CY, Zoom float64
}

func viewPath(savePath string) string {
	return strings.TrimSuffix(savePath, Ext) + ViewExt
}

// SaveView writes the camera sidecar for slot atomically (temp file and
// rename), replacing any earlier one.
func (s Store) SaveView(slot string, v ViewFile) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("create saves folder: %w", err)
	}
	slot = sanitize(slot)
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode view: %w", err)
	}
	tmp, err := os.CreateTemp(s.Dir, slot+ViewExt+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create view: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("write view: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("write view: %w", err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(s.Dir, slot+ViewExt)); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("store view: %w", err)
	}
	return nil
}

// LoadView reads the camera sidecar beside the save at savePath. A missing,
// unreadable or invalid file reports false: the save loads as it always did.
func (s Store) LoadView(savePath string) (ViewFile, bool) {
	data, err := os.ReadFile(viewPath(savePath))
	if err != nil {
		return ViewFile{}, false
	}
	var v ViewFile
	if json.Unmarshal(data, &v) != nil {
		return ViewFile{}, false
	}
	fin := func(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }
	if !fin(v.CX) || !fin(v.CY) || !fin(v.Zoom) || v.Zoom <= 0 {
		return ViewFile{}, false
	}
	return v, true
}
