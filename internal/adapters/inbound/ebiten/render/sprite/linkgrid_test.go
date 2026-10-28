package sprite

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/application/game"
)

// testKinds classifies the kinds these tests build like the renderer
// does: walls, doors and coolers on the wall layer, fences and gates on
// the fence layer, conduits and devices on the power layer.
type testKinds struct{}

func (testKinds) Building(kind string) LinkCell {
	switch kind {
	case "wall", "door", "cooler":
		return WallLink
	case FenceKind, GateKind:
		return FenceLink
	}
	return LinkCell{}
}

func (testKinds) Power(kind string) LinkCell {
	switch kind {
	case "power conduit", "hidden conduit":
		return ConduitLink
	case "power switch":
		return LinkCell{Flags: LinkConduit}
	case "battery", "cooler":
		return LinkCell{Flags: LinkDevice}
	}
	return LinkCell{}
}

type testGrid = LinkGrid[testKinds]

// linkWorld builds a read model from rows of cells: R rock, S smoothed
// rock, W wall, D door, F fence, G fence gate, c a stone chunk on soil,
// anything else soil. StructVer is ver.
func linkWorld(t *testing.T, ver uint64, rows ...string) (game.WorldView, int, int) {
	t.Helper()
	code := map[string]uint8{}
	for i, n := range game.TerrainNames() {
		code[n] = uint8(i)
	}
	h, wd := len(rows), len(rows[0])
	w := game.WorldView{TerrainW: wd, Terrain: make([]uint8, wd*h), StructVer: ver}
	for y, row := range rows {
		for x, ch := range row {
			i := y*wd + x
			w.Terrain[i] = code["soil"]
			switch ch {
			case 'R':
				w.Terrain[i] = code["rock"]
			case 'S':
				w.Terrain[i] = code["rock_smooth"]
			case 'W':
				w.Buildings = append(w.Buildings, game.BuildingView{Kind: "wall", Stuff: "granite blocks", X: x, Y: y, W: 1, H: 1})
			case 'D':
				w.Buildings = append(w.Buildings, game.BuildingView{Kind: "door", Stuff: "wood", X: x, Y: y, W: 1, H: 1})
			case 'F':
				w.Buildings = append(w.Buildings, game.BuildingView{Kind: "fence", Stuff: "wood", X: x, Y: y, W: 1, H: 1})
			case 'G':
				w.Buildings = append(w.Buildings, game.BuildingView{Kind: "fence gate", Stuff: "wood", X: x, Y: y, W: 1, H: 1})
			case 'c':
				w.Items = append(w.Items, game.ItemView{Kind: "granite chunk", Qty: 1, X: x, Y: y})
			}
		}
	}
	return w, wd, h
}

func syncedGrid(t *testing.T, rows ...string) *testGrid {
	t.Helper()
	w, wd, h := linkWorld(t, 1, rows...)
	g := &testGrid{}
	g.Sync(w, wd, h)
	return g
}

func TestMaskAll16(t *testing.T) {
	t.Parallel()
	// a wall in the middle of a 3x3, with each subset of its four
	// cardinal neighbours walled, has exactly that subset as its mask
	for m := 0; m < 16; m++ {
		rows := []byte("........." /* 3x3 flattened */)
		rows[4] = 'W'
		if m&art.WallN != 0 {
			rows[1] = 'W'
		}
		if m&art.WallE != 0 {
			rows[5] = 'W'
		}
		if m&art.WallS != 0 {
			rows[7] = 'W'
		}
		if m&art.WallW != 0 {
			rows[3] = 'W'
		}
		g := syncedGrid(t, string(rows[0:3]), string(rows[3:6]), string(rows[6:9]))
		if got := g.Mask(1, 1); got != m {
			t.Fatalf("mask %d: got %d", m, got)
		}
		if g.Corners(1, 1) != 0 {
			t.Fatalf("mask %d: no diagonals, no corners, got %d", m, g.Corners(1, 1))
		}
	}
}

func TestWallLinksRock(t *testing.T) {
	t.Parallel()
	g := syncedGrid(t,
		".....",
		".RWS.",
		".....")
	if got := g.Mask(2, 1); got != art.WallE|art.WallW {
		t.Fatalf("wall between rough and smooth rock: mask %d", got)
	}
	if !g.Linked(2, 1, 1, 1) || !g.Linked(2, 1, 3, 1) {
		t.Fatal("a wall links to natural rock both ways")
	}
}

func TestRockLinksWall(t *testing.T) {
	t.Parallel()
	g := syncedGrid(t,
		".....",
		".RW..",
		".....")
	if got := g.Mask(1, 1); got != art.WallE {
		t.Fatalf("rock beside a wall: mask %d, want E", got)
	}
	g = syncedGrid(t,
		".....",
		".RD..",
		".....")
	if got := g.Mask(1, 1); got != art.WallE {
		t.Fatalf("rock beside a door: mask %d, want E", got)
	}
}

func TestFenceOneWay(t *testing.T) {
	t.Parallel()
	g := syncedGrid(t,
		".....",
		".FWG.",
		".....")
	if got := g.Mask(1, 1); got != art.WallE {
		t.Fatalf("fence joins the wall: mask %d", got)
	}
	if got := g.Mask(2, 1); got != 0 {
		t.Fatalf("a wall never joins fences: mask %d", got)
	}
	if got := g.Mask(3, 1); got != art.WallW {
		t.Fatalf("gate joins the wall: mask %d", got)
	}
	g = syncedGrid(t,
		".....",
		".FRF.",
		".....")
	if g.Mask(1, 1) != 0 || g.Mask(2, 1) != 0 {
		t.Fatal("fences and rock do not link")
	}
}

func TestDiagonalOnlyNoLink(t *testing.T) {
	t.Parallel()
	g := syncedGrid(t,
		"....",
		".W..",
		"..W.",
		"....")
	if g.Mask(1, 1) != 0 || g.Mask(2, 2) != 0 {
		t.Fatal("diagonal walls do not link")
	}
	if g.Corners(1, 1) != 0 || g.Corners(2, 2) != 0 {
		t.Fatal("diagonal walls fill no corner")
	}
}

func TestCorners2x2(t *testing.T) {
	t.Parallel()
	g := syncedGrid(t,
		"....",
		".WW.",
		".WW.",
		"....")
	want := map[[2]int][2]int{ // cell -> mask, corners
		{1, 1}: {art.WallE | art.WallS, CornerSE},
		{2, 1}: {art.WallW | art.WallS, CornerSW},
		{1, 2}: {art.WallN | art.WallE, CornerNE},
		{2, 2}: {art.WallN | art.WallW, CornerNW},
	}
	for c, mc := range want {
		if m, k := g.Mask(c[0], c[1]), g.Corners(c[0], c[1]); m != mc[0] || k != mc[1] {
			t.Fatalf("%v: mask %d corners %d, want %d %d", c, m, k, mc[0], mc[1])
		}
	}
	if got := LinkedKey("structure/wall_stone", g.Mask(1, 1), g.Corners(1, 1)); got != "structure/wall_stone_06_c2" {
		t.Fatalf("top-left of a 2x2 = %s", got)
	}
}

func TestCornerNoDiagonal(t *testing.T) {
	t.Parallel()
	// an L: the corner cell links N and E but the NE diagonal is open
	g := syncedGrid(t,
		".W..",
		".WW.",
		"....")
	if m, k := g.Mask(1, 1), g.Corners(1, 1); m != art.WallN|art.WallE || k != 0 {
		t.Fatalf("L corner: mask %d corners %d", m, k)
	}
	if got := LinkedKey("structure/wall_stone", 3, 0); got != "structure/wall_stone_03" {
		t.Fatalf("notched corner key = %s", got)
	}
}

func TestCornerDiagonalOnly(t *testing.T) {
	t.Parallel()
	// only the diagonal is walled: no cardinals, no corner
	g := syncedGrid(t,
		"..W.",
		".W..",
		"....")
	if g.Mask(1, 1) != 0 || g.Corners(1, 1) != 0 {
		t.Fatalf("diagonal only: mask %d corners %d", g.Mask(1, 1), g.Corners(1, 1))
	}
	// N linked and the diagonal set, but E open: still no NE corner
	g = syncedGrid(t,
		".WW.",
		".W..",
		"....")
	if g.Corners(1, 1) != 0 {
		t.Fatalf("one cardinal + diagonal: corners %d", g.Corners(1, 1))
	}
}

func TestMapEdgeRockLinksWallDoesNot(t *testing.T) {
	t.Parallel()
	g := syncedGrid(t,
		"R..W",
		"....",
		"W..R")
	if got := g.Mask(0, 0); got != art.WallN|art.WallW {
		t.Fatalf("corner rock: mask %d, want N|W", got)
	}
	if got := g.Mask(3, 2); got != art.WallS|art.WallE {
		t.Fatalf("corner rock: mask %d, want S|E", got)
	}
	if g.Mask(3, 0) != 0 || g.Mask(0, 2) != 0 {
		t.Fatal("walls never link off the map")
	}
}

func TestMapEdgeCorners(t *testing.T) {
	t.Parallel()
	g := syncedGrid(t,
		"RR.",
		"RR.",
		"...")
	// top-left rock: N, W off the map, E and S rock, so every corner
	// with linked cardinals and a linked diagonal fills
	if m, k := g.Mask(0, 0), g.Corners(0, 0); m != 15 || k != CornerNE|CornerSW|CornerNW|CornerSE {
		t.Fatalf("map-corner rock: mask %d corners %d", m, k)
	}
	if got := LinkedKey("rock/rock_wall", 15, 15); got != "rock/rock_wall_15_cf" {
		t.Fatalf("solid key = %s", got)
	}
	// (1, 0): N off map, W rock, S rock, E open: NW (off map) and SW fill
	if m, k := g.Mask(1, 0), g.Corners(1, 0); m != art.WallN|art.WallS|art.WallW || k != CornerNW|CornerSW {
		t.Fatalf("top edge rock: mask %d corners %d", m, k)
	}
}

func TestMineCentreOf3x3(t *testing.T) {
	t.Parallel()
	w, wd, h := linkWorld(t, 1,
		".....",
		".RRR.",
		".RRR.",
		".RRR.",
		".....")
	g := &testGrid{}
	g.Sync(w, wd, h)
	before := map[[2]int][2]int{}
	for y := 1; y <= 3; y++ {
		for x := 1; x <= 3; x++ {
			before[[2]int{x, y}] = [2]int{g.Mask(x, y), g.Corners(x, y)}
		}
	}
	// mine the centre: the sim changes the terrain and bumps StructVer
	soil := w.Terrain[0]
	terr := append([]uint8(nil), w.Terrain...)
	terr[2*wd+2] = soil
	w.Terrain, w.StructVer = terr, 2
	g.Sync(w, wd, h)
	card := map[[2]int]int{{2, 1}: art.WallS, {3, 2}: art.WallW, {2, 3}: art.WallN, {1, 2}: art.WallE}
	for c, bit := range card {
		if got := g.Mask(c[0], c[1]); got != before[c][0]&^bit {
			t.Fatalf("%v lost bit %d: mask %d -> %d", c, bit, before[c][0], got)
		}
	}
	diag := map[[2]int]int{{1, 1}: CornerSE, {3, 1}: CornerSW, {3, 3}: CornerNW, {1, 3}: CornerNE}
	for c, bit := range diag {
		if before[c][1]&bit == 0 {
			t.Fatalf("%v should have had corner %d before", c, bit)
		}
		if got := g.Corners(c[0], c[1]); got != before[c][1]&^bit {
			t.Fatalf("%v lost corner %d: %d -> %d", c, bit, before[c][1], got)
		}
		if got := g.Mask(c[0], c[1]); got != before[c][0] {
			t.Fatalf("%v diagonal mine kept the mask: %d -> %d", c, before[c][0], got)
		}
	}
}

func TestChunksNeverLink(t *testing.T) {
	t.Parallel()
	g := syncedGrid(t,
		".....",
		".RcW.",
		".....")
	if g.Mask(1, 1) != 0 || g.Mask(3, 1) != 0 || g.Mask(2, 1) != 0 {
		t.Fatalf("a chunk joins nothing: %d %d %d", g.Mask(1, 1), g.Mask(2, 1), g.Mask(3, 1))
	}
}

func TestLinkedKeyFiltersIrrelevantCorners(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mask, corners int
		want          string
	}{
		{0, 15, "rock/rock_wall_00"},
		{art.WallN | art.WallS, 15, "rock/rock_wall_05"}, // no two adjacent cardinals
		{art.WallN | art.WallE, CornerSW | CornerNE, "rock/rock_wall_03_c1"},
		{art.WallN | art.WallE | art.WallS, 15, "rock/rock_wall_07_c3"}, // NE, SE only
		{15, CornerNW, "rock/rock_wall_15_c8"},
		{15, 0, "rock/rock_wall_15"},
	}
	for _, c := range cases {
		if got := LinkedKey("rock/rock_wall", c.mask, c.corners); got != c.want {
			t.Fatalf("LinkedKey(%d, %d) = %s, want %s", c.mask, c.corners, got, c.want)
		}
	}
	// every family has exactly 47 distinct keys
	seen := map[string]bool{}
	for m := 0; m < 16; m++ {
		for k := 0; k < 16; k++ {
			seen[LinkedKey("p", m, k)] = true
		}
	}
	if len(seen) != 47 {
		t.Fatalf("%d variants, want 47", len(seen))
	}
}

func TestLinkGridRebuildOnlyOnVersion(t *testing.T) {
	w, wd, h := linkWorld(t, 5,
		".....",
		".WW..",
		".....")
	g := &testGrid{}
	g.Sync(w, wd, h)
	if g.Mask(1, 1) != art.WallE {
		t.Fatal("first sync builds")
	}
	if n := testing.AllocsPerRun(100, func() { g.Sync(w, wd, h) }); n != 0 {
		t.Fatalf("unchanged version allocates %v per sync", n)
	}
	// a changed world with the same version is not re-read
	w2, _, _ := linkWorld(t, 5,
		".....",
		".W...",
		".....")
	g.Sync(w2, wd, h)
	if g.Mask(1, 1) != art.WallE {
		t.Fatal("same version must not rebuild")
	}
	w2.StructVer = 6
	g.Sync(w2, wd, h)
	if g.Mask(1, 1) != 0 {
		t.Fatal("a new version rebuilds")
	}
	// rebuilding at the same size reuses the cells
	w.StructVer = 7
	if n := testing.AllocsPerRun(20, func() { w.StructVer++; g.Sync(w, wd, h) }); n != 0 {
		t.Fatalf("rebuild allocates %v", n)
	}
}

func TestLinkedPlannedFramesJoinWalls(t *testing.T) {
	t.Parallel()
	w, wd, h := linkWorld(t, 1,
		".....",
		".RW..",
		".....")
	w.Frames = []game.FrameView{{Kind: "wall", Stuff: "wood", X: 3, Y: 1, W: 1, H: 1}, {Kind: "wall", X: 3, Y: 2, W: 1, H: 1}}
	g := &testGrid{}
	g.Sync(w, wd, h)
	p := g.Plan(w.Frames, nil)
	if got := g.PlannedMask(p, LinkWall|LinkRock, 3, 1); got != art.WallW|art.WallS {
		t.Fatalf("frame joins the built wall and the frame below: %d", got)
	}
	if g.Mask(2, 1) != art.WallW {
		t.Fatal("built walls ignore frames")
	}
}

func TestPlannedCornersFillAcrossFramesAndWalls(t *testing.T) {
	t.Parallel()
	// a built wall at (1, 1); frames at (2, 1) and (2, 2); a ghost at (1, 2)
	w, wd, h := linkWorld(t, 1,
		"....",
		".W..",
		"....",
		"....")
	w.Frames = []game.FrameView{{Kind: "wall", X: 2, Y: 1}, {Kind: "wall", X: 2, Y: 2}}
	g := &testGrid{}
	g.Sync(w, wd, h)
	p := g.Plan(w.Frames, []PlannedCell{{X: 1, Y: 2, Flags: WallLink.Flags}})
	if m, k := g.PlannedMask(p, WallLink.Accepts, 1, 2), g.PlannedCorners(p, WallLink.Accepts, 1, 2); m != art.WallN|art.WallE || k != CornerNE {
		t.Fatalf("ghost in a 2x2 of wall, frames and ghost: mask %d corners %d", m, k)
	}
	if k := g.PlannedCorners(p, WallLink.Accepts, 2, 1); k != CornerSW {
		t.Fatalf("frame corner = %d", k)
	}
	// the plan is rebuilt each call: without the ghost the corner opens
	p = g.Plan(w.Frames, nil)
	if k := g.PlannedCorners(p, WallLink.Accepts, 2, 1); k != 0 {
		t.Fatalf("stale planned set: corner %d", k)
	}
}
