// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/modstate"
)

// countingSource is a fake modstate.Source: it hands out its blobs and
// counts the calls.
type countingSource struct {
	blobs []modstate.Blob
	calls int
}

func (s *countingSource) SaveBlobs() []modstate.Blob {
	s.calls++
	return s.blobs
}

func twoBlobs() []modstate.Blob {
	return []modstate.Blob{
		{Plugin: "tally", File: "scripts/tally.wasm", Data: []byte{1, 0, 0, 0, 7}},
		{Plugin: "alpha", File: "scripts/a.wasm", Data: []byte{2}},
	}
}

func loadBytes(t *testing.T, data []byte) *Game {
	t.Helper()
	g, _, err := Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func saveBytes(t *testing.T, g *Game) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := g.Save(&buf, "mods"); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestModStateRoundTrip (PLG-063 C1): a v18 save keeps the source's blobs,
// TakeLoadedModState hands them over once, and a second take is empty.
func TestModStateRoundTrip(t *testing.T) {
	g := NewColony(Config{Seed: 3, Width: 32, Height: 32})
	src := &countingSource{blobs: twoBlobs()}
	UseModState(g, src)
	h := loadBytes(t, saveBytes(t, g))
	if src.calls != 1 {
		t.Fatalf("a save asked the source %d times, want 1", src.calls)
	}
	blobs, notes := TakeLoadedModState(h)
	if !reflect.DeepEqual(blobs, twoBlobs()) || notes != nil {
		t.Fatalf("loaded %v, notes %v; want %v and none", blobs, notes, twoBlobs())
	}
	if blobs, notes := TakeLoadedModState(h); blobs != nil || notes != nil {
		t.Fatalf("a second take gave %v, %v; want nothing", blobs, notes)
	}
}

// TestModStateChecksBothWays: Check runs when saving (a blob over the cap is
// never written) and when loading (a hand-made save's bad blob becomes a
// note).
func TestModStateChecksBothWays(t *testing.T) {
	g := NewColony(Config{Seed: 3, Width: 32, Height: 32})
	big := modstate.Blob{Plugin: "p", File: "big.wasm", Data: make([]byte, modstate.MaxData+1)}
	UseModState(g, &countingSource{blobs: append(twoBlobs(), big)})
	blobs, notes := TakeLoadedModState(loadBytes(t, saveBytes(t, g)))
	if !reflect.DeepEqual(blobs, twoBlobs()) || notes != nil {
		t.Fatalf("saved %v, notes %v; want the two small blobs and no note", blobs, notes)
	}

	h := loadAsVersionWith(t, NewColony(Config{Seed: 3, Width: 32, Height: 32}), SaveVersion, func(s *saveState) {
		s.ModState = []modstate.Blob{twoBlobs()[0], twoBlobs()[0], {File: "x.wasm"}}
	})
	blobs, notes = TakeLoadedModState(h)
	if len(blobs) != 1 || len(notes) != 2 || notes[0].File != "scripts/tally.wasm" || notes[1].File != "x.wasm" {
		t.Fatalf("loaded %v, notes %v; want one blob and two notes", blobs, notes)
	}
}

// TestStateHashNeverAsksModState (PLG-063 C2): the state hash makes no
// Source call, and a Source does not move it.
func TestStateHashNeverAsksModState(t *testing.T) {
	g := NewColony(Config{Seed: 5, Width: 32, Height: 32})
	g.Step(120)
	without := g.StateHash()
	src := &countingSource{blobs: twoBlobs()}
	UseModState(g, src)
	with := g.StateHash()
	if src.calls != 0 {
		t.Fatalf("StateHash asked the source %d times, want 0", src.calls)
	}
	if with != without {
		t.Fatalf("hash with a source %#x, without %#x", with, without)
	}
}

// TestEncodeSaveLeavesTheRun (PLG-063 C3): saving between steps, with a
// source, leaves the run where a run without saves goes.
func TestEncodeSaveLeavesTheRun(t *testing.T) {
	a := NewColony(Config{Seed: 7, Width: 32, Height: 32})
	b := NewColony(Config{Seed: 7, Width: 32, Height: 32})
	src := &countingSource{blobs: twoBlobs()}
	UseModState(b, src)
	for range 5 {
		a.Step(60)
		b.Step(60)
		if err := EncodeSave(b, io.Discard, "between"); err != nil {
			t.Fatal(err)
		}
	}
	if src.calls != 5 {
		t.Fatalf("5 saves asked the source %d times", src.calls)
	}
	if ha, hb := a.StateHash(), b.StateHash(); ha != hb {
		t.Fatalf("saving moved the run: %#x vs %#x", hb, ha)
	}
}

// TestOldSavesLoadWithoutModState (PLG-063 C4): the v1, v2 and v12
// fixtures, and a save one version back, load with no mod state and no
// notes.
func TestOldSavesLoadWithoutModState(t *testing.T) {
	for _, path := range []string{"testdata/premotion_v1.sav", ng1V2Fixture, saveV12Fixture} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if blobs, notes := TakeLoadedModState(loadBytes(t, data)); blobs != nil || notes != nil {
			t.Errorf("%s: mod state %v, notes %v; want none", path, blobs, notes)
		}
	}
	g := NewColony(Config{Seed: 3, Width: 32, Height: 32})
	UseModState(g, &countingSource{blobs: twoBlobs()})
	if blobs, notes := TakeLoadedModState(loadAsVersion(t, g, 17)); blobs != nil || notes != nil {
		t.Errorf("v%d: mod state %v, notes %v; want none", 17, blobs, notes)
	}
}

// TestModStateNotInheritedAcrossLoads (PLG-063 C5): a v17 save loaded after
// a v18 one with blobs has none.
func TestModStateNotInheritedAcrossLoads(t *testing.T) {
	g := NewColony(Config{Seed: 3, Width: 32, Height: 32})
	UseModState(g, &countingSource{blobs: twoBlobs()})
	first := loadBytes(t, saveBytes(t, g))
	if blobs, _ := TakeLoadedModState(first); len(blobs) != 2 {
		t.Fatalf("v18 load gave %v, want 2 blobs", blobs)
	}
	UseModState(first, &countingSource{blobs: twoBlobs()})
	second := loadAsVersion(t, first, 17)
	if blobs, notes := TakeLoadedModState(second); blobs != nil || notes != nil {
		t.Fatalf("v17 load after a v18 one gave %v, %v; want nothing", blobs, notes)
	}
}

// TestSaveNotesWhatItDrops (PLG-063 C9): a blob dropped while saving leaves
// a note for TakeLoadedModState on the saving game, and the note does not
// move the state hash.
func TestSaveNotesWhatItDrops(t *testing.T) {
	g := NewColony(Config{Seed: 3, Width: 32, Height: 32})
	before := g.StateHash()
	big := modstate.Blob{Plugin: "p", File: "big.wasm", Data: make([]byte, modstate.MaxData+1)}
	UseModState(g, &countingSource{blobs: append(twoBlobs(), big)})
	for range 2 {
		if err := EncodeSave(g, io.Discard, "drops"); err != nil {
			t.Fatal(err)
		}
	}
	blobs, notes := TakeLoadedModState(g)
	if blobs != nil || len(notes) != 2 || notes[0].File != "big.wasm" || notes[1].File != "big.wasm" {
		t.Fatalf("after 2 saves: blobs %v, notes %v; want a note for big.wasm per save", blobs, notes)
	}
	if after := g.StateHash(); after != before {
		t.Fatalf("saving with a dropped blob moved the hash: %#x vs %#x", after, before)
	}
	if _, notes := TakeLoadedModState(g); notes != nil {
		t.Fatalf("a second take gave %v", notes)
	}
}
