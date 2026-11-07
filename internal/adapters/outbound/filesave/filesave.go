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
	"strconv"
	"strings"
	"time"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/atomicfile"
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
	path := filepath.Join(s.Dir, slot+Ext)
	if err := atomicfile.Write(path, 0o600, encode); err != nil {
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
// survives another window size or screen density). The top level is map
// 0's camera and Maps every other open map's by its decimal id (MAP-1), so
// a sidecar written before there were several maps loads as map 0's.
// Version is the zoom's unit (ViewVersion); only the top level carries it.
type ViewFile struct {
	CX, CY, Zoom float64
	Version      int                 `json:"v,omitempty"`
	Maps         map[string]ViewFile `json:"maps,omitempty"`
}

// ViewVersion is the sidecar's current zoom unit. 0 (no "v"): a multiple
// of a fixed 64 px tile. 1 (UIS-087): a multiple of the default zoom,
// whose cell is the window's height / 48. The old unit cannot be carried
// over (it depended on the window it was saved at), so an older sidecar
// loads at the default zoom with its centres kept (ViewFile.upgraded).
const ViewVersion = 1

// DefaultZoom is the zoom an older sidecar's cameras load at.
const DefaultZoom = 1.0

// upgraded is v in the current unit: an older sidecar's valid cameras at
// DefaultZoom, their centres kept (an invalid one stays invalid, so it is
// still left out).
func (v ViewFile) upgraded() ViewFile {
	if v.Version >= ViewVersion {
		return v
	}
	if v.valid() {
		v.Zoom = DefaultZoom
	}
	if v.Maps != nil {
		maps := make(map[string]ViewFile, len(v.Maps))
		for k, m := range v.Maps {
			if m.valid() {
				m.Zoom = DefaultZoom
			}
			maps[k] = m
		}
		v.Maps = maps
	}
	v.Version = ViewVersion
	return v
}

// valid reports the camera is finite with a positive zoom.
func (v ViewFile) valid() bool {
	fin := func(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }
	return fin(v.CX) && fin(v.CY) && fin(v.Zoom) && v.Zoom > 0
}

// ViewsOf is the sidecar for each map's camera: map 0 at the top level,
// the others under Maps.
func ViewsOf(views map[game.MapID]ViewFile) ViewFile {
	var out ViewFile
	for id, v := range views {
		v.Maps = nil
		if id == 0 {
			out.CX, out.CY, out.Zoom = v.CX, v.CY, v.Zoom
			continue
		}
		if out.Maps == nil {
			out.Maps = map[string]ViewFile{}
		}
		out.Maps[strconv.FormatUint(uint64(id), 10)] = v
	}
	return out
}

// PerMap is the sidecar's valid cameras by map id: map 0 from the top
// level, the others from Maps (an entry with a bad id or camera is left
// out).
func (v ViewFile) PerMap() map[game.MapID]ViewFile {
	out := map[game.MapID]ViewFile{}
	if v.valid() {
		out[0] = ViewFile{CX: v.CX, CY: v.CY, Zoom: v.Zoom}
	}
	for k, m := range v.Maps {
		id, err := strconv.ParseUint(k, 10, 32)
		if err != nil || !m.valid() {
			continue
		}
		out[game.MapID(id)] = ViewFile{CX: m.CX, CY: m.CY, Zoom: m.Zoom}
	}
	return out
}

func viewPath(savePath string) string {
	return strings.TrimSuffix(savePath, Ext) + ViewExt
}

// SaveView writes the camera sidecar for slot atomically (temp file and
// rename), replacing any earlier one, stamped with ViewVersion.
func (s Store) SaveView(slot string, v ViewFile) error {
	slot = sanitize(slot)
	v.Version = ViewVersion
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode view: %w", err)
	}
	if err := atomicfile.WriteBytes(filepath.Join(s.Dir, slot+ViewExt), 0o600, data); err != nil {
		return fmt.Errorf("store view: %w", err)
	}
	return nil
}

// LoadView reads the camera sidecar beside the save at savePath. A missing
// or unreadable file, or one without a single valid camera, reports false:
// the save loads as it always did. An older sidecar comes back in the
// current unit (upgraded). PerMap gives the cameras by map.
func (s Store) LoadView(savePath string) (ViewFile, bool) {
	data, err := os.ReadFile(viewPath(savePath))
	if err != nil {
		return ViewFile{}, false
	}
	var v ViewFile
	if json.Unmarshal(data, &v) != nil || len(v.PerMap()) == 0 {
		return ViewFile{}, false
	}
	return v.upgraded(), true
}
