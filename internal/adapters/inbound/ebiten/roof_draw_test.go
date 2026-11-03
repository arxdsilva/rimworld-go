package ebiten

import (
	overlaypkg "rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

func TestRoomLabelRoofedPercent(t *testing.T) {
	t.Parallel()
	w := game.WorldView{TerrainW: 2, RoomGrid: []int{0, 1}, Rooms: []game.RoomView{
		{ID: 0, Role: "bedroom", Label: "decent", Score: 12, Roofed: 1},
		{ID: 1, Role: "barn", Label: "dull", Score: 3, Lit: true, Roofed: 0.426},
	}}
	if got := world.RoomLabelAt(w, 0, 0); !strings.HasSuffix(got, ", roofed 100%") || !strings.HasPrefix(got, "Room: bedroom") {
		t.Errorf("full roof: %q", got)
	}
	if got := world.RoomLabelAt(w, 1, 0); !strings.HasSuffix(got, ", lit, roofed 43%") {
		t.Errorf("partial roof: %q", got)
	}
}

// roofedRoomWorld: a 7x6 room walled on every side (walls at x=0,6 and y=0,5),
// its interior roofed, and one roofed tile outside it.
func roofedRoomWorld() game.WorldView {
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
				w.RoofCells[i] = overlaypkg.RoofBuilt
			}
		}
	}
	w.RoofCells[3*9+3] = 0 // an unroofed hole
	w.RoofCells[2*9+8] = overlaypkg.RoofBuilt
	return w
}

func TestOverlayOnlyWhenToggledOrForced(t *testing.T) {
	t.Parallel()
	w := roofedRoomWorld()
	screen := ebiten.NewImage(640, 400)
	for _, c := range []struct{ toggled, forced, want bool }{
		{false, false, false}, {true, false, true}, {false, true, true}, {true, true, true},
	} {
		r := New(&stubSim{w: 9, h: 6})
		r.overlays().Roof.On, r.overlays().Roof.Forced = c.toggled, c.forced
		if r.overlays().Roof.Shown() != c.want {
			t.Fatalf("%+v: roofOverlayOn %v", c, r.overlays().Roof.Shown())
		}
		r.drawRoofOverlay(screen, w, 640, 400, r.cam.Tile())
		if drew := r.overlays().RoofDraw.OverlayVerts > 0; drew != c.want {
			t.Fatalf("%+v: overlay drew %d vertices", c, r.overlays().RoofDraw.OverlayVerts)
		}
		// no roof data: nothing, even when on
		r.drawRoofOverlay(screen, game.WorldView{TerrainW: 9}, 640, 400, r.cam.Tile())
		if r.overlays().RoofDraw.OverlayVerts != 0 {
			t.Fatalf("%+v: overlay drew without roof data", c)
		}
	}
	// every marking draws: hatch, dashes and stipple
	w.RoofCells[1*9+1] |= overlaypkg.RoofPlanRemove
	w.RoofCells[3*9+3] = overlaypkg.RoofPlanBuild
	w.RoofCells[4*9+1] = overlaypkg.RoofNatural
	r := New(&stubSim{w: 9, h: 6})
	r.overlays().Roof.On = true
	r.drawRoofOverlay(screen, w, 640, 400, r.cam.Tile())
	r.overlays().DrawRoofUnsupported(r.liveView(), screen, 2, 2, r.cam.Tile())
	if r.overlays().RoofDraw.OverlayVerts == 0 {
		t.Fatal("overlay drew nothing")
	}
}
