package main

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"

	"rimworld-go/pkg/audiomanifest"
	"rimworld-go/pkg/sfxsynth"
)

func TestGenerateIsDeterministicAndKeepsOtherSources(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	music := audiomanifest.Entry{ID: "music.calm_1", Bus: audiomanifest.BusMusic, Volume: 0.8,
		Files: []string{"music/calm_1.ogg"}, Source: audiomanifest.SourceElevenLabs, License: "x"}
	if err := writeManifest(dir, audiomanifest.Manifest{Sounds: []audiomanifest.Entry{music}}); err != nil {
		t.Fatal(err)
	}
	if _, err := generate(dir); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, "generated/audio/sfx/chop_0.wav"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := generate(dir); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, "generated/audio/sfx/chop_0.wav"))
	if !bytes.Equal(first, second) {
		t.Fatal("chop_0.wav differs between runs")
	}
	m, err := readManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Lookup("music.calm_1"); !ok {
		t.Fatal("ElevenLabs entry dropped")
	}
	if len(m.Sounds) != len(sounds)+1 {
		t.Fatalf("%d sounds listed, want %d", len(m.Sounds), len(sounds)+1)
	}
}

func TestFileStem(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]string{"ui.click": "ui/click", "sfx.chop": "sfx/chop", "amb.campfire": "ambience/campfire"} {
		if got := fileStem(id); got != want {
			t.Fatalf("fileStem(%s) = %s, want %s", id, got, want)
		}
	}
}

func TestEveryRecipeRendersAudibly(t *testing.T) {
	t.Parallel()
	for _, s := range sounds {
		for v := 0; v < max(s.recipe.Variants, 1); v++ {
			x := sfxsynth.Render(s.recipe, v)
			sum := 0.0
			for _, f := range x {
				if math.IsNaN(f) || math.IsInf(f, 0) {
					t.Fatalf("%s v%d: non-finite sample", s.recipe.ID, v)
				}
				sum += f * f
			}
			if rms := math.Sqrt(sum / float64(len(x))); rms < 0.01 {
				t.Fatalf("%s v%d: nearly silent (rms %.4f)", s.recipe.ID, v, rms)
			}
		}
	}
}

func TestEveryRecipeHasAUniqueIDAndVolume(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, s := range sounds {
		if seen[s.recipe.ID] || s.volume <= 0 || s.volume > 1 || len(s.recipe.Layers) == 0 {
			t.Fatalf("bad sound %q", s.recipe.ID)
		}
		seen[s.recipe.ID] = true
	}
}
