// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"encoding/gob"
	"testing"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/sim/statehash"
)

func TestLegacyLaborGobImport(t *testing.T) {
	type legacyEntry struct {
		ID entity.EntityID
		V  uint16
	}
	original := struct{ Labor []legacyEntry }{
		Labor: []legacyEntry{{ID: 7, V: uint16(1<<work.Doctor | 1<<work.Haul)}},
	}
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(original); err != nil {
		t.Fatal(err)
	}
	var decoded saveState
	if err := gob.NewDecoder(&b).Decode(&decoded); err != nil {
		t.Fatalf("decode original uint16 Labor record: %v", err)
	}
	restored, ok := importLabor(decoded.Labor, decoded.LaborWide).Get(7)
	if !ok || !restored.Enabled(work.Doctor) || !restored.Enabled(work.Haul) ||
		restored.Enabled(work.Tailor) {
		t.Fatalf("old work mask decoded incorrectly: %b", restored)
	}
}

// Save hash keeps the same lower sixteen bits under the legacy Labor key.
// New work columns contribute only when any of those bits is enabled.
func TestWorkMaskStateHashIgnoresWireWidth(t *testing.T) {
	old := saveState{Labor: []entry[legacyLaborMask]{{ID: 3, V: 0x81}}}
	wide := saveState{LaborWide: []entry[work.Mask]{{ID: 3, V: 0x81}}}
	legacyHash := statehash.OfFields(normalisedFields(SaveInfo{}, &old))
	if got := statehash.OfFields(normalisedFields(SaveInfo{}, &wide)); got != legacyHash {
		t.Fatalf("wide storage changed old work-mask hash: got %#x want %#x", got, legacyHash)
	}
	wide.LaborWide[0].V |= work.Mask(1) << work.Tailor
	if got := statehash.OfFields(normalisedFields(SaveInfo{}, &wide)); got == legacyHash {
		t.Fatal("enabling a new work column did not change state hash")
	}
}

func TestArtWorkLegacyAndExplicitOffRoundTrip(t *testing.T) {
	g := New(Config{Seed: 83, Width: 20, Height: 12})
	a, _ := g.SpawnNamedPawn(3, 3, "Ada")
	b, _ := g.SpawnNamedPawn(7, 3, "Bo")
	g.Step(1) // initialize native caches just as a live colony does
	g.setPriority(a, work.Craft, 2)
	g.setPriority(a, work.Art, 2)
	g.setPriority(b, work.Craft, 0)
	g.setPriority(b, work.Art, 0)
	base := g.StateHash()
	old := loadAsVersionWith(t, g, 21, func(s *saveState) {
		for id, row := range s.PrioList {
			s.PrioList[id] = row[:int(work.Art)]
		}
		for i := range s.LaborWide {
			s.LaborWide[i].V = s.LaborWide[i].V.Set(work.Art, false)
		}
	})
	if old.prioOf(a)[work.Art] != 2 || old.prioOf(b)[work.Art] != 0 {
		t.Fatalf("historical Art choices: %v %v", old.prioOf(a), old.prioOf(b))
	}
	assertWakeHash(t, "current save round trip", g, saveLoad(t, g))
	assertWakeHash(t, "legacy Art inheritance", g, old)
	old.setPriority(a, work.Art, 0)
	if old.StateHash() == base {
		t.Fatal("independent disabled Art not represented in hash")
	}
	next := saveLoad(t, old)
	if next.prioOf(a)[work.Art] != 0 || next.prioOf(a)[work.Craft] != 2 || next.StateHash() != old.StateHash() {
		t.Fatal("explicit Art off was not preserved")
	}
}
