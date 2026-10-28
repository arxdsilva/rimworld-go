package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/pkg/audiomanifest"
)

// voiceSource is the renderer file that owns the species -> voice group
// table, relative to the repo root (the parent of the assets directory).
const voiceSource = "internal/adapters/inbound/ebiten/render/cue/animal_sounds.go"

// voiceEvents are the four sounds every species has (SND-003).
var voiceEvents = []string{"call", "attack", "hurt", "die"}

var voicePair = regexp.MustCompile(`"([a-z_]+)":\s*"([a-z_]+)"`)

// parseVoiceGroups reads the VoiceGroups map literal out of the renderer
// source, so the check cannot drift from the table the game uses.
func parseVoiceGroups(src string) (map[string]string, error) {
	i := strings.Index(src, "VoiceGroups = map[string]string{")
	if i < 0 {
		return nil, errors.New("VoiceGroups table not found")
	}
	body := src[i:]
	if j := strings.Index(body, "\n}"); j >= 0 {
		body = body[:j]
	}
	out := map[string]string{}
	for _, m := range voicePair.FindAllStringSubmatch(body, -1) {
		out[m[1]] = m[2]
	}
	if len(out) == 0 {
		return nil, errors.New("VoiceGroups table is empty")
	}
	return out, nil
}

// checkAnimalCoverage wants every roster species' four events to resolve to
// a listed sound through the fallback chain animal.<key>.<event>, then
// animal.<group>.<event>, then animal.generic.<event>, and every group
// entry to name a real species.
func checkAnimalCoverage(m audiomanifest.Manifest, groups map[string]string) error {
	keys := map[string]bool{}
	for _, s := range animal.All() {
		keys[s.String()] = true
	}
	for k := range groups {
		if !keys[k] {
			return fmt.Errorf("voiceGroups lists %q, which is not in the animal roster", k)
		}
	}
	for _, s := range animal.All() {
		key := s.String()
		group := "generic"
		if g, ok := groups[key]; ok {
			group = g
		}
		for _, ev := range voiceEvents {
			ok := false
			for _, id := range []string{"animal." + key + "." + ev, "animal." + group + "." + ev, "animal.generic." + ev} {
				if e, found := m.Lookup(id); found && len(e.Files) > 0 {
					ok = true
					break
				}
			}
			if !ok {
				return fmt.Errorf("animal.%s.%s resolves to no sound (tried the species, group %q and generic)", key, ev, group)
			}
		}
	}
	return nil
}

// checkAnimals runs the roster coverage check for the repo that holds
// assets; with no renderer source beside it (a scratch assets dir) there
// is no table to check against and it is skipped.
func checkAnimals(assets string, m audiomanifest.Manifest) error {
	src, err := os.ReadFile(filepath.Join(assets, "..", filepath.FromSlash(voiceSource)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	groups, err := parseVoiceGroups(string(src))
	if err != nil {
		return fmt.Errorf("%s: %w", voiceSource, err)
	}
	return checkAnimalCoverage(m, groups)
}
