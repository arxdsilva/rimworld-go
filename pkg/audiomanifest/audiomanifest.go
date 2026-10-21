// Package audiomanifest is the game's sound list
// (assets/metadata/audio/audio.json): every sound id with its files, mix
// bus, loudness and provenance. cmd/audiogen writes it and the audio
// adapter reads it.
package audiomanifest

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Rel is the manifest path relative to the assets root.
const Rel = "metadata/audio/audio.json"

// Dir is where sound files live, relative to the assets root; entry
// files are relative to it.
const Dir = "generated/audio"

// Buses.
const (
	BusMusic    = "music"
	BusEffects  = "effects"
	BusAmbience = "ambience"
	BusUI       = "ui"
)

// Sources.
const (
	SourceProcedural = "procedural" // effects: cmd/audiogen recipes
	SourceComposed   = "composed"   // music: cmd/audiogen -music scores
	SourceElevenLabs = "elevenlabs"
)

// Entry is one sound.
type Entry struct {
	ID     string   `json:"id"`
	Bus    string   `json:"bus"`
	Loop   bool     `json:"loop,omitempty"`
	Volume float64  `json:"volume"`
	Files  []string `json:"files"` // variants; one is picked per play

	Source  string  `json:"source"`
	Recipe  string  `json:"recipe,omitempty"`
	Seed    uint64  `json:"seed,omitempty"`
	Tool    string  `json:"tool,omitempty"`
	Prompt  string  `json:"prompt,omitempty"`
	Seconds float64 `json:"seconds,omitempty"`
	Credits int     `json:"credits,omitempty"`
	Date    string  `json:"generated,omitempty"`
	License string  `json:"license"`
	Review  string  `json:"reviewed_by,omitempty"`
}

// Manifest is the sound list.
type Manifest struct {
	Version int     `json:"version"`
	Sounds  []Entry `json:"sounds"`
}

// Decode reads and validates a manifest.
func Decode(r io.Reader) (Manifest, error) {
	var m Manifest
	if err := json.NewDecoder(r).Decode(&m); err != nil {
		return m, fmt.Errorf("read sound list: %w", err)
	}
	return m, m.Validate()
}

// Encode writes the manifest sorted by id, indented.
func Encode(w io.Writer, m Manifest) error {
	m.Version = 1
	sort.Slice(m.Sounds, func(i, j int) bool { return m.Sounds[i].ID < m.Sounds[j].ID })
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(m)
}

// Validate checks every entry has an id, a known bus, files, a source and
// a license, and that ids are unique.
func (m Manifest) Validate() error {
	seen := map[string]bool{}
	for _, e := range m.Sounds {
		switch {
		case e.ID == "":
			return fmt.Errorf("sound without an id")
		case seen[e.ID]:
			return fmt.Errorf("%s: listed twice", e.ID)
		case e.Bus != BusMusic && e.Bus != BusEffects && e.Bus != BusAmbience && e.Bus != BusUI:
			return fmt.Errorf("%s: unknown bus %q", e.ID, e.Bus)
		case len(e.Files) == 0:
			return fmt.Errorf("%s: no files", e.ID)
		case e.Source == "" || e.License == "":
			return fmt.Errorf("%s: missing source or license", e.ID)
		case e.Volume <= 0 || e.Volume > 1:
			return fmt.Errorf("%s: volume %v outside (0, 1]", e.ID, e.Volume)
		}
		for _, f := range e.Files {
			if strings.Contains(f, "..") || strings.HasPrefix(f, "/") {
				return fmt.Errorf("%s: file %q escapes %s", e.ID, f, Dir)
			}
		}
		seen[e.ID] = true
	}
	return nil
}

// Lookup returns the entry for id.
func (m Manifest) Lookup(id string) (Entry, bool) {
	for _, e := range m.Sounds {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// BusOf is the default bus of an id by its prefix (ui., sfx., amb., music.).
func BusOf(id string) string {
	switch {
	case strings.HasPrefix(id, "ui."):
		return BusUI
	case strings.HasPrefix(id, "amb."):
		return BusAmbience
	case strings.HasPrefix(id, "music."):
		return BusMusic
	}
	return BusEffects
}
