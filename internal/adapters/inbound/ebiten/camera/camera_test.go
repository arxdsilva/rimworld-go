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
