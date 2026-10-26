package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/terrain"
)

// PLAY-021 (INFRA): where the electric pieces may stand.

func placementGame() *Game { return New(Config{Seed: 1, Width: 20, Height: 12}) }

// ventAt puts a steam vent (2x2) with its top-left cell at (x, y).
func ventAt(t *testing.T, g *Game, x, y int) {
	t.Helper()
	if id := g.placeGenerated(construction.Building{Kind: construction.SteamVent, X: x, Y: y}); id == 0 {
		t.Fatalf("no steam vent at %d,%d", x, y)
	}
}

func TestGeothermalSameSizeAsVent(t *testing.T) {
	t.Parallel()
	if construction.DefOf(construction.GeothermalGenerator).Size != construction.DefOf(construction.SteamVent).Size {
		t.Fatal("a geothermal generator must cover a steam vent exactly")
	}
}

func TestGeothermalOnlyExactlyOverASteamVent(t *testing.T) {
	t.Parallel()
	g := placementGame()
	ventAt(t, g, 6, 4) // covers (6,4) (7,4) (6,5) (7,5)
	geo := construction.GeothermalGenerator
	if ok, r := g.canPlaceStuffAt(geo, item.Steel, construction.South, 6, 4); !ok {
		t.Fatalf("a geothermal generator over the vent: %q", r)
	}
	for _, c := range []struct {
		x, y int
		want string
	}{
		{12, 8, ReasonNeedsVent}, // open ground
		{5, 3, ReasonNeedsVent},  // one cell short of the vent: touches it
		{7, 5, ReasonNeedsVent},  // overlaps one cell of the vent
		{7, 4, ReasonNeedsVent},  // half over the vent
		{5, 4, ReasonNeedsVent},  // half over the vent
	} {
		if ok, r := g.canPlaceStuffAt(geo, item.Steel, construction.South, c.x, c.y); ok || r != c.want {
			t.Errorf("geothermal at %d,%d: %v %q, want %q", c.x, c.y, ok, r, c.want)
		}
	}
	// anything else on the vent is still refused as a natural feature
	for _, k := range []construction.Kind{construction.Wall, construction.Lamp, construction.Battery, construction.SolarPanel} {
		st := item.Wood
		if construction.DefOf(k).MetalOnly {
			st = item.Steel
		}
		if ok, r := g.canPlaceStuffAt(k, st, construction.South, 6, 4); ok || r != ReasonNatural {
			t.Errorf("%s on the vent: %v %q, want %q", k, ok, r, ReasonNatural)
		}
	}
	// the public rule agrees (research aside)
	if ok, r := g.CanPlaceAt("geothermal generator", "steel", "south", 12, 8); ok {
		t.Fatalf("CanPlaceAt open ground: %q", r)
	}
}

func TestGeothermalIsBuiltOverTheVentAndOnceOnly(t *testing.T) {
	t.Parallel()
	g := placementGame()
	ventAt(t, g, 6, 4)
	geo := construction.GeothermalGenerator
	id := buildNow(t, g, geo, item.Steel, construction.South, 6, 4)
	for _, c := range [][2]int{{6, 4}, {7, 4}, {6, 5}, {7, 5}} {
		if g.layerBldgAt(c[0], c[1], construction.LayerObject) == 0 || g.grid.Walkable(c[0], c[1]) {
			t.Fatalf("cell %v should be covered by the generator", c)
		}
	}
	if got := g.layerBldgAt(7, 5, construction.LayerObject); got != id {
		t.Fatalf("generator covers (7,5) as %d, want %d", got, id)
	}
	if ok, _ := g.canPlaceStuffAt(geo, item.Steel, construction.South, 6, 4); ok {
		t.Fatal("a second generator over the first")
	}
	// the vent keeps being a natural feature
	if !g.naturalAt(6, 4) {
		t.Fatal("the vent is still there under the generator")
	}
	// and a planned generator holds the vent
	g2 := placementGame()
	ventAt(t, g2, 6, 4)
	g2.placeFrameStuff(geo, item.Steel, construction.South, 6, 4)
	if g2.layerFrameAt(6, 4, construction.LayerObject) == 0 {
		t.Fatal("no geothermal frame over the vent")
	}
	if ok, r := g2.canPlaceStuffAt(geo, item.Steel, construction.South, 6, 4); ok || r != ReasonPlanned {
		t.Fatalf("a second blueprint over the first: %v %q", ok, r)
	}
}

func TestCoolerRules(t *testing.T) {
	t.Parallel()
	g := placementGame()
	for x := 4; x <= 8; x++ {
		if x != 6 { // a gap at 6,4
			buildNow(t, g, construction.Wall, item.Wood, construction.South, x, 4)
		}
	}
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 6, 8) // a lone wall piece
	buildNow(t, g, construction.Fence, item.Wood, construction.South, 12, 6)
	buildNow(t, g, construction.Door, item.Wood, construction.South, 14, 6)
	cooler := construction.Cooler
	// the gap between two walls is fine, and so is a wall piece
	if ok, r := g.canPlaceStuffAt(cooler, item.Steel, construction.South, 6, 4); !ok {
		t.Fatalf("cooler in a wall gap: %q", r)
	}
	if ok, r := g.canPlaceStuffAt(cooler, item.Steel, construction.South, 5, 4); !ok {
		t.Fatalf("cooler replacing a wall: %q", r)
	}
	if got := g.ReplacesAt("cooler", "steel", 5, 4); got != "wall" {
		t.Fatalf("ReplacesAt = %q, want wall", got)
	}
	if got := g.ReplacesAt("cooler", "steel", 6, 4); got != "" {
		t.Fatalf("a gap replaces nothing, got %q", got)
	}
	if ok, r := g.canPlaceStuffAt(cooler, item.Steel, construction.South, 6, 8); !ok {
		t.Fatalf("cooler replacing a lone wall piece: %q", r)
	}
	// open ground is not a wall
	for _, c := range [][2]int{{10, 8}, {6, 6}, {3, 4}, {9, 4}} {
		if ok, r := g.canPlaceStuffAt(cooler, item.Steel, construction.South, c[0], c[1]); ok || r != ReasonNeedsWall {
			t.Errorf("cooler on open ground %v: %v %q, want %q", c, ok, r, ReasonNeedsWall)
		}
	}
	// it replaces walls only: a fence or a door stays
	for _, c := range [][2]int{{12, 6}, {14, 6}} {
		if ok, r := g.canPlaceStuffAt(cooler, item.Steel, construction.South, c[0], c[1]); ok || r != ReasonOccupied {
			t.Errorf("cooler over the piece at %v: %v %q, want %q", c, ok, r, ReasonOccupied)
		}
	}
	// a wall piece that the cooler replaces keeps the wall-replacement mark
	// flow of a door: the wall comes down first
	g.placeFrameStuff(cooler, item.Steel, construction.South, 5, 4)
	fid := g.layerFrameAt(5, 4, construction.LayerObject)
	fr, ok := g.frames.Get(fid)
	if !ok || fr.Replaces == 0 || !fr.ReplaceMarked {
		t.Fatalf("cooler frame over the wall: %+v ok=%v", fr, ok)
	}
	if b, _ := g.bldgs.Get(fr.Replaces); !b.Deconstruct || b.Kind != construction.Wall {
		t.Fatalf("the wall under the cooler is not marked: %+v", b)
	}
	if ok, r := g.canPlaceStuffAt(cooler, item.Steel, construction.South, 5, 4); ok || r != ReasonPlanned {
		t.Fatalf("second cooler blueprint: %v %q", ok, r)
	}
}

func TestCoolerFacesAcrossItsWall(t *testing.T) {
	t.Parallel()
	g := placementGame()
	// an east-west wall (y=4) and a north-south wall (x=14)
	for x := 4; x <= 8; x++ {
		buildNow(t, g, construction.Wall, item.Wood, construction.South, x, 4)
	}
	for y := 3; y <= 7; y++ {
		buildNow(t, g, construction.Wall, item.Wood, construction.South, 14, y)
	}
	cases := []struct {
		x, y     int
		prefer   string
		wantAuto string
	}{
		{6, 4, "south", "south"}, // east-west wall: cold side north or south
		{6, 4, "north", "north"},
		{6, 4, "east", "north"}, // across the wall: turned on in rotation order
		{6, 4, "west", "south"},
		{14, 5, "east", "east"}, // north-south wall: cold side east or west
		{14, 5, "west", "west"},
		{14, 5, "south", "east"},
		{14, 5, "north", "west"},
	}
	for _, c := range cases {
		if got := g.AutoFacingAt("cooler", c.prefer, c.x, c.y); got != c.wantAuto {
			t.Errorf("cooler at %d,%d preferring %s faces %s, want %s", c.x, c.y, c.prefer, got, c.wantAuto)
		}
	}
	// the planned cooler takes the facing across its wall
	g.placeFrameStuff(construction.Cooler, item.Steel, construction.East, 6, 4)
	fr, _ := g.frames.Get(g.layerFrameAt(6, 4, construction.LayerObject))
	if fr.Facing != construction.North {
		t.Fatalf("frame facing = %s, want north", fr.Facing)
	}
	// doors keep their own rule
	if got := g.AutoFacingAt("door", "east", 6, 4); got != "south" {
		t.Fatalf("door in an east-west wall faces %s, want south", got)
	}
}

func TestCoolerFacingTable(t *testing.T) {
	t.Parallel()
	// h counts walls east and west of the cooler, v those north and south
	cases := []struct {
		h, v   int
		prefer construction.Facing
		want   construction.Facing
	}{
		{2, 0, construction.South, construction.South},
		{2, 0, construction.East, construction.North},
		{2, 0, construction.North, construction.North},
		{2, 0, construction.West, construction.South},
		{0, 2, construction.South, construction.East},
		{0, 2, construction.East, construction.East},
		{0, 2, construction.North, construction.West},
		{0, 2, construction.West, construction.West},
		{1, 0, construction.West, construction.South},
		{0, 1, construction.North, construction.West},
		// no clear axis: the choice is kept (no flipping)
		{0, 0, construction.West, construction.West},
		{1, 1, construction.East, construction.East},
		{2, 2, construction.North, construction.North},
	}
	for _, c := range cases {
		if got := coolerFacing(c.h, c.v, c.prefer); got != c.want {
			t.Errorf("coolerFacing(%d,%d,%s) = %s, want %s", c.h, c.v, c.prefer, got, c.want)
		}
	}
}

// TestCoolerReplacesWallEndToEnd: a colonist takes the wall down and the
// cooler stands in its place, facing across the wall; the conduit under
// the wall stays.
func TestCoolerReplacesWallEndToEnd(t *testing.T) {
	t.Parallel()
	g := placementGame()
	for x := 4; x <= 8; x++ {
		buildNow(t, g, construction.Wall, item.Wood, construction.South, x, 4)
	}
	cond := buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, 6, 4)
	if _, err := g.SpawnNamedPawn(2, 8, "Ada"); err != nil {
		t.Fatal(err)
	}
	g.placeFrameStuff(construction.Cooler, item.Steel, construction.East, 6, 4)
	fid := g.layerFrameAt(6, 4, construction.LayerObject)
	fr, _ := g.frames.Get(fid)
	fr.Wood, fr.Comp = fr.Need, fr.CompNeed // hauled already: this test is about the replacement
	g.frames.Set(fid, fr)
	if !stepChunks(g, 3000, func() bool { _, ok := building(g, 6, 4, "cooler"); return ok }) {
		t.Fatalf("no cooler; frames %+v", g.World().Frames)
	}
	if _, ok := building(g, 6, 4, "wall"); ok {
		t.Fatal("the wall still stands where the cooler is")
	}
	if b, _ := building(g, 6, 4, "cooler"); b.Facing != "north" {
		t.Fatalf("cooler faces %q, want north (across the east-west wall)", b.Facing)
	}
	if conduitAt(g, 6, 4) != cond {
		t.Fatal("the conduit under the wall must survive the replacement")
	}
	if g.grid.Walkable(6, 4) {
		t.Fatal("the cooler blocks its cell like a wall")
	}
}

// TestTorchLampPlacesLikeALamp: the torch lamp goes anywhere a lamp does.
func TestTorchLampPlacesLikeALamp(t *testing.T) {
	t.Parallel()
	g := placementGame()
	g.setTerrain(2, 2, terrain.Rock)
	fillTerrain(g, 3, 2, 3, 2, terrain.ShallowWater)
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 5, 2)
	buildNow(t, g, construction.Workbench, item.Wood, construction.South, 8, 2)
	buildNow(t, g, construction.Chair, item.Wood, construction.South, 11, 2)
	buildNow(t, g, construction.Carpet, item.CarpetStraw, construction.South, 12, 2)
	ventAt(t, g, 14, 2)
	g.placeFrameStuff(construction.Lamp, item.Wood, construction.South, 16, 2)
	for x := 0; x < 20; x++ {
		for y := 0; y < 6; y++ {
			okL, rL := g.canPlaceStuffAt(construction.Lamp, item.Wood, construction.South, x, y)
			okT, rT := g.canPlaceStuffAt(construction.TorchLamp, item.Wood, construction.South, x, y)
			if okL != okT || (!okL && rL != rT) {
				t.Errorf("(%d,%d): lamp %v %q, torch lamp %v %q", x, y, okL, rL, okT, rT)
			}
		}
	}
	if ok, r := g.canPlaceStuffAt(construction.TorchLamp, item.Wood, construction.South, 9, 8); !ok {
		t.Fatalf("a torch lamp on open ground: %q", r)
	}
}

// TestElectricPiecesPlaceLikeObjects: the solar panel (3x3), the wind
// turbine and battery (1x2, rotatable), the heater and the generators take
// the ordinary object rules.
func TestElectricPiecesPlaceLikeObjects(t *testing.T) {
	t.Parallel()
	g := placementGame()
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 10, 5)
	solar, wind, battery := construction.SolarPanel, construction.WindTurbine, construction.Battery
	// a 3x3 panel covers nine cells
	id := buildNow(t, g, solar, item.Steel, construction.South, 2, 2)
	for x := 2; x <= 4; x++ {
		for y := 2; y <= 4; y++ {
			if g.layerBldgAt(x, y, construction.LayerObject) != id || g.grid.Walkable(x, y) {
				t.Fatalf("solar panel should cover %d,%d", x, y)
			}
		}
	}
	if g.layerBldgAt(5, 4, construction.LayerObject) != 0 || g.layerBldgAt(3, 5, construction.LayerObject) != 0 {
		t.Fatal("the panel covers more than 3x3")
	}
	for _, c := range []struct {
		k    construction.Kind
		f    construction.Facing
		x, y int
		want string // "" for placeable
	}{
		{solar, construction.South, 4, 4, ReasonOccupied}, // overlaps the panel
		{solar, construction.South, 8, 4, ReasonOccupied}, // the wall at 10,5 is in the footprint
		{solar, construction.South, 18, 4, ReasonOutside}, // hangs off the map
		{solar, construction.South, 14, 6, ""},
		{wind, construction.South, 14, 2, ""}, // 1x2 south: (14,2) (14,3)
		{wind, construction.East, 14, 2, ""},  // 2x1 east: (14,2) (15,2)
		{wind, construction.East, 19, 2, ReasonOutside},
		{wind, construction.South, 10, 4, ReasonOccupied}, // second cell on the wall
		{battery, construction.South, 16, 9, ""},
		{battery, construction.East, 12, 9, ""},
		{battery, construction.South, 3, 3, ReasonOccupied},
		{construction.Heater, construction.South, 16, 6, ""},
		{construction.WoodGenerator, construction.South, 16, 2, ""},
		{construction.PowerSwitch, construction.South, 17, 7, ""},
	} {
		ok, r := g.canPlaceStuffAt(c.k, item.Steel, c.f, c.x, c.y)
		if (c.want == "") != ok || (!ok && r != c.want) {
			t.Errorf("%s %s at %d,%d: %v %q, want %q", c.k, c.f, c.x, c.y, ok, r, c.want)
		}
	}
	// the wind turbine and battery turn: east covers two cells in a row
	wid := buildNow(t, g, wind, item.Steel, construction.East, 14, 2)
	if g.layerBldgAt(15, 2, construction.LayerObject) != wid || g.layerBldgAt(14, 3, construction.LayerObject) != 0 {
		t.Fatal("an east-facing turbine covers (14,2) and (15,2)")
	}
}
