package game

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"testing"

	"rimworld-go/internal/application/game/bodytemp"
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// twoMaps is a colony with a second open map (MAP-0) at another size: a
// pawn, a wall, a stack, a sculpture and a chilled deer stand on map 1.
func twoMaps(t *testing.T) (g *Game, pawn, wall, stack, art, deer entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 3, Width: 20, Height: 12})
	if _, err := g.SpawnNamedPawn(2, 2, "Ada"); err != nil {
		t.Fatal(err)
	}
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 6, 6)
	m := newMapState(Config{Width: 16, Height: 10}, 1)
	m.tile = 42
	g.maps, g.mapState = append(g.maps, m), m
	initMapHosts(g)
	var err error
	if pawn, err = g.SpawnNamedPawn(3, 3, "Bo"); err != nil {
		t.Fatal(err)
	}
	wall = buildNow(t, g, construction.Wall, item.Wood, construction.South, 5, 5)
	art = buildNow(t, g, construction.SmallSculpture, item.Wood, construction.South, 9, 2)
	g.craft.Art.Import(map[entity.EntityID]craft.Artwork{art: {Seed: 7, Maker: "Bo"}})
	if stack, err = g.SpawnStack(item.Wood, 10, 7, 7); err != nil {
		t.Fatal(err)
	}
	if deer, err = g.SpawnAnimal("deer", 12, 8); err != nil {
		t.Fatal(err)
	}
	g.exposure[deer] = bodytemp.Sev{Hypo: 30}
	g.mapState = g.view
	g.Step(1) // rooms built, as a load builds them
	return
}

func TestSaveTwoMapsRoundTrip(t *testing.T) {
	t.Parallel()
	g, pawn, wall, stack, art, deer := twoMaps(t)
	h := saveLoad(t, g)
	if len(h.maps) != 2 || h.mapState != h.maps[0] || h.view != h.maps[0] {
		t.Fatalf("loaded %d maps, current map %d", len(h.maps), h.mapState.id)
	}
	if h.StateHash() != g.StateHash() {
		t.Fatal("a two-map save changed the state hash")
	}
	m := h.maps[1]
	if m.id != 1 || m.tile != 42 || m.grid.Width() != 16 || m.grid.Height() != 10 || m.climate != g.maps[1].climate {
		t.Fatalf("map 1 came back as id %d tile %d %dx%d", m.id, m.tile, m.grid.Width(), m.grid.Height())
	}
	for i, gm := range g.maps {
		hm := h.maps[i]
		for _, id := range gm.pawnIDs {
			if a, b := gm.positions.Ref(id), hm.positions.Ref(id); b == nil || *a != *b {
				t.Fatalf("map %d pawn %d moved", i, id)
			}
		}
		if len(hm.pawnIDs) != len(gm.pawnIDs) || len(hm.bldgIDs) != len(gm.bldgIDs) || len(hm.itemIDs) != len(gm.itemIDs) {
			t.Fatalf("map %d lost entities", i)
		}
	}
	if !m.positions.Has(pawn) || h.maps[0].positions.Has(pawn) || !m.bldgs.Has(wall) || !m.stacks.Has(stack) || !m.animals.Has(deer) {
		t.Fatal("map 1's pawn, wall, stack or deer is not on map 1")
	}
	if _, ok := h.exposure[deer]; !ok {
		t.Fatal("the exposure of an animal on map 1 was dropped")
	}
	h.mapState = m
	if h.craft.Art.Export()[art].Maker != "Bo" {
		t.Fatal("the sculpture on map 1 lost its artwork")
	}
	h.mapState = h.view
	g.Step(60)
	h.Step(60)
	if h.StateHash() != g.StateHash() {
		t.Fatal("a loaded two-map game diverged")
	}
}

func TestSaveKeepsViewedMap(t *testing.T) {
	t.Parallel()
	g, _, _, _, _, _ := twoMaps(t)
	before := g.StateHash()
	g.ViewMap(1)
	if g.StateHash() != before {
		t.Fatal("the viewed map is in the state hash")
	}
	h := saveLoad(t, g)
	if g.mapState != g.maps[1] {
		t.Fatal("Save left another map current")
	}
	if h.ViewedMap() != 1 || h.mapState != h.maps[1] {
		t.Fatalf("viewed map %d after load", h.ViewedMap())
	}
}

func TestSaveSingleMapHasNoExtraMaps(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	if _, err := g.SpawnNamedPawn(1, 1, "Ada"); err != nil {
		t.Fatal(err)
	}
	g.Step(1)
	var buf bytes.Buffer
	if err := g.Save(&buf, "one"); err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	dec := gob.NewDecoder(zr)
	var info SaveInfo
	var s saveState
	if dec.Decode(&info) != nil || dec.Decode(&s) != nil {
		t.Fatal("decode")
	}
	if info.Version != 12 || SaveVersion != 12 || s.ExtraMaps != nil || s.View != 0 {
		t.Fatalf("v%d save with %d extra maps, view %d", info.Version, len(s.ExtraMaps), s.View)
	}
	if l := loadAsVersion(t, g, 11); l.StateHash() != g.StateHash() || len(l.maps) != 1 {
		t.Fatal("a v11 save no longer loads as it was")
	}
	if SupportedSaveVersion(SaveVersion + 1) {
		t.Fatal("a newer save is accepted")
	}
}
