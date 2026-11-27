// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
)

func saveThoughtGame(t *testing.T, c Content) []byte {
	t.Helper()
	g := New(Config{Seed: 11, Width: 12, Height: 8, Content: c})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.remember(id, mood.SleptOutside)
	g.afterEat(id, item.Stack{Kind: item.Potatoes, Qty: 1}, false)
	var buf bytes.Buffer
	if err := g.Save(&buf, "thoughts"); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// C5: the zero Content and an explicit core book save the same bytes.
func TestSaveIgnoresHowCoreThoughtsAreSpelled(t *testing.T) {
	t.Parallel()
	if !bytes.Equal(saveThoughtGame(t, Content{}), saveThoughtGame(t, Content{Thoughts: mood.Core()})) {
		t.Fatal("Content{} and Content{Thoughts: Core()} save differently")
	}
}

// C5: a save made under a patched book loads under core. A def-driven
// memory (slept outside) shows core's value and line again. A diet thought
// bakes its value into the memory when it is made, so the patched -3 stays
// until that memory expires; this is the documented behaviour, not a bug.
func TestPatchedSaveLoadsUnderCore(t *testing.T) {
	t.Parallel()
	patched := tunedThoughts(t, map[mood.Kind]func(*mood.Tuning){
		mood.SleptOutside: setValue("under the stars", -2),
		mood.AteRaw:       setValue("raw again", -3),
	})
	data := saveThoughtGame(t, Content{Thoughts: patched})
	l, _, err := LoadContent(bytes.NewReader(data), Content{})
	if err != nil {
		t.Fatal(err)
	}
	var id = l.Snapshot()[0].ID
	core := mood.Core().DefOf(mood.SleptOutside)
	var found bool
	for _, th := range l.thoughts(id) {
		switch th.Kind {
		case mood.SleptOutside:
			found = true
			if th.Value != core.Value || th.Label != core.Label {
				t.Errorf("slept outside shows %q %d, want core's %q %d", th.Label, th.Value, core.Label, core.Value)
			}
		case mood.AteRaw:
			if th.Value != -3 {
				t.Errorf("baked diet value %d, want the patched -3", th.Value)
			}
		}
	}
	if !found {
		t.Fatal("slept-outside memory lost in the load")
	}
}
