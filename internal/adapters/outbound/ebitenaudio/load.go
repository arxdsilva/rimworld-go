package ebitenaudio

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"rimworld-go/internal/platform/assetroot"

	"rimworld-go/pkg/audiomanifest"
	"rimworld-go/pkg/sfxsynth"
)

// LocateAudio finds the "assets" directory holding the sound list
// (audiomanifest.Rel) above the working directory or the executable
// (assetroot) and returns it.
func LocateAudio() (string, error) {
	return assetroot.FindParent("assets", filepath.FromSlash(audiomanifest.Rel), false)
}

// load reads and validates the sound list under the assets root, then
// every file it names: WAVs decode to mono 48 kHz PCM, OGGs are kept as
// encoded bytes (one-shots decode to a cached PCM on first play; loops and music stream). A file that is missing, unreadable
// or at another rate is logged and skipped; only a bad sound list is an
// error. Variants keep their manifest order per kind.
func load(root string) (audiomanifest.Manifest, map[string][][]int16, map[string][][]byte, error) {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(audiomanifest.Rel)))
	if err != nil {
		return audiomanifest.Manifest{}, nil, nil, err
	}
	defer f.Close()
	m, err := audiomanifest.Decode(f)
	if err != nil {
		return m, nil, nil, err
	}
	pcm := map[string][][]int16{}
	ogg := map[string][][]byte{}
	dir := filepath.Join(root, filepath.FromSlash(audiomanifest.Dir))
	for _, e := range m.Sounds {
		for _, name := range e.Files {
			path := filepath.Join(dir, filepath.FromSlash(name))
			switch strings.ToLower(filepath.Ext(name)) {
			case ".wav":
				samples, err := readWAV(path)
				if err != nil {
					log.Printf("sound %s: %v", e.ID, err)
					continue
				}
				pcm[e.ID] = append(pcm[e.ID], samples)
			case ".ogg":
				data, err := os.ReadFile(path)
				if err == nil {
					_, _, err = decodeOGG(data)
				}
				if err != nil {
					log.Printf("sound %s: %s: %v", e.ID, name, err)
					continue
				}
				ogg[e.ID] = append(ogg[e.ID], data)
			default:
				log.Printf("sound %s: %s: not a .wav or .ogg file", e.ID, name)
			}
		}
	}
	return m, pcm, ogg, nil
}

func readWAV(path string) ([]int16, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	samples, rate, _, err := sfxsynth.ReadWAV(bytes.NewReader(data))
	switch {
	case err != nil:
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	case rate != sampleRate:
		return nil, fmt.Errorf("%s: %d Hz, want %d", filepath.Base(path), rate, sampleRate)
	case len(samples) == 0:
		return nil, fmt.Errorf("%s: no samples", filepath.Base(path))
	}
	return samples, nil
}
