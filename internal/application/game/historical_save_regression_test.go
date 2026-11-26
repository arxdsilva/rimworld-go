// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"testing"

	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/statehash"
)

func TestLegacySaveWithoutPriorities(t *testing.T) {
	g := NewColony(Config{Seed: 7, Width: 48, Height: 40})
	id := g.pawnIDs[0]
	delete(g.prio, id)
	old := work.Mask(0).Set(work.Haul, true).Set(work.Grow, true).Set(work.Craft, true)
	g.labor.Set(id, old)
	before := g.prioOf(id)
	if before[work.Haul] == 0 || before[work.Grow] == 0 {
		t.Fatalf("invalid fixture: %v", before)
	}
	migrateLegacyLabor(g, 1, []entry[legacyLaborMask]{{ID: id, V: legacyLaborMask(old)}})
	after := g.prioOf(id)
	t.Logf("before=%v after=%v", before, after)
	if after[work.Haul] != before[work.Haul] || after[work.Grow] != before[work.Grow] || after[work.Craft] != before[work.Craft] {
		t.Fatal("enabled legacy work lost")
	}
}
func TestV1SaveHandleWorkMigration(t *testing.T) {
	old := work.Mask(0).Set(work.Handle, true)
	got := work.LegacyMask(1, uint16(old), 0, false)
	if !got.Enabled(work.Handle) {
		t.Fatal("v1 Handle bit lost")
	}
}
func TestVisitGroupSequence(t *testing.T) {
	g := New(Config{Seed: 7, Width: 20, Height: 20})
	h := visitGround{g: g}
	id := h.GroupID()
	g.worldS().visitGroups = []visit.Group{{ID: id}}
	visitsOf(g).TickGroups(1)
	if len(g.worldS().visitGroups) != 0 {
		t.Fatal("group not closed")
	}
	t.Logf("closed group=%d exportWorld nil=%v", id, g.exportWorld() == nil)
	loaded := saveLoad(t, g)
	a, b := h.GroupID(), (visitGround{g: loaded}).GroupID()
	t.Logf("next original=%d loaded=%d", a, b)
	if a != b {
		t.Fatal("group sequence lost on reload")
	}
}
func TestLastBillHash(t *testing.T) {
	g := New(Config{Seed: 7, Width: 20, Height: 20})
	id := buildNow(t, g, construction.Campfire, item.Wood, construction.South, 7, 7)
	if len(g.craft.Bills.Of(id)) != 1 {
		t.Fatal("fixture missing default bill")
	}
	bid, _ := g.craft.Bills.Apply(command.Command{Kind: command.RemoveBill, X: 7, Y: 7, Index: 0})
	if bid != id {
		t.Fatalf("remove failed: %d vs %d", bid, id)
	}
	live := g.craft.Bills.Of(id)
	if live == nil || len(live) != 0 {
		t.Fatalf("fixture did not create allocated empty: %v", live)
	}
	var raw bytes.Buffer
	if err := encodeSave(g, &raw, savefmt.Stamp{Name: "statehash"}, nil); err != nil {
		t.Fatal(err)
	}
	dec := gob.NewDecoder(&raw)
	var info SaveInfo
	var saved saveState
	if err := dec.Decode(&info); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&saved); err != nil {
		t.Fatal(err)
	}
	a, b := g.StateHash(), statehash.OfFields(normalisedFields(info, &saved))
	t.Logf("live bill nil=%v decoded bill nil=%v direct hash=%x decoded hash=%x", live == nil, saved.Bills[id] == nil, a, b)
	if a != b {
		t.Fatal("direct state hash differs from persisted state")
	}
}

func TestRejectReverseMixedMapSave(t *testing.T) {
	g, _, _, _, _, _ := twoMaps(t)
	info, s := animalSaveState(t, g)
	s.Animals = make([]entry[animal.State], 0, len(s.AnimalRecords))
	for _, r := range s.AnimalRecords {
		s.Animals = append(s.Animals, entry[animal.State]{ID: r.ID, V: r.State.State()})
	}
	s.AnimalFormat, s.AnimalRecords = 0, nil
	if len(s.WorldAnimalRecords) != 0 {
		t.Fatal("fixture unexpectedly contains world animals")
	}
	if _, err := loadAnimalStateFile(t, info, s); err == nil {
		t.Fatal("mixed root legacy/extra grouped accepted")
	}
}

func TestMothballCheckpoint(t *testing.T) {
	g, id := freshWorldPawn(t)
	g.Step(15001)
	if !g.worldSt.worldPawnMothballed[id] {
		t.Fatal("fixture not mothballed")
	}
	loaded := saveLoad(t, g)
	if !loaded.worldSt.worldPawnMothballed[id] {
		t.Fatal("lost cadence membership on load")
	}
	for _, ticks := range []int{1, 200, 15000} {
		g.Step(ticks)
		loaded.Step(ticks)
		if g.StateHash() != loaded.StateHash() {
			t.Fatalf("checkpoint continuation diverged after %d ticks", ticks)
		}
	}
}

// These files were produced by the actual historical v1 writers, not by
// relabeling a modern snapshot. See testdata/historical-v1-labor-fixtures.md.
func TestHistoricalWriterLabor(t *testing.T) {
	for _, n := range []int{11, 12} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			f, err := os.Open(fmt.Sprintf("testdata/historical-v1-labor-%d.sav", n))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			g, info, err := Load(f)
			if err != nil {
				t.Fatal(err)
			}
			if info.Version != 1 || len(g.pawnIDs) != 1 {
				t.Fatalf("unexpected fixture: %+v", info)
			}
			id := g.pawnIDs[0]
			got := g.prioOf(id)
			want := map[work.Type]uint8{
				work.Clean: 3, work.Haul: 3, work.Craft: 3, work.Smith: 3, work.Tailor: 3,
				work.Patient: 3, work.BedRest: 3, work.Basic: 3,
				work.Handle: 0, work.Grow: 0, work.PlantCut: 0,
			}
			if n == 12 {
				want[work.Clean], want[work.Haul], want[work.Craft] = 2, 0, 4
				want[work.Smith], want[work.Tailor] = 4, 4
				want[work.Handle], want[work.Grow], want[work.PlantCut] = 1, 2, 2
			}
			for w, p := range want {
				if got[w] != p {
					t.Errorf("work %v = %d, want %d (all: %v)", w, got[w], p, got)
				}
			}
			mask, _ := g.labor.Get(id)
			if mask != got.Mask() {
				t.Fatalf("mask %v != priorities %v", mask, got)
			}
			if loaded := saveLoad(t, g); loaded.prioOf(id) != got {
				t.Fatal("migrated priorities not durable")
			}
		})
	}
}
