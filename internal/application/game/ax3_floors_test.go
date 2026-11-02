package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/room"
)

// TestCarpetFloorKeepsItsColour (AX-3): DesignateBuild's Index is the
// carpet floor's colour, from blueprint to building, in both views.
func TestCarpetFloorKeepsItsColour(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	g.Enqueue(command.Command{Kind: command.DesignateBuild, X: 4, Y: 4, Build: uint8(construction.CarpetFloor), Stuff: uint8(item.Cloth), Index: 49})
	g.Step(1)
	fid, fr, ok := frameOf(g, 4, 4, construction.LayerFloor)
	if !ok || fr.Tint != 49 {
		t.Fatalf("carpet floor blueprint %+v", fr)
	}
	for _, v := range g.World().Frames {
		if v.ID == fid && v.AX.Colour != construction.CarpetColours[49] {
			t.Fatalf("frame view colour %q", v.AX.Colour)
		}
	}
	g.completeFrame(fid, fr)
	b, _ := g.bldgs.Get(g.layerBldgAt(4, 4, construction.LayerFloor))
	if b.Kind != construction.CarpetFloor || b.Tint != 49 {
		t.Fatalf("carpet floor %+v", b)
	}
	for _, v := range g.World().Buildings {
		if v.Kind == "carpet floor" && v.AX.Colour != "powder blue" {
			t.Fatalf("building view colour %q", v.AX.Colour)
		}
	}
	// an index off the palette leaves a new blueprint in the first colour
	g.Enqueue(command.Command{Kind: command.DesignateBuild, X: 6, Y: 4, Build: uint8(construction.CarpetFloor), Stuff: uint8(item.Cloth), Index: 400})
	g.Step(1)
	if _, fr, ok := frameOf(g, 6, 4, construction.LayerFloor); !ok || fr.Tint != 0 {
		t.Fatalf("off-palette blueprint %+v", fr)
	}
}

// TestFloorsTakeTheirMaterials (AX-3): the wood floor is wood only, stone
// tiles and flagstones take any stone blocks, the metal and fibre floors
// their one material, and every wiki floor's work is the same in any of
// them.
func TestFloorsTakeTheirMaterials(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	for _, c := range []struct {
		kind, stuff string
		ok          bool
	}{
		{"floor", "granite blocks", false},
		{"floor", "wood", true},
		{"stone tile", "marble blocks", true},
		{"flagstone", "slate blocks", true},
		{"flagstone", "wood", false},
		{"straw matting", "hay", true},
		{"gold tile", "silver", false},
		{"sterile tile", "steel", true},
		{"carpet floor", "cloth", true},
	} {
		if ok, why := g.CanPlaceStuff(c.kind, c.stuff, 3, 3); ok != c.ok {
			t.Errorf("%s of %s: %v (%s), want %v", c.kind, c.stuff, ok, why, c.ok)
		}
	}
	if !g.BuildNow("stone tile", "marble blocks", "south", 3, 3) || !g.BuildNow("flagstone", "sandstone blocks", "south", 4, 3) {
		t.Fatal("stone floors not built")
	}
	if construction.WorkFor(construction.StoneTile, item.MarbleBlocks) != 110 || construction.WorkFor(construction.Concrete, item.Steel) != 10 {
		t.Fatal("wiki floor work scaled by material")
	}
}

// TestStoneFloorsMigrateToStoneTiles (AX-3, save v11): a wood floor of
// stone blocks, built or planned, becomes a stone tile; legacy stone is
// granite; wood floors stay.
func TestStoneFloorsMigrateToStoneTiles(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	g.placeGenerated(construction.Building{Kind: construction.Floor, Stuff: item.SlateBlocks, X: 2, Y: 2})
	g.placeGenerated(construction.Building{Kind: construction.Floor, Stuff: item.Stone, X: 3, Y: 2})
	g.placeGenerated(construction.Building{Kind: construction.Floor, Stuff: item.Wood, X: 4, Y: 2})
	fid := g.entities.Next()
	g.frameIDs = append(g.frameIDs, fid)
	g.frames.Set(fid, construction.Frame{Kind: construction.Floor, Stuff: item.MarbleBlocks, X: 5, Y: 2, Need: 1})
	g.migrateSave(10)
	for x, want := range map[int][2]any{2: {construction.StoneTile, item.SlateBlocks}, 3: {construction.StoneTile, item.GraniteBlocks},
		4: {construction.Floor, item.Wood}} {
		b, _ := g.bldgs.Get(g.layerBldgAt(x, 2, construction.LayerFloor))
		if b.Kind != want[0] || b.Stuff != want[1] {
			t.Errorf("floor at %d: %v of %v, want %v", x, b.Kind, b.Stuff, want)
		}
	}
	if fr, _ := g.frames.Get(fid); fr.Kind != construction.StoneTile || fr.Stuff != item.MarbleBlocks {
		t.Errorf("planned stone floor %+v", fr)
	}
}

// TestSterileTileRoom (AX-3): a clean hut laid in sterile tile gives the
// room cleanliness +0.6, cuts the infection factor to 30 % and raises the
// surgery cleanliness; the same hut in wood halves the chance and reads 0.
func TestSterileTileRoom(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		kind, stuff string
		factor      int32
		clean       int
		surgY       int32
	}{
		{"floor", "wood", infection.RoomClean, 0, 0},
		{"sterile tile", "steel", infection.RoomSterile, 60, 60},
	} {
		g := New(Config{Seed: 1, Width: 16, Height: 12})
		id, _ := g.SpawnNamedPawn(9, 5, "Ada")
		hut(t, g, 6, 2, 12, 8)
		for y := 3; y < 8; y++ {
			for x := 7; x < 12; x++ {
				if !g.BuildNow(c.kind, c.stuff, "south", x, y) {
					t.Fatalf("%s at %d,%d", c.kind, x, y)
				}
			}
		}
		g.warmIndexes(true)
		if f := g.roomInfectionFactor(id); f != c.factor {
			t.Errorf("%s: infection factor %d, want %d", c.kind, f, c.factor)
		}
		r := g.roomAt(9, 5)
		if got := g.rooms[r].tally.Inputs(g.rooms[r].contents.Cells, 0, 0, 0).CleanlinessHundredths; got != c.clean {
			t.Errorf("%s: room cleanliness %d, want %d", c.kind, got, c.clean)
		}
		if got, out := g.surgeryCleanliness(9, 5); out || got != c.surgY {
			t.Errorf("%s: surgery cleanliness %d (outdoors %v), want %d", c.kind, got, out, c.surgY)
		}
	}
}

// TestFloorBeautyInRooms (AX-3): a gold-tiled room is far more beautiful
// than a wood one, through the same tally the room stats read.
func TestFloorBeautyInRooms(t *testing.T) {
	t.Parallel()
	beauty := func(k construction.Kind, stuff item.Kind) int {
		g := New(Config{Seed: 1, Width: 16, Height: 12})
		hut(t, g, 6, 2, 12, 8)
		for y := 3; y < 8; y++ {
			for x := 7; x < 12; x++ {
				g.placeGenerated(construction.Building{Kind: k, Stuff: stuff, X: x, Y: y})
			}
		}
		g.warmIndexes(true)
		return g.rooms[g.roomAt(9, 5)].tally.BeautyTenths
	}
	if w, gold := beauty(construction.Floor, item.Wood), beauty(construction.GoldTile, item.Gold); w != 0 || gold != 25*110 {
		t.Fatalf("floor beauty: wood %d, gold %d", w, gold)
	}
	if b, c := room.FloorStats(construction.Flagstone, item.GraniteBlocks); b != 0 || c != 0 {
		t.Fatalf("flagstone %d %d", b, c)
	}
}

// TestFloorOverFloorAndCarpetRepaint (AX-3): a floor laid over another
// floor swaps it in place once built (the old one refunded as a
// deconstruction would), and a carpet floor laid again in another colour
// repaints it; the same floor in the same material is the neutral "same".
func TestFloorOverFloorAndCarpetRepaint(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	if !g.BuildNow("floor", "wood", "south", 4, 4) || !g.BuildNow("steel tile", "steel", "south", 6, 4) {
		t.Fatal("floors not built")
	}
	fl := g.layerBldgAt(4, 4, construction.LayerFloor)
	if ok, r := g.CanPlaceStuff("sterile tile", "steel", 4, 4); !ok {
		t.Fatalf("sterile tile over wood: %q", r)
	}
	if got := g.ReplacesAt("sterile tile", "steel", 4, 4); got != "wood floor → sterile tile" {
		t.Fatalf("ReplacesAt %q", got)
	}
	if ok, r := g.CanPlaceStuff("steel tile", "steel", 6, 4); ok || r != ReasonSame {
		t.Fatalf("steel tile over steel tile: %v %q", ok, r)
	}
	g.placeFrameStuff(construction.SterileTile, item.Steel, construction.South, 4, 4)
	fid, fr := swapFrameOn(t, g, 4, 4, construction.LayerFloor)
	if fr.Swaps != fl {
		t.Fatalf("floor swap frame %+v", fr)
	}
	finishFrame(g, fid)
	if b, _ := g.bldgs.Get(fl); b.Kind != construction.SterileTile || b.Stuff != item.Steel || g.layerBldgAt(4, 4, construction.LayerFloor) != fl {
		t.Fatalf("floor after the swap %+v", b)
	}

	// a carpet floor repainted from crimson (0) to mint
	g.placeGenerated(construction.Building{Kind: construction.CarpetFloor, Stuff: item.Cloth, X: 8, Y: 4})
	cid := g.layerBldgAt(8, 4, construction.LayerFloor)
	mint, _ := construction.ParseColour("mint")
	g.Enqueue(command.Command{Kind: command.DesignateBuild, X: 8, Y: 4, Build: uint8(construction.CarpetFloor), Stuff: uint8(item.Cloth), Index: int(mint)})
	g.Step(1)
	cfid, cf := swapFrameOn(t, g, 8, 4, construction.LayerFloor)
	if cf.Swaps != cid || cf.Tint != mint {
		t.Fatalf("repaint frame %+v", cf)
	}
	for _, v := range g.World().Buildings {
		if v.ID == cid && v.SwapTo != "carpet floor" {
			t.Fatalf("pane swap line %q", v.SwapTo)
		}
	}
	finishFrame(g, cfid)
	if b, _ := g.bldgs.Get(cid); b.Kind != construction.CarpetFloor || b.Tint != mint {
		t.Fatalf("carpet after the repaint %+v", b)
	}
}

// TestFloorsAreQuickerThanSoil (AX-3, wiki: Floors, Terrain): a step on a
// built floor takes 13/15 of the plain-ground ticks, on straw matting
// 14/15; bare soil keeps them all.
func TestFloorsAreQuickerThanSoil(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	g.placeGenerated(construction.Building{Kind: construction.StoneTile, Stuff: item.GraniteBlocks, X: 3, Y: 3})
	g.placeGenerated(construction.Building{Kind: construction.StrawMatting, Stuff: item.Hay, X: 4, Y: 3})
	g.warmIndexes(true)
	for _, c := range []struct{ x, base, want int }{
		{5, TicksPerCell, TicksPerCell}, {3, TicksPerCell, 13}, {4, TicksPerCell, 14},
		{5, TicksPerCellDiag, TicksPerCellDiag}, {3, TicksPerCellDiag, 18}, {4, TicksPerCellDiag, 20},
	} {
		if got := g.weatherSlow(c.x, 3, c.base); got != c.want {
			t.Errorf("step of %d ticks at %d,3: %d, want %d", c.base, c.x, got, c.want)
		}
	}
}
