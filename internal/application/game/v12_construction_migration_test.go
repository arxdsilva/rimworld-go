// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"encoding/gob"
	"os"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/domain/economy/item"
	"testing"
)

// Undo only the intentional construction migration in an isolated restored
// state to verify the historical golden hash without changing its pin.
func TestV12ConstructionMigrationCausalReplay(t *testing.T) {
	data, err := os.ReadFile(saveV12Fixture)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := savefmt.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	d := gob.NewDecoder(zr)
	var info SaveInfo
	var historical saveState
	if err := d.Decode(&info); err != nil {
		t.Fatal(err)
	}
	if err := savefmt.Finish(d.Decode(&historical), zr); err != nil {
		t.Fatal(err)
	}
	g, _, err := Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	migrated := g.StateHash()
	again, _ := saveLoadInfo(t, g)
	if h := again.StateHash(); h != migrated {
		t.Fatalf("re-save changed migration hash: %x != %x", h, migrated)
	}
	g.Step(600)
	again.Step(600)
	if h, w := again.StateHash(), g.StateHash(); h != w {
		t.Fatalf("migration replay: %x != %x", h, w)
	}
	g, _, err = Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.maps) != 1+len(historical.ExtraMaps) {
		t.Fatal("map count changed")
	}
	changed := 0
	revert := func(saved *saveState) {
		for _, e := range saved.Frames {
			if fr, ok := g.frames.Get(e.ID); ok && fr.Kind == e.V.Kind && (fr.Need != e.V.Need || fr.CompNeed != e.V.CompNeed) {
				t.Logf("frame %d %s primary %d->%d component %d->%d", e.ID, fr.Kind, e.V.Need, fr.Need, e.V.CompNeed, fr.CompNeed)
				fr.Need, fr.CompNeed = e.V.Need, e.V.CompNeed
				g.frames.Set(e.ID, fr)
				changed++
			}
		}
		for _, id := range g.bldgIDs {
			if b, ok := g.bldgs.Get(id); ok && b.MaterialBasis != (item.MaterialBasis{}) {
				t.Logf("completed %d %s historical basis %+v", id, b.Kind, b.MaterialBasis)
				b.MaterialBasis = item.MaterialBasis{}
				g.bldgs.Set(id, b)
				changed++
			}
		}
		for _, id := range g.itemIDs {
			if st, ok := g.stacks.Get(id); ok && st.MaterialBasis != (item.MaterialBasis{}) {
				t.Logf("crate %d historical basis %+v", id, st.MaterialBasis)
				st.MaterialBasis = item.MaterialBasis{}
				g.stacks.Set(id, st)
				changed++
			}
		}
	}
	previous := g.mapState
	g.mapState = g.maps[0]
	revert(&historical)
	for i := range historical.ExtraMaps {
		m := &historical.ExtraMaps[i]
		g.mapState = mapByID(g, m.ID)
		if g.mapState == nil {
			t.Fatalf("map %d missing", m.ID)
		}
		revert(&m.State)
	}
	g.mapState = previous
	reverted := g.StateHash()
	const historicalV12Hash = uint64(0xde9f3c38a6932cd8)
	t.Logf("migrated=%#x reverted=%#x old pin=%#x construction changes=%d", migrated, reverted, historicalV12Hash, changed)
	if changed == 0 || reverted != historicalV12Hash {
		t.Fatal("construction migration alone does not explain the v12 hash delta")
	}
}
