package main

import (
	"testing"

	"rimworld-go/pkg/audiomanifest"
	"rimworld-go/pkg/sfxsynth"
)

// The fourteen wave D structure sounds: eight one-shots and six bench loops.
func TestStructureSoundsRenderAndLoop(t *testing.T) {
	t.Parallel()
	got := structureSounds()
	if len(got) != 14 {
		t.Fatalf("%d structure sounds, want 14", len(got))
	}
	loops := 0
	for _, snd := range got {
		rc := snd.recipe
		a, b := sfxsynth.Render(rc, 0), sfxsynth.Render(rc, 0)
		if len(a) == 0 || len(a) != len(b) {
			t.Fatalf("%s renders %d and %d samples", rc.ID, len(a), len(b))
		}
		if rc.Loop > 0 {
			loops++
			if audiomanifest.BusOf(rc.ID) != audiomanifest.BusAmbience {
				t.Errorf("%s: a loop belongs on the ambience bus", rc.ID)
			}
		}
	}
	if loops != 6 {
		t.Fatalf("%d loops, want 6", loops)
	}
}
