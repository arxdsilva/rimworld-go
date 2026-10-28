package ebiten

import (
	"fmt"
	"math"
	"testing"
)

// The map fills the whole window and the HUD floats over it (PLAY-018):
// the logical viewport is the screen, the physical one is the same
// scaled, and the App's map viewport is the physical window.
func TestColonyViewportIsTheWholeScreen(t *testing.T) {
	for _, s := range []float64{1, 1.5, 2} {
		l := colonyViewport(1280, 720)
		if l != (viewport{X0: 0, Y0: 0, X1: 1280, Y1: 720}) {
			t.Fatalf("logical viewport %+v, want the whole 1280x720 screen", l)
		}
		if p := colonyViewportPhysical(1280, 720, s); p != (viewport{X0: 0, Y0: 0, X1: 1280 * s, Y1: 720 * s}) {
			t.Fatalf("s=%v: physical viewport %+v", s, p)
		}
	}
	for _, sc := range globalScales {
		a, _ := globalColony(t, newGlobalSim(), sc.ui, sc.ww, sc.wh)
		if v := a.mapViewport(); v != (viewport{X0: 0, Y0: 0, X1: sc.ww, Y1: sc.wh}) {
			t.Errorf("ui %d: map viewport %+v, want the whole %vx%v window", sc.ui, v, sc.ww, sc.wh)
		}
	}
}

// The first call centres the camera on the middle of the whole screen, and
// a camera pushed to any limit keeps the map within the slack of the
// screen's own edges, not of an inner area.
func TestCameraClampsToTheWholeScreen(t *testing.T) {
	t.Parallel()
	v := colonyViewport(1280, 720)
	r := New(&stubSim{w: 40, h: 40})
	r.cam.Zoom = 1
	r.ClampCamera(v) // first call: centred on the map
	tile := r.tilePx()
	if cx := (v.X0+v.X1)/2 + r.cam.OffsetX; cx != 20*tile {
		t.Errorf("map centre at %v, want %v: not centred on the screen", cx, 20*tile)
	}
	if cy := (v.Y0+v.Y1)/2 + r.cam.OffsetY; cy != 20*tile {
		t.Errorf("map centre y at %v, want %v", cy, 20*tile)
	}
	r.cam.OffsetX, r.cam.OffsetY = -1e6, -1e6 // pushed to the top-left limit
	r.ClampCamera(v)
	if left := -r.cam.OffsetX; left > edgeSlack*v.W()+1e-6 {
		t.Errorf("the map's left edge is %v px in, more than the slack %v", left, edgeSlack*v.W())
	}
	if top := -r.cam.OffsetY; top > edgeSlack*v.H()+1e-6 {
		t.Errorf("the map's top edge is %v px in, more than the slack %v", top, edgeSlack*v.H())
	}
	if top := -r.cam.OffsetY; top < edgeSlack*v.H()-1e-6 {
		t.Errorf("the top limit %v is short of the screen's slack %v: something still reserves a top strip", top, edgeSlack*v.H())
	}
	r.cam.OffsetX, r.cam.OffsetY = 1e6, 1e6 // and the bottom-right
	r.ClampCamera(v)
	if right := 40*tile - r.cam.OffsetX; right < v.X1-edgeSlack*v.W()-1e-6 {
		t.Errorf("the map's right edge %v is further in than the slack from %v", right, v.X1)
	}
	if bottom := 40*tile - r.cam.OffsetY; bottom < v.Y1-edgeSlack*v.H()-1e-6 || bottom > v.Y1-edgeSlack*v.H()+1e-6 {
		t.Errorf("the map's bottom edge %v should end the slack %v above the screen's bottom %v", bottom, edgeSlack*v.H(), v.Y1)
	}
}

// centerOnMap puts the target in the middle of the whole window.
func TestCenterOnMapTargetsTheWindowCentre(t *testing.T) {
	for _, sc := range globalScales {
		sim := &stubSim{w: 80, h: 80}
		a, _ := globalColony(t, sim, sc.ui, sc.ww, sc.wh)
		a.renderer.cam.Zoom = 1
		a.centerOnMap(40.5, 40.5)
		x, y := a.renderer.worldToScreenF(40.5, 40.5)
		v := a.mapViewport()
		if wantX, wantY := (v.X0+v.X1)/2, (v.Y0+v.Y1)/2; math.Abs(float64(x)-wantX) > 1 || math.Abs(float64(y)-wantY) > 1 {
			t.Errorf("ui %d: target drawn at (%v,%v), want the window centre (%v,%v)", sc.ui, x, y, wantX, wantY)
		}
	}
}

// Edge scrolling works at every edge and corner of the window, including
// over the places the HUD floats (the tab strip, the right column): the
// viewport is the whole window.
func TestEdgeScrollAtEveryScreenEdge(t *testing.T) {
	for _, sc := range globalScales {
		t.Run(fmt.Sprintf("ui%d", sc.ui), func(t *testing.T) {
			a, f := globalColony(t, &stubSim{w: 80, h: 80}, sc.ui, sc.ww, sc.wh)
			a.renderer.edgeScroll = true
			f.Cur.Focused = true
			w, h := a.pw, a.ph
			for _, c := range []struct {
				name   string
				x, y   int
				dx, dy float64
			}{
				{"top-left corner", 0, 0, -1, -1},
				{"top edge (the old top strip)", w / 2, 0, 0, -1},
				{"top-right corner", w - 1, 0, 1, -1},
				{"right edge (the right column)", w - 1, h / 2, 1, 0},
				{"bottom-right corner", w - 1, h - 1, 1, 1},
				{"bottom edge (the tab strip)", w / 2, h - 1, 0, 1},
				{"bottom-left corner", 0, h - 1, -1, 1},
				{"left edge", 0, h / 2, -1, 0},
				{"middle of the screen", w / 2, h / 2, 0, 0},
				{"just inside the band", w / 2, int(6*screenDev()) + 2, 0, 0},
			} {
				f.Cur.Mx, f.Cur.My = c.x, c.y
				f.Frame(t, a)
				in := a.cameraInput(a.mapViewport())
				if in.dx != c.dx || in.dy != c.dy {
					t.Errorf("%s (%d,%d): pan (%v,%v), want (%v,%v)", c.name, c.x, c.y, in.dx, in.dy, c.dx, c.dy)
				}
			}
		})
	}
}

// The wheel zooms about the cursor everywhere on the map, including where
// the top strip used to be.
func TestZoomAnchorsOverTheFormerTopStrip(t *testing.T) {
	a, f := globalColony(t, &stubSim{w: 80, h: 80}, 100, 1280, 720)
	r := a.renderer
	r.smooth.Instant = true
	a.centerOnMap(40, 40)
	x, y := 700, 10 // physical, in the old strip
	wx, wy := r.screenToWorld(x, y)
	f.Cur.Mx, f.Cur.My, f.Cur.WheelY = x, y, 2
	f.Frame(t, a)
	if r.cam.Zoom <= 1 {
		t.Fatalf("zoom %v after wheel steps in", r.cam.Zoom)
	}
	if gx, gy := r.screenToWorld(x, y); math.Abs(gx-wx) > 0.01 || math.Abs(gy-wy) > 0.01 {
		t.Fatalf("the cursor's world point moved: (%v,%v), want (%v,%v)", gx, gy, wx, wy)
	}
}
