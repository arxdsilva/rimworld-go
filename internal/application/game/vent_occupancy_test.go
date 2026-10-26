package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
)

// PLAY-021: steam vents moved to the foundation layer (so a geothermal
// generator can stand on one), but a vent still occupies its cells like
// it did on the object layer: no sowing, no grass regrowth, no smoothing
// and no stockpile storage on it.

// ventGame is a 40x30 colony and the first cell (scanning from 4,4) where
// ok holds, with a steam vent placed so that cell is its top-left.
func ventGame(t *testing.T, ok func(g *Game, x, y int) bool) (*Game, int, int) {
	t.Helper()
	g := New(Config{Seed: 3, Width: 40, Height: 30})
	for y := 4; y < 26; y++ {
		for x := 4; x < 36; x++ {
			if !ok(g, x, y) || !ok(g, x+1, y) || !ok(g, x, y+1) || !ok(g, x+1, y+1) {
				continue
			}
			if g.placeGenerated(construction.Building{Kind: construction.SteamVent, Stuff: item.GraniteBlocks, X: x, Y: y}) == 0 {
				continue
			}
			return g, x, y
		}
	}
	t.Skip("no 2x2 cell block where the check holds on this map")
	return nil, 0, 0
}

func TestNoSowingOnAVent(t *testing.T) {
	g, x, y := ventGame(t, func(g *Game, x, y int) bool { return g.sowable(x, y) })
	if g.sowable(x, y) {
		t.Fatalf("sowable on the vent at %d,%d", x, y)
	}
}

func TestNoGrassRegrowsOnAVent(t *testing.T) {
	g, x, y := ventGame(t, func(g *Game, x, y int) bool { return g.grassCanRegrow(x, y) })
	if g.grassCanRegrow(x, y) {
		t.Fatalf("grass regrows on the vent at %d,%d", x, y)
	}
}

func TestNoStockpileStorageOnAVent(t *testing.T) {
	g, x, y := ventGame(t, func(g *Game, x, y int) bool {
		return g.CanZoneAt(ZoneStockpile, x, y) && g.layerBldgAt(x, y, construction.LayerObject) == 0
	})
	g.paintZone(zone.KindStockpile, zone.Zone{Food: true, Wood: true, Stone: true}, x, y, x+1, y+1)
	if cx, cy, ok := g.stockpileCellWhere(item.Wood, x, y, nil); ok {
		t.Fatalf("a stockpile cell on the vent: %d,%d", cx, cy)
	}
}
