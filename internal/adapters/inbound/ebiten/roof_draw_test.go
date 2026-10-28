package ebiten

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// roofLabelWorld is a 10x6 map: tiles x<5 are room 0 (small), x>=5 room
// 1 (large); column 4 of row 0 is a wall.
func roofLabelWorld() game.WorldView {
	w := game.WorldView{TerrainW: 10, RoomGrid: make([]int, 60), RoofCells: make([]uint8, 60),
		Rooms: []game.RoomView{{ID: 0, Cells: 20}, {ID: 1, Cells: 151}}}
	for i := range w.RoomGrid {
		if i%10 >= 5 {
			w.RoomGrid[i] = 1
		}
	}
	w.RoomGrid[4] = -2
	w.RoofCells[4] = roofBuilt // a roof flag on a wall still reads nothing
	w.RoofCells[10] = roofBuilt | roofPlanRemove
	w.RoofCells[11] = roofPlanBuild
	w.RoofCells[12] = roofNatural
	w.RoofCells[13] = roofBuilt
	w.RoofCells[20] = roofNatural | roofThick
	return w
}

func TestRoomLabelRoofedPercent(t *testing.T) {
	t.Parallel()
	w := game.WorldView{TerrainW: 2, RoomGrid: []int{0, 1}, Rooms: []game.RoomView{
		{ID: 0, Role: "bedroom", Label: "decent", Score: 12, Roofed: 1},
		{ID: 1, Role: "barn", Label: "dull", Score: 3, Lit: true, Roofed: 0.426},
	}}
	if got := roomLabelAt(w, 0, 0); !strings.HasSuffix(got, ", roofed 100%") || !strings.HasPrefix(got, "Room: bedroom") {
		t.Errorf("full roof: %q", got)
	}
	if got := roomLabelAt(w, 1, 0); !strings.HasSuffix(got, ", lit, roofed 43%") {
		t.Errorf("partial roof: %q", got)
	}
}

// eaveWorld: a 7x6 room walled on every side (walls at x=0,6 and y=0,5),
// its interior roofed, and one roofed tile outside it.
func eaveWorld() game.WorldView {
	w := game.WorldView{TerrainW: 9, RoomGrid: make([]int, 9*6), RoofCells: make([]uint8, 9*6)}
	for y := 0; y < 6; y++ {
		for x := 0; x < 9; x++ {
			i := y*9 + x
			switch {
			case x >= 7:
				w.RoomGrid[i] = -1
			case x == 0 || x == 6 || y == 0 || y == 5:
				w.RoomGrid[i] = -2
			default:
				w.RoomGrid[i] = 0
				w.RoofCells[i] = roofBuilt
			}
		}
	}
	w.RoofCells[3*9+3] = 0 // an unroofed hole: no eave from it
	w.RoofCells[2*9+8] = roofBuilt
	return w
}

func TestEavesOnRoofedInteriorEdges(t *testing.T) {
	t.Parallel()
	w := eaveWorld()
	edges := eaveEdges(w, 0, 0, 8, 5, nil)
	has := map[eaveEdge]bool{}
	for _, e := range edges {
		has[e] = true
	}
	for _, want := range []eaveEdge{{X: 1, Y: 1, Side: sideN}, {X: 1, Y: 1, Side: sideW}, {X: 5, Y: 4, Side: sideE}, {X: 5, Y: 4, Side: sideS}, {X: 3, Y: 1, Side: sideN}, {X: 1, Y: 3, Side: sideW}} {
		if !has[want] {
			t.Errorf("missing eave %+v", want)
		}
	}
	for _, e := range edges {
		if e.X == 3 && e.Y == 3 {
			t.Errorf("eave on an unroofed tile: %+v", e)
		}
		if roomIDAt(w, e.X, e.Y) == -2 {
			t.Errorf("eave on a wall: %+v", e)
		}
	}
	// interior only: perimeter tiles of the 5x4 interior, one per wall side
	if len(edges) != 2*5+2*4+0 {
		t.Errorf("got %d eave edges, want 18: %v", len(edges), edges)
	}
	// the outside roofed tile at (8,2) sits beside no wall
	if eaveWidth(sideN, TileSize) != 10 || eaveWidth(sideE, TileSize) != 6 || eaveWidth(sideS, TileSize/2) != 3 {
		t.Error("eave widths: want 10 px under a top wall, 6 elsewhere, scaled by zoom")
	}
	if eaveAlpha(lightState{SunL: [3]float32{0, 0, 0.5}}) != 30 || eaveAlpha(lightState{ShadowAlpha: 0.12}) != 18 {
		t.Error("eave alpha: want 30 by day, 18 at night")
	}
	if n := len(eaveEdges(game.WorldView{TerrainW: 9, RoomGrid: w.RoomGrid}, 0, 0, 8, 5, nil)); n != 0 {
		t.Errorf("no roof data drew %d eaves", n)
	}
	r := New(&stubSim{w: 9, h: 6})
	r.drawEaves(ebiten.NewImage(640, 400), w, 640, 400, r.tilePx())
	if len(r.overlays().RoofDraw.Edges) != 18 {
		t.Errorf("drawEaves found %d edges", len(r.overlays().RoofDraw.Edges))
	}
}

func TestOverlayOnlyWhenToggledOrForced(t *testing.T) {
	t.Parallel()
	w := eaveWorld()
	screen := ebiten.NewImage(640, 400)
	for _, c := range []struct{ toggled, forced, want bool }{
		{false, false, false}, {true, false, true}, {false, true, true}, {true, true, true},
	} {
		r := New(&stubSim{w: 9, h: 6})
		r.overlays().Roof.On, r.overlays().Roof.Forced = c.toggled, c.forced
		if r.roofOverlayOn() != c.want {
			t.Fatalf("%+v: roofOverlayOn %v", c, r.roofOverlayOn())
		}
		r.drawRoofOverlay(screen, w, 640, 400, r.tilePx())
		if drew := r.overlays().RoofDraw.OverlayVerts > 0; drew != c.want {
			t.Fatalf("%+v: overlay drew %d vertices", c, r.overlays().RoofDraw.OverlayVerts)
		}
		// no roof data: nothing, even when on
		r.drawRoofOverlay(screen, game.WorldView{TerrainW: 9}, 640, 400, r.tilePx())
		if r.overlays().RoofDraw.OverlayVerts != 0 {
			t.Fatalf("%+v: overlay drew without roof data", c)
		}
	}
	// every marking draws: hatch, dashes and stipple
	w.RoofCells[1*9+1] |= roofPlanRemove
	w.RoofCells[3*9+3] = roofPlanBuild
	w.RoofCells[4*9+1] = roofNatural
	r := New(&stubSim{w: 9, h: 6})
	r.overlays().Roof.On = true
	r.drawRoofOverlay(screen, w, 640, 400, r.tilePx())
	r.drawRoofUnsupported(screen, 2, 2, r.tilePx())
	if r.overlays().RoofDraw.OverlayVerts == 0 {
		t.Fatal("overlay drew nothing")
	}
}
