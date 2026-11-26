// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"os"
	"testing"

	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/sim/statehash"
)

func TestHistoricalOffmapConstruction(t *testing.T) {
	data, err := os.ReadFile("testdata/historical-v20-offmap-construction.sav")
	if err != nil {
		t.Fatal(err)
	}
	g, info, err := Load(bytes.NewReader(data))
	if err != nil || info.Version != 20 {
		t.Fatalf("load: %v, version %d", err, info.Version)
	}
	want := item.MaterialBasis{Primary: item.MaterialV20, Secondary: item.MaterialV20}
	check := func(g *Game) {
		t.Helper()
		find := func(stacks []item.Stack) {
			t.Helper()
			for _, st := range stacks {
				if st.Kind == item.MinifiedBuilding && st.Building == uint8(construction.Table) {
					if st.MaterialBasis != want || trade.CarryStack(st, 1).MaterialBasis != want {
						t.Fatalf("lost paid basis: %+v", st)
					}
					return
				}
			}
			t.Fatal("missing historical table")
		}
		find(travelOf(g).Goods(60))
		var pack []item.Stack
		for _, ent := range senseOf(g).Packs[entity.EntityID(27)].Items {
			pack = append(pack, ent.Stack)
		}
		find(pack)
	}
	check(g)
	h := saveLoad(t, g)
	check(h)
	if g.StateHash() != h.StateHash() {
		t.Fatal("historical cargo checkpoint changed hash")
	}
}

func TestV12MaterialBasisCausalHash(t *testing.T) {
	data, err := os.ReadFile(saveV12Fixture)
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	const oldV12Reference = uint64(0xde9f3c38a6932cd8)
	hash := g.StateHash()
	t.Logf("migrated v12 hash=%#x historical reference=%#x", hash, oldV12Reference)
	if hash == oldV12Reference {
		t.Fatal("historical material-basis migration did not change the authoritative state")
	}
	h := saveLoad(t, g)
	if h.StateHash() != hash {
		t.Fatal("v12 reload changes state")
	}
	g.Step(600)
	h.Step(600)
	if a, b := g.StateHash(), h.StateHash(); a != b {
		t.Fatalf("v12 continuation %#x != %#x", a, b)
	}
	g, _, err = Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	info, s, err := snapshotForSave(g, savefmt.Stamp{Name: "statehash"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var changed []entity.EntityID
	var neutralize func(*saveState)
	neutralize = func(s *saveState) {
		for i := range s.Bldgs {
			b := &s.Bldgs[i]
			if b.V.MaterialBasis != (item.MaterialBasis{}) {
				t.Logf("building %d kind=%v basis=%+v", b.ID, b.V.Kind, b.V.MaterialBasis)
				changed = append(changed, b.ID)
				b.V.MaterialBasis = item.MaterialBasis{}
			}
		}
		for i := range s.ExtraMaps {
			neutralize(&s.ExtraMaps[i].State)
		}
	}
	neutralize(s)
	if len(changed) != 8 {
		t.Fatalf("expected the eight historical buildings, got %v", changed)
	}
	without := statehash.OfFields(normalisedFields(info, s))
	t.Logf("v12 hash with only building material basis neutralised=%#x", without)
	if without != oldV12Reference {
		t.Fatalf("migration does not explain the entire old/new delta: %#x != %#x", without, oldV12Reference)
	}
}
