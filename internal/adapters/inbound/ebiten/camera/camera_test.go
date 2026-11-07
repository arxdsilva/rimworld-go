package camera

import (
	"math"
	"testing"
)

func TestClampAxis(t *testing.T) {
	// map 1000px, viewport 0..500, slack 50
	if got := ClampAxis(-500, 1000, 0, 500, 50); got != -50 {
		t.Errorf("near edge: got %v, want -50", got)
	}
	if got := ClampAxis(5000, 1000, 0, 500, 50); got != 550 {
		t.Errorf("far edge: got %v, want 550", got)
	}
	if got := ClampAxis(200, 1000, 0, 500, 50); got != 200 {
		t.Errorf("inside: got %v, want 200", got)
	}
	// map 200px in a 500px viewport is centred: left edge at 150
	if got := ClampAxis(0, 200, 0, 500, 50); got != -150 {
		t.Errorf("small map: got %v, want -150", got)
	}
}

func TestViewRoundTripAcrossViewportsAndDensity(t *testing.T) {
	src := Camera{Zoom: 1.5, Dev: 2}
	vA := Viewport{0, 0, 2000, 1400}
	src.OffsetX, src.OffsetY = 3000, 2500
	w := src.View(vA)
	for _, c := range []struct {
		v   Viewport
		dev float64
	}{{vA, 2}, {Viewport{0, 0, 1280, 720}, 1}, {Viewport{0, 0, 3000, 2000}, 3}} {
		var d Camera
		d.Dev = c.dev
		d.Resume(w)
		d.Clamp(c.v, 200, 200, [2]int{})
		got := d.View(c.v)
		if math.Abs(got.CX-w.CX) > 1e-6 || math.Abs(got.CY-w.CY) > 1e-6 || got.Zoom != w.Zoom {
			t.Errorf("%+v dev %v: view %+v, want %+v", c.v, c.dev, got, w)
		}
		if !d.Placed {
			t.Error("a resumed camera is placed")
		}
	}
}

// TestCellFollowsTheWindowHeight (UIS-080, UIS-084): the default zoom
// shows CellsTall cells of window height, so a cell is the window's height
// / 48 in map pixels at any width; a resize keeps the world point at the
// top-left corner.
func TestCellFollowsTheWindowHeight(t *testing.T) {
	for _, c := range []struct{ w, h, cell float64 }{{1920, 1080, 22.5}, {1280, 720, 15}, {5120, 2762, 57.541666}} {
		cam := Camera{Zoom: 1}
		v := Viewport{0, 0, c.w, c.h}
		cam.Clamp(v, 250, 250, [2]int{})
		if math.Abs(cam.Tile()-c.cell) > 1e-3 {
			t.Errorf("%vx%v: cell %v, want %v", c.w, c.h, cam.Tile(), c.cell)
		}
		if rows := v.H() / cam.Tile(); math.Abs(rows-CellsTall) > 1e-9 {
			t.Errorf("%vx%v: %v cells tall at the default zoom, want %d", c.w, c.h, rows, CellsTall)
		}
	}
	cam := Camera{Zoom: 1.5}
	cam.Clamp(Viewport{0, 0, 1920, 1080}, 250, 250, [2]int{})
	wx, wy := cam.OffsetX/cam.Tile(), cam.OffsetY/cam.Tile()
	var s Smooth
	s.Vx = 100
	cam.SyncUnit(720, &s)
	if gx, gy := cam.OffsetX/cam.Tile(), cam.OffsetY/cam.Tile(); math.Abs(gx-wx) > 1e-9 || math.Abs(gy-wy) > 1e-9 {
		t.Errorf("resize moved the top-left world point: (%v,%v), want (%v,%v)", gx, gy, wx, wy)
	}
	if cam.Tile() != 15*1.5 || math.Abs(s.Vx-100*15/22.5) > 1e-9 {
		t.Errorf("after the resize: tile %v, velocity %v", cam.Tile(), s.Vx)
	}
	if b := (Camera{Zoom: 0.5}); b.Tile() != TileSize/2 {
		t.Errorf("a camera never synced (a bake's) is px per tile as given: %v", b.Tile())
	}
}

// TestZoomOutFloorIsFixedUnlessTheMapIsSmall (UIS-085): MinZoom on a map
// bigger than the floor shows; a small map is held at its fit, never past
// the default zoom.
func TestZoomOutFloorIsFixedUnlessTheMapIsSmall(t *testing.T) {
	var c Camera
	v := Viewport{0, 0, 1920, 1080}
	for _, side := range []int{250, 325} {
		if z := c.MinZoomFor(v, side, side); z != MinZoom {
			t.Errorf("%d map: floor %v, want %v", side, z, MinZoom)
		}
	}
	// 9 px cells at 1080: 120 rows and 213 columns at the floor
	if cell := UnitFor(1080) * MinZoom; math.Abs(cell-9) > 1e-9 {
		t.Errorf("floor cell %v px at 1080, want 9", cell)
	}
	if cell := UnitFor(1080) * MaxZoom; math.Abs(cell-49.05) > 1e-9 {
		t.Errorf("max cell %v px at 1080, want 49.05", cell)
	}
	small := c.MinZoomFor(v, 60, 60) // 60 rows: the floor would leave most of the screen black
	if small <= MinZoom || small > 1 {
		t.Errorf("60 map: floor %v, want between %v and 1", small, MinZoom)
	}
	if long := 60 * UnitFor(1080) * small / 1920; long < MinCover-1e-9 {
		t.Errorf("60 map at its floor covers %v of the width, want at least %v", long, MinCover)
	}
	if z := c.MinZoomFor(v, 10, 10); z != 1 {
		t.Errorf("a tiny map stops at the default zoom, got %v", z)
	}
}

func TestResumeClampsAndIgnoresBadViews(t *testing.T) {
	v := Viewport{0, 0, 1000, 800}
	var c Camera
	c.Resume(View{CX: -50, CY: 9999, Zoom: 50})
	c.Clamp(v, 100, 100, [2]int{})
	if c.Zoom != MaxZoom {
		t.Errorf("zoom %v, want %v", c.Zoom, MaxZoom)
	}
	if g := c.View(v); g.CX < -1 || g.CY > 101 {
		t.Errorf("centre %+v is off the map", g)
	}
	var d, e Camera
	d.Resume(View{CX: math.NaN(), CY: 1, Zoom: 1})
	d.Resume(View{CX: 1, CY: 1, Zoom: 0})
	e.Zoom = 1
	d.Zoom = 1
	d.Clamp(v, 100, 100, [2]int{})
	e.Clamp(v, 100, 100, [2]int{})
	if d.OffsetX != e.OffsetX || d.OffsetY != e.OffsetY || d.Zoom != e.Zoom {
		t.Errorf("bad view changed centring: %+v vs %+v", d, e)
	}
	var f Camera
	f.Resume(View{CX: 10, CY: 10, Zoom: 0.0001})
	f.Clamp(v, 100, 100, [2]int{})
	if f.Zoom < f.MinZoomFor(v, 100, 100) {
		t.Errorf("zoom %v below the limit", f.Zoom)
	}
}
