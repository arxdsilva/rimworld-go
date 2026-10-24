package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/terrain"
)

// Auto-orientation of doors and fence gates (PLAY-011 §1): a door or gate
// spans the axis with more solid neighbours (walls, doors, fences, gates,
// built or planned, and rock); a tie keeps the current facing.

func orientGame(t *testing.T) *Game {
	t.Helper()
	return New(Config{Seed: 1, Width: 16, Height: 12})
}

// bldgFacing is the facing of the object-layer building on (x, y).
func bldgFacing(t *testing.T, g *Game, x, y int) construction.Facing {
	t.Helper()
	b, ok := g.bldgs.Get(g.layerBldgAt(x, y, construction.LayerObject))
	if !ok {
		t.Fatalf("no building at %d,%d", x, y)
	}
	return b.Facing
}

// unbuild removes the object-layer building on (x, y) and the refund it
// drops there, so the tile can be built again.
func unbuild(g *Game, x, y int) {
	g.removeBuilding(g.layerBldgAt(x, y, construction.LayerObject))
	for id := g.itemAt(x, y); id != 0; id = g.itemAt(x, y) {
		g.deleteItem(id)
	}
}

// frameFacing is the facing of the object-layer frame on (x, y).
func frameFacing(t *testing.T, g *Game, x, y int) construction.Facing {
	t.Helper()
	fr, ok := frameAtObj(g, x, y)
	if !ok {
		t.Fatalf("no frame at %d,%d", x, y)
	}
	return fr.Facing
}

func TestDoorFacesAlongWallRun(t *testing.T) {
	t.Parallel()
	g := orientGame(t)
	// horizontal run: walls west and east of the door
	placeBuilt(t, g, construction.Wall, construction.South, 4, 5)
	placeBuilt(t, g, construction.Wall, construction.South, 6, 5)
	g.placeFrameStuff(construction.Door, item.Wood, construction.East, 5, 5) // the player's facing loses to the run
	if f := frameFacing(t, g, 5, 5); f != construction.South {
		t.Fatalf("door in a horizontal run faces %v, want south", f)
	}
	// vertical run: walls north and south
	placeBuilt(t, g, construction.Wall, construction.South, 10, 4)
	placeBuilt(t, g, construction.Wall, construction.South, 10, 6)
	placeBuilt(t, g, construction.Door, construction.South, 10, 5)
	if f := bldgFacing(t, g, 10, 5); f != construction.East {
		t.Fatalf("door in a vertical run faces %v, want east", f)
	}
	if got := g.AutoFacingAt("door", "south", 10, 2); got != "south" {
		t.Fatalf("free cell AutoFacingAt = %q", got)
	}
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 13, 4)
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 13, 6)
	if got := g.AutoFacingAt("door", "south", 13, 5); got != "east" {
		t.Fatalf("AutoFacingAt between planned walls = %q, want east", got)
	}
	if got := g.AutoFacingAt("wall", "west", 13, 5); got != "west" {
		t.Fatalf("a wall keeps its facing, got %q", got)
	}
	if got := g.AutoFacingAt("castle", "west", 13, 5); got != "west" {
		t.Fatalf("unknown kind keeps the facing, got %q", got)
	}
}

func TestDoorBetweenFencesFacesAlongFence(t *testing.T) {
	t.Parallel()
	g := orientGame(t)
	placeBuilt(t, g, construction.Fence, construction.South, 5, 4)
	placeBuilt(t, g, construction.Fence, construction.South, 5, 6)
	g.placeFrameStuff(construction.Door, item.Wood, construction.South, 5, 5)
	if f := frameFacing(t, g, 5, 5); f != construction.East {
		t.Fatalf("door between north-south fences faces %v, want east", f)
	}
}

func TestDoorAtTJunctionPicksLongerRun(t *testing.T) {
	t.Parallel()
	g := orientGame(t)
	for _, c := range [][2]int{{4, 5}, {6, 5}, {5, 4}} { // west, east, north
		placeBuilt(t, g, construction.Wall, construction.South, c[0], c[1])
	}
	g.placeFrameStuff(construction.Door, item.Wood, construction.East, 5, 5)
	if f := frameFacing(t, g, 5, 5); f != construction.South {
		t.Fatalf("T-junction (2 across, 1 up) faces %v, want south", f)
	}
	for _, c := range [][2]int{{10, 4}, {10, 6}, {11, 5}} { // north, south, east
		placeBuilt(t, g, construction.Wall, construction.South, c[0], c[1])
	}
	g.placeFrameStuff(construction.Door, item.Wood, construction.South, 10, 5)
	if f := frameFacing(t, g, 10, 5); f != construction.East {
		t.Fatalf("T-junction (2 up, 1 across) faces %v, want east", f)
	}
}

func TestFreeStandingDoorKeepsPlayerFacing(t *testing.T) {
	t.Parallel()
	g := orientGame(t)
	placeBuilt(t, g, construction.Door, construction.East, 5, 5)
	if f := bldgFacing(t, g, 5, 5); f != construction.East {
		t.Fatalf("free-standing door faces %v, want the player's east", f)
	}
	placeBuilt(t, g, construction.Door, construction.South, 9, 5)
	if f := bldgFacing(t, g, 9, 5); f != construction.South {
		t.Fatalf("free-standing door faces %v, want south", f)
	}
}

func TestDoorPlacedFirstTurnsWhenWallsGoUp(t *testing.T) {
	t.Parallel()
	g := orientGame(t)
	// a door frame turns when wall frames are planned beside it
	g.placeFrameStuff(construction.Door, item.Wood, construction.South, 5, 5)
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 5, 4)
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 5, 6)
	if f := frameFacing(t, g, 5, 5); f != construction.East {
		t.Fatalf("door frame faces %v after walls were planned, want east", f)
	}
	// a built door turns when walls are built beside it
	placeBuilt(t, g, construction.Door, construction.South, 10, 5)
	placeBuilt(t, g, construction.Wall, construction.South, 10, 4)
	before := g.structVer
	placeBuilt(t, g, construction.Wall, construction.South, 10, 6)
	if f := bldgFacing(t, g, 10, 5); f != construction.East {
		t.Fatalf("built door faces %v after walls were built, want east", f)
	}
	if g.structVer == before {
		t.Fatal("a facing change must bump structVer")
	}
	// cancelling the planned walls hands the frame back to a tie: it keeps east
	g.cancelRect(5, 4, 5, 4)
	if f := frameFacing(t, g, 5, 5); f != construction.East {
		t.Fatalf("door frame faces %v after a wall was cancelled", f)
	}
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 4, 5)
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 6, 5)
	if f := frameFacing(t, g, 5, 5); f != construction.South {
		t.Fatalf("door frame faces %v with walls east and west, want south", f)
	}
	g.cancelFrame(g.layerFrameAt(4, 5, construction.LayerObject))
	g.cancelFrame(g.layerFrameAt(6, 5, construction.LayerObject))
	if f := frameFacing(t, g, 5, 5); f != construction.East {
		t.Fatalf("door frame faces %v once the east-west walls are cancelled, want east", f)
	}
}

func TestDoorInCrossOnlyFlipsOnRealMajority(t *testing.T) {
	t.Parallel()
	g := orientGame(t)
	placeBuilt(t, g, construction.Door, construction.South, 5, 5)
	for _, c := range [][2]int{{6, 5}, {4, 5}, {5, 4}, {5, 6}} { // E, W, then N, S
		placeBuilt(t, g, construction.Wall, construction.South, c[0], c[1])
	}
	if f := bldgFacing(t, g, 5, 5); f != construction.South {
		t.Fatalf("2/2 cross faces %v, want the tie kept (south)", f)
	}
	// one wall off the side the door does not span: still south
	unbuild(g, 5, 4)
	if f := bldgFacing(t, g, 5, 5); f != construction.South {
		t.Fatalf("removing the north wall flipped the door to %v", f)
	}
	placeBuilt(t, g, construction.Wall, construction.South, 5, 4) // back to the tie
	if f := bldgFacing(t, g, 5, 5); f != construction.South {
		t.Fatalf("restoring the tie flipped the door to %v", f)
	}
	// both walls of the east-west side gone: the door turns
	unbuild(g, 6, 5)
	unbuild(g, 4, 5)
	if f := bldgFacing(t, g, 5, 5); f != construction.East {
		t.Fatalf("door faces %v with only north and south walls, want east", f)
	}
}

func TestDoorNoFlapOnTieCell(t *testing.T) {
	t.Parallel()
	g := orientGame(t)
	// one wall east and one north: a tie, the player's south stays
	placeBuilt(t, g, construction.Wall, construction.South, 6, 5)
	placeBuilt(t, g, construction.Wall, construction.South, 5, 4)
	placeBuilt(t, g, construction.Door, construction.South, 5, 5)
	if f := bldgFacing(t, g, 5, 5); f != construction.South {
		t.Fatalf("tie cell faces %v, want south", f)
	}
	changes, last := 0, bldgFacing(t, g, 5, 5)
	for i := 0; i < 6; i++ {
		placeBuilt(t, g, construction.Wall, construction.South, 5, 6) // v=2 > h=1
		if f := bldgFacing(t, g, 5, 5); f != last {
			changes, last = changes+1, f
		}
		unbuild(g, 5, 6) // back to a tie
		if f := bldgFacing(t, g, 5, 5); f != last {
			changes, last = changes+1, f
		}
	}
	if changes != 1 || last != construction.East {
		t.Fatalf("facing changed %d times (last %v), want once to east", changes, last)
	}
}

func TestGateInVerticalFenceFacesEast(t *testing.T) {
	t.Parallel()
	g := orientGame(t)
	placeBuilt(t, g, construction.Fence, construction.South, 5, 4)
	placeBuilt(t, g, construction.Fence, construction.South, 5, 6)
	placeBuilt(t, g, construction.FenceGate, construction.South, 5, 5)
	if f := bldgFacing(t, g, 5, 5); f != construction.East {
		t.Fatalf("gate in a north-south fence faces %v, want east", f)
	}
	placeBuilt(t, g, construction.Fence, construction.South, 9, 5)
	placeBuilt(t, g, construction.Fence, construction.South, 11, 5)
	placeBuilt(t, g, construction.FenceGate, construction.North, 10, 5)
	if f := bldgFacing(t, g, 10, 5); f != construction.South {
		t.Fatalf("gate in an east-west fence faces %v, want south", f)
	}
}

func TestDoorInRockCliffOrients(t *testing.T) {
	t.Parallel()
	g := orientGame(t)
	g.setTerrain(5, 4, terrain.Rock)
	g.setTerrain(5, 6, terrain.Rock)
	placeBuilt(t, g, construction.Door, construction.South, 5, 5)
	if f := bldgFacing(t, g, 5, 5); f != construction.East {
		t.Fatalf("door in a north-south cliff faces %v, want east", f)
	}
	// rock east and west appears, the north rock is mined away: south
	g.setTerrain(4, 5, terrain.Rock)
	g.setTerrain(6, 5, terrain.Rock)
	if f := bldgFacing(t, g, 5, 5); f != construction.East {
		t.Fatalf("2/2 rock tie flipped the door to %v", f)
	}
	g.setTerrain(5, 4, terrain.Stone)
	if f := bldgFacing(t, g, 5, 5); f != construction.South {
		t.Fatalf("door faces %v after the north rock was mined, want south", f)
	}
}

func TestOldSaveOrientsDoorsKeepsGates(t *testing.T) {
	t.Parallel()
	g := orientGame(t)
	placeBuilt(t, g, construction.Wall, construction.South, 5, 4)
	placeBuilt(t, g, construction.Wall, construction.South, 5, 6)
	placeBuilt(t, g, construction.Door, construction.South, 5, 5)
	placeBuilt(t, g, construction.Fence, construction.South, 9, 4)
	placeBuilt(t, g, construction.Fence, construction.South, 9, 6)
	placeBuilt(t, g, construction.FenceGate, construction.South, 9, 5)
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 12, 4)
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 12, 6)
	g.placeFrameStuff(construction.Door, item.Wood, construction.South, 12, 5)
	// an old save stored every door (built or planned) facing south, and
	// the gate as the player left it
	setFacing := func(x, y int, f construction.Facing) {
		if bid := g.layerBldgAt(x, y, construction.LayerObject); bid != 0 {
			b, _ := g.bldgs.Get(bid)
			b.Facing = f
			g.bldgs.Set(bid, b)
		}
		if fid := g.layerFrameAt(x, y, construction.LayerObject); fid != 0 {
			fr, _ := g.frames.Get(fid)
			fr.Facing = f
			g.frames.Set(fid, fr)
		}
	}
	setFacing(5, 5, construction.South)
	setFacing(9, 5, construction.South)
	setFacing(12, 5, construction.South)
	var buf bytes.Buffer
	if err := g.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	// a new save is marked oriented and loads unchanged
	saved := false
	_ = downgradeSave(t, buf.Bytes(), func(s *saveState) { saved = s.DoorsOriented })
	if !saved {
		t.Fatal("a new save must write DoorsOriented")
	}
	l, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if f := bldgFacing(t, l, 5, 5); f != construction.South {
		t.Fatalf("a new save re-oriented its door to %v", f)
	}
	old := downgradeSave(t, buf.Bytes(), func(s *saveState) { s.DoorsOriented = false })
	l, _, err = Load(bytes.NewReader(old))
	if err != nil {
		t.Fatal(err)
	}
	if f := bldgFacing(t, l, 5, 5); f != construction.East {
		t.Fatalf("old save door faces %v, want east", f)
	}
	if f := frameFacing(t, l, 12, 5); f != construction.East {
		t.Fatalf("old save door frame faces %v, want east", f)
	}
	if f := bldgFacing(t, l, 9, 5); f != construction.South {
		t.Fatalf("old save gate faces %v, want its stored south", f)
	}
}
