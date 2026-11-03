package world

import (
	"image/color"
	"slices"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// cordLamp is a lamp on net 1 wired from (x, y) to (lx, ly).
func cordLamp(id, x, y, lx, ly int) game.BuildingView {
	b := game.BuildingView{Kind: "lamp", X: x, Y: y, W: 1, H: 1, PowerRole: "consumer", PowerNet: 1,
		HasLink: true, LinkFromX: x, LinkFromY: y, LinkX: lx, LinkY: ly}
	b.ID = entity.EntityID(1000 + id)
	return b
}

// panned draws a frame at tile px a tile, the camera at (ox, oy) tiles,
// and lists the wires it drew by their from cell.
func (r *rig) panned(tile, ox, oy float64) [][2]int {
	var cords [][2]int
	r.p.Overlays.Wiring.Nets.Trace = func(part string, x, y int, _ color.NRGBA) {
		if part == "cord" {
			cords = append(cords, [2]int{x, y})
		}
	}
	r.cam.Zoom = tile / 64
	r.cam.OffsetX, r.cam.OffsetY = ox*tile, oy*tile
	r.v.Cam = *r.cam
	r.v.SW, r.v.SH, r.v.Tile = 640, 400, tile
	r.layers = r.layers[:0]
	r.p.Draw(r.screen)
	return cords
}

// The wires draw over the objects, so a wall hides none, and under the fog,
// so unexplored ground still hides them; not on the whole-map tier.
func TestCordsDrawAfterObjectsBeforeFog(t *testing.T) {
	w := rigWorld(32, 32)
	w.Buildings = append(w.Buildings, cordLamp(1, 1, 1, 2, 1))
	r := newRig(t, w, 32, 32)
	for _, tile := range []float64{64, 32} {
		cords := r.panned(tile, 0, 0)
		ob, co, fog := slices.Index(r.layers, "objects"), slices.Index(r.layers, "power cords"), slices.Index(r.layers, "fog")
		stub := slices.Index(r.layers, "power cord stubs")
		if stub < 0 || ob < stub || co < ob || fog < co {
			t.Fatalf("%v px: layers %v: want power cord stubs, objects, power cords, fog in that order", tile, r.layers)
		}
		if len(cords) != 1 || cords[0] != [2]int{1, 1} {
			t.Fatalf("%v px: wires %v", tile, cords)
		}
	}
	if cords := r.panned(8, 0, 0); len(cords) != 0 || slices.Contains(r.layers, "power cords") || slices.Contains(r.layers, "power cord stubs") {
		t.Fatalf("map tier: wires %v, layers %v", cords, r.layers)
	}
}

// The chunk lists pad the view by VisPad, less than a wire reaches: with
// the view's left edge at x 21 they start at chunk 1 (x 16), and a lamp at
// x 15 wires six tiles into the screen. Its wire still shows, and the
// object pass's lists stay as they were. Likewise north.
func TestCordsReachInFromOutsideTheFrameChunks(t *testing.T) {
	w := rigWorld(64, 64)
	w.Buildings = append(w.Buildings,
		cordLamp(1, 15, 24, 21, 24), // west of the view, chunk 0, wiring in
		cordLamp(2, 25, 15, 25, 21), // north of the view, chunk row 0, wiring in
		cordLamp(3, 2, 24, 8, 24),   // chunk 0 too, wire wholly off the view
	)
	r := newRig(t, w, 64, 64)
	cords := r.panned(64, 21, 21)
	l := r.p.LOD
	if l.cx0 != 1 || l.cy0 != 1 {
		t.Fatalf("the frame's chunks start at %d,%d: the lamps are not outside them", l.cx0, l.cy0)
	}
	for _, c := range [][2]int{{15, 24}, {25, 15}} {
		if !slices.Contains(cords, c) {
			t.Errorf("no wire from %v in %v", c, cords)
		}
	}
	if len(cords) != 2 {
		t.Errorf("wires %v, want the two that reach the view", cords)
	}
	inner := (l.cx1 - l.cx0 + 1) * (l.cy1 - l.cy0 + 1)
	if len(l.Views) != inner {
		t.Errorf("the object pass's lists hold %d chunks, want %d", len(l.Views), inner)
	}
	if got := r.p.Bk.CordChunks().Views; len(got) <= inner {
		t.Errorf("the wire pass walks %d chunks of %d in view", len(got), inner)
	}
}

// The widened list stops at the map's edge and is reused frame to frame.
func TestCordChunksStayOnTheMap(t *testing.T) {
	r := newRig(t, rigWorld(32, 32), 32, 32) // two chunks a side
	r.panned(64, 0, 0)
	got := r.p.Bk.CordChunks().Views
	if len(got) != 4 {
		t.Fatalf("%d chunks on a 2×2 chunk map", len(got))
	}
	if &r.p.Bk.CordChunks().Views[0] != &got[0] {
		t.Fatal("the list was not reused")
	}
}
