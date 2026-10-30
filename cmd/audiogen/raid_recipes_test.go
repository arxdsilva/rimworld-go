package main

import (
	"testing"

	"rimworld-go/pkg/audiomanifest"
	"rimworld-go/pkg/sfxsynth"
)

// The three wave E raid sounds: a pod falling (about 2 s), landing, and
// the ship part's ambience loop.
func TestRaidSoundsRenderAndLoop(t *testing.T) {
	t.Parallel()
	got := raidSounds()
	if len(got) != 3 {
		t.Fatalf("%d raid sounds, want 3", len(got))
	}
	for _, snd := range got {
		rc := snd.recipe
		a, b := sfxsynth.Render(rc, 0), sfxsynth.Render(rc, 0)
		if len(a) == 0 || len(a) != len(b) {
			t.Fatalf("%s renders %d and %d samples", rc.ID, len(a), len(b))
		}
		if rc.ID == "sfx.droppod_incoming" {
			if sec := float64(len(a)) / float64(sfxsynth.Rate); sec < 1.8 || sec > 2.4 {
				t.Errorf("%s is %.2f s, want about 2", rc.ID, sec)
			}
		}
		if (rc.Loop > 0) != (rc.ID == "amb.ship_part_hum") || rc.Loop > 0 && audiomanifest.BusOf(rc.ID) != audiomanifest.BusAmbience {
			t.Errorf("%s: loop %v on bus %v", rc.ID, rc.Loop, audiomanifest.BusOf(rc.ID))
		}
	}
}

// TestWave3SoundsRender: the ten wave 3 one-shots render, are short and not
// looped.
func TestWave3SoundsRender(t *testing.T) {
	t.Parallel()
	got := wave3Sounds()
	if len(got) != 10 {
		t.Fatalf("%d wave 3 sounds, want 10", len(got))
	}
	for _, snd := range got {
		rc := snd.recipe
		a := sfxsynth.Render(rc, 0)
		if sec := float64(len(a)) / float64(sfxsynth.Rate); len(a) == 0 || sec > 1.5 || rc.Loop > 0 {
			t.Errorf("%s: %.2f s, loop %v", rc.ID, sec, rc.Loop)
		}
	}
}
