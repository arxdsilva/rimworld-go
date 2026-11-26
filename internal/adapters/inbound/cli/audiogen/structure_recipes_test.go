// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package audiogen

import (
	"math"
	"testing"

	"rimworld-go/pkg/audiomanifest"
	"rimworld-go/pkg/sfxsynth"
)

// The fifteen structure sounds: eight one-shots and seven bench and fuse loops.
func TestStructureSoundsRenderAndLoop(t *testing.T) {
	t.Parallel()
	got := structureSounds()
	if len(got) != 15 {
		t.Fatalf("%d structure sounds, want 15", len(got))
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
	if loops != 7 {
		t.Fatalf("%d loops, want 7", loops)
	}
}

// The turret fuse hiss: a seeded, one-second ambience loop that is
// deterministic and audible.
func TestTurretFuseHissLoop(t *testing.T) {
	t.Parallel()
	const id = "amb.turret_fuse_hiss"
	var found *sound
	for _, snd := range structureSounds() {
		if snd.recipe.ID == id {
			snd := snd
			found = &snd
		}
	}
	if found == nil {
		t.Fatalf("no recipe %s", id)
	}
	rc := found.recipe
	if rc.Loop != 1.0 || rc.Seed != 946 {
		t.Errorf("loop %v seed %d, want 1.0 and 946", rc.Loop, rc.Seed)
	}
	if audiomanifest.BusOf(id) != audiomanifest.BusAmbience {
		t.Errorf("%s: a loop belongs on the ambience bus", id)
	}
	a, b := sfxsynth.Render(rc, 0), sfxsynth.Render(rc, 0)
	if len(a) != len(b) {
		t.Fatalf("renders %d and %d samples", len(a), len(b))
	}
	var sum float64
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("sample %d differs between renders", i)
		}
		sum += a[i] * a[i]
	}
	if rms := math.Sqrt(sum / float64(len(a))); rms < 0.01 {
		t.Errorf("rms %.4f, want audible (>= 0.01)", rms)
	}
	if secs := float64(len(a)) / float64(sfxsynth.Rate); math.Abs(secs-1.0) > 0.05 {
		t.Errorf("%.3f s long, want about 1.0 s", secs)
	}
}
