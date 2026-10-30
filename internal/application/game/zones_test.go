package game

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

func zoneCmd(g *Game, k command.Kind, x1, y1, x2, y2 int, build uint8) {
	g.Enqueue(command.Command{Kind: k, X: x1, Y: y1, X2: x2, Y2: y2, Build: build})
	g.ApplyPending()
}

func zoneOwner(t *testing.T, g *Game, x, y int) ZoneView {
	t.Helper()
	for _, v := range g.World().Zones {
		for _, c := range v.Cells {
			if c == [2]int{x, y} {
				return v
			}
		}
	}
	return ZoneView{}
}

// A tile is in one zone at most: a later drag skips taken tiles.
func TestZonesNeverOverlap(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 20})
	zoneCmd(g, command.DesignateStockpile, 2, 2, 6, 6, 0)
	zoneCmd(g, command.DesignateGrowZone, 4, 4, 9, 9, 0)
	zoneCmd(g, command.DesignatePen, 0, 0, 12, 12, 0)
	w := g.World()
	if len(w.Zones) != 3 {
		t.Fatalf("zones %+v", w.Zones)
	}
	seen := map[[2]int]bool{}
	for _, v := range w.Zones {
		for _, c := range v.Cells {
			if seen[c] {
				t.Fatalf("tile %v in two zones", c)
			}
			seen[c] = true
		}
	}
	if v := zoneOwner(t, g, 5, 5); v.Kind != ZoneStockpile || len(v.Cells) != 25 {
		t.Fatalf("the stockpile keeps its tiles: %+v", v)
	}
	if v := zoneOwner(t, g, 8, 8); v.Kind != ZoneGrow || len(v.Cells) != 36-9 || v.Crop != "potato" {
		t.Fatalf("grow zone %+v", v)
	}
	// PLAY-023: the map's edge ring (row 0, column 0: 25 cells) takes no zone
	if v := zoneOwner(t, g, 1, 1); v.Kind != ZonePen || len(v.Cells) != 169-25-27-25 {
		t.Fatalf("pen %+v", v)
	}
	if g.CanZoneAt(ZonePen, 5, 5) || !g.CanZoneAt(ZoneStockpile, 15, 15) || g.CanZoneAt("moat", 15, 15) || g.CanZoneAt(ZoneGrow, -1, 0) {
		t.Fatal("CanZoneAt: taken, free, unknown kind, off the map")
	}
	if p := zonesOf(w, ZonePen); len(zonesOf(w, ZoneStockpile)) != 1 || len(zonesOf(w, ZoneGrow)) != 1 ||
		len(p) != 1 || p[0].MinX != 1 || p[0].MinY != 1 || p[0].MaxX != 12 || p[0].MaxY != 12 {
		t.Fatalf("zone views %+v", w.Zones)
	}
}

// Touching zones of one kind are one zone; grow zones only with one crop.
func TestAdjacentZonesMerge(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	zoneCmd(g, command.DesignateStockpile, 1, 1, 3, 3, 0)
	zoneCmd(g, command.DesignateStockpile, 7, 1, 9, 3, 0)
	if len(g.World().Zones) != 2 {
		t.Fatal("apart: two stockpiles")
	}
	zoneCmd(g, command.DesignateStockpile, 4, 2, 6, 2, 0) // a bridge
	w := g.World()
	if len(w.Zones) != 1 || len(w.Zones[0].Cells) != 21 || w.Zones[0].ID != 1 {
		t.Fatalf("bridged stockpiles merge into the first: %+v", w.Zones)
	}
	radish, _ := CropID("radish")
	zoneCmd(g, command.DesignateGrowZone, 1, 10, 3, 12, radish)
	zoneCmd(g, command.DesignateGrowZone, 4, 10, 6, 12, radish)
	zoneCmd(g, command.DesignateGrowZone, 7, 10, 9, 12, uint8(plant.Tree))
	w = g.World()
	if len(w.Zones) != 3 || zoneOwner(t, g, 1, 10).ID != zoneOwner(t, g, 6, 12).ID {
		t.Fatalf("same crop merges: %+v", w.Zones)
	}
	if v := zoneOwner(t, g, 8, 11); v.Crop != "tree" || len(v.Cells) != 9 {
		t.Fatalf("a tree zone beside radishes stays apart: %+v", v)
	}
	// cancelling the middle splits the zone, settings kept
	zoneCmd(g, command.DesignateCancel, 3, 10, 4, 12, 0)
	a, b := zoneOwner(t, g, 1, 10), zoneOwner(t, g, 6, 10)
	if a.ID == b.ID || a.Crop != "radish" || b.Crop != "radish" || len(a.Cells) != 6 || len(b.Cells) != 6 {
		t.Fatalf("split %+v %+v", a, b)
	}
}

// Grow zones only take fertile ground: not rough stone or rock; gravel is
// poor but plantable. Stockpiles may sit on rough stone.
func TestGrowZoneSkipsInfertileGround(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	g.setTerrain(2, 1, terrain.Stone)
	g.setTerrain(3, 1, terrain.Rock)
	g.setTerrain(4, 1, terrain.Gravel)
	zoneCmd(g, command.DesignateGrowZone, 1, 1, 5, 1, 0)
	if v := zoneOwner(t, g, 1, 1); len(v.Cells) != 1 {
		t.Fatalf("west piece %+v", v)
	}
	if v := zoneOwner(t, g, 4, 1); len(v.Cells) != 2 || v.Kind != ZoneGrow {
		t.Fatalf("gravel is plantable: %+v", v)
	}
	if zoneOwner(t, g, 2, 1).ID != 0 || zoneOwner(t, g, 3, 1).ID != 0 {
		t.Fatal("rough stone and rock are not grow zone tiles")
	}
	if g.CanZoneAt(ZoneGrow, 2, 1) || !g.CanZoneAt(ZoneStockpile, 2, 1) || g.CanZoneAt(ZoneStockpile, 3, 1) {
		t.Fatal("CanZoneAt by kind and ground")
	}
	zoneCmd(g, command.DesignateStockpile, 2, 3, 2, 1, 0)
	if v := zoneOwner(t, g, 2, 1); v.Kind != ZoneStockpile || len(v.Cells) != 3 {
		t.Fatalf("stockpile on rough stone %+v", v)
	}
}

// A haul goes to the stockpile tile nearest the load; topping up a stack
// with room still comes first.
func TestStockpileCellPicksNearest(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 10})
	zoneCmd(g, command.DesignateStockpile, 1, 1, 3, 3, 0) // off the edge ring (PLAY-023)
	zoneCmd(g, command.DesignateStockpile, 20, 4, 23, 7, 0)
	if x, y, ok := g.stockpileCell(item.Stack{Kind: item.Wood}, 24, 8); !ok || x != 23 || y != 7 {
		t.Fatalf("nearest from the east: %d,%d %v", x, y, ok)
	}
	if x, y, ok := g.stockpileCell(item.Stack{Kind: item.Wood}, 10, 5); !ok || x != 3 || y != 1 {
		t.Fatalf("nearest from the middle (tie: lower y): %d,%d %v", x, y, ok)
	}
	if _, err := g.SpawnStack(item.Wood, 5, 1, 1); err != nil {
		t.Fatal(err)
	}
	if x, y, ok := g.stockpileCell(item.Stack{Kind: item.Wood}, 24, 8); !ok || x != 1 || y != 1 {
		t.Fatalf("top up the stack first: %d,%d %v", x, y, ok)
	}
	if x, y, ok := g.stockpileCell(item.Stack{Kind: item.GraniteBlocks}, 2, 2); !ok || x != 2 || y != 2 {
		t.Fatalf("another kind skips the stack's tile: %d,%d %v", x, y, ok)
	}
}

// Older saves hold rectangles: they are painted in saved order, so
// overlaps resolve first-wins and touching ones merge.
func TestLoadMigratesLegacyZoneRects(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 20})
	var buf bytes.Buffer
	if err := g.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	// rewrite the save as an older version would have: rectangles, no map
	zr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	dec := gob.NewDecoder(zr)
	var info SaveInfo
	var s saveState
	if err := dec.Decode(&info); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&s); err != nil {
		t.Fatal(err)
	}
	s.ZoneMap = zoneSave{}
	s.Stockpiles = []zone.Stockpile{{MinX: 0, MinY: 0, MaxX: 3, MaxY: 3, AllowFood: true}, {MinX: 4, MinY: 0, MaxX: 5, MaxY: 1, AllowWood: true}}
	s.GrowZones = []zone.GrowZone{{MinX: 2, MinY: 2, MaxX: 6, MaxY: 6, Crop: uint8(plant.Radish)}}
	s.Pens = []pen{{10, 10, 12, 12}}
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	enc := gob.NewEncoder(zw)
	if err := enc.Encode(info); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(&s); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&out)
	if err != nil {
		t.Fatal(err)
	}
	w := h.World()
	if len(w.Zones) != 2 {
		t.Fatalf("zones %+v", w.Zones)
	}
	if st := zoneOwner(t, h, 5, 1); st.Kind != ZoneStockpile || len(st.Cells) != 20 || !st.Food || st.Wood {
		t.Fatalf("touching stockpiles merge, first keeps its filters: %+v", st)
	}
	if gz := zoneOwner(t, h, 6, 6); gz.Kind != ZoneGrow || gz.Crop != "radish" || len(gz.Cells) != 25-4 {
		t.Fatalf("the grow zone keeps only free tiles: %+v", gz)
	}
	// PLAY-015: the pen rectangle became a pen marker in its middle (open:
	// no fence around it)
	if len(w.Pens) != 1 || w.Pens[0].X != 11 || w.Pens[0].Y != 11 || w.Pens[0].Enclosed {
		t.Fatalf("pen %+v", w.Pens)
	}
	// and the migrated map round-trips through a new save
	var again bytes.Buffer
	if err := h.Save(&again, "new"); err != nil {
		t.Fatal(err)
	}
	k, _, err := Load(&again)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(k.World().Zones, w.Zones) {
		t.Fatal("zone map round trip differs")
	}
}

// A tame animal keeps to a pen that is not a rectangle (PLAY-015: a
// fenced L with a pen marker).
func TestTameAnimalKeepsToLShapedPen(t *testing.T) {
	g := New(Config{Seed: 3, Width: 30, Height: 20})
	in := func(x, y int) bool {
		return x >= 14 && x <= 22 && y >= 8 && y <= 9 || x >= 14 && x <= 15 && y >= 10 && y <= 16
	}
	for y := 7; y <= 17; y++ {
		for x := 13; x <= 23; x++ {
			near := false
			for _, d := range [][2]int{{0, 0}, {1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
				near = near || in(x+d[0], y+d[1])
			}
			if near && !in(x, y) && !g.BuildNow("fence", "wood", "south", x, y) {
				t.Fatalf("fence at %d,%d", x, y)
			}
		}
	}
	m := buildMarker(t, g, 15, 12)
	if c := g.penRegion(m); !c.enclosed || c.cells != 9*2+2*7 {
		t.Fatalf("the L is one pen: %+v", *c)
	}
	buck, _ := g.SpawnAnimal("dustbuck", 18, 8)
	g.TameAnimal(buck, "Moss")
	// wandering inside, every walk ends on a pen tile
	for i := 0; i < 60; i++ {
		g.Step(25)
		a, _ := g.animals.Get(buck)
		if n := len(a.Path); n > 0 {
			if end := a.Path[n-1]; !in(end.X, end.Y) {
				t.Fatalf("wanders out of the pen to %v", end)
			}
		}
		if p, _ := g.positions.Get(buck); !in(p.X, p.Y) {
			t.Fatalf("out of the pen at %v", p)
		}
	}
}

// zonesOf is w's zones of one kind, in id order.
func zonesOf(w WorldView, kind string) []ZoneView {
	var out []ZoneView
	for _, z := range w.Zones {
		if z.Kind == kind {
			out = append(out, z)
		}
	}
	return out
}

// inZone reports whether tile (x, y) is one of z's cells.
func inZone(z ZoneView, x, y int) bool {
	for _, c := range z.Cells {
		if c == [2]int{x, y} {
			return true
		}
	}
	return false
}
