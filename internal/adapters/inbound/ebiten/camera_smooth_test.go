package ebiten

import (
	"math"
	"testing"
)

func testLimits() camLimits {
	return camLimits{v: viewport{0, 0, 800, 600}, mapW: 100, mapH: 100, minZoom: 0.2}
}

func testCam() *camera {
	return &camera{zoom: 1, placed: true, offsetX: 1000, offsetY: 1000}
}

const dt60 = 1.0 / 60

func TestPanAcceleratesAndCoasts(t *testing.T) {
	var s camSmooth
	c := testCam()
	lim := testLimits()
	for i := 0; i < 30; i++ { // 0.5 s held
		s.pushPan(1, 0)
		s.step(dt60, c, lim)
	}
	maxPerFrame := float64(panPixels)
	before := c.offsetX
	s.pushPan(1, 0)
	s.step(dt60, c, lim)
	if got := c.offsetX - before; got < 0.95*maxPerFrame || got > maxPerFrame+0.01 {
		t.Fatalf("steady pan %v px/frame, want ~%v", got, maxPerFrame)
	}
	// release: coasts, then stops within ~200 ms (applyCamera clears the held direction per Update)
	s.dirX, s.dirY = 0, 0
	var last float64
	for i := 0; i < 12; i++ {
		last = c.offsetX
		s.step(dt60, c, lim)
	}
	if c.offsetX <= before+maxPerFrame {
		t.Fatalf("expected coasting after release")
	}
	if d := c.offsetX - last; d > 0.1 {
		t.Fatalf("still moving %v px/frame after 200ms", d)
	}
	if c.offsetY != 1000 {
		t.Fatalf("y drifted: %v", c.offsetY)
	}
}

func TestPanDoesNotOvershootBounds(t *testing.T) {
	var s camSmooth
	c := testCam()
	lim := testLimits()
	hi := 100*TileSize*c.zoom - lim.v.x1 + edgeSlack*lim.v.w()
	for i := 0; i < 600; i++ {
		s.pushPan(1, 0)
		s.step(dt60, c, lim)
		if c.offsetX > hi+1e-9 {
			t.Fatalf("overshoot: %v > %v", c.offsetX, hi)
		}
	}
	for i := 0; i < 30; i++ {
		s.step(dt60, c, lim)
	}
	if math.Abs(c.offsetX-hi) > 1e-6 {
		t.Fatalf("offset %v, want bound %v", c.offsetX, hi)
	}
}

func TestZoomEasesToTargetAnchoredAtCursor(t *testing.T) {
	var s camSmooth
	c := testCam()
	lim := testLimits()
	ax, ay := 300.0, 200.0
	tile := float64(TileSize)
	wx, wy := (ax+c.offsetX)/(tile*c.zoom), (ay+c.offsetY)/(tile*c.zoom)
	s.zoomAt(1.1, ax, ay)
	s.step(dt60, c, lim)
	if c.zoom <= 1 || c.zoom >= 1.1 {
		t.Fatalf("first frame zoom %v should be partway to 1.1", c.zoom)
	}
	for i := 0; i < 9; i++ { // ~150 ms total
		s.step(dt60, c, lim)
		tp := tile * c.zoom
		gx, gy := (ax+c.offsetX)/tp, (ay+c.offsetY)/tp
		if math.Abs(gx-wx) > 1e-6 || math.Abs(gy-wy) > 1e-6 {
			t.Fatalf("anchor drifted: (%v,%v) want (%v,%v)", gx, gy, wx, wy)
		}
	}
	if math.Abs(c.zoom-1.1) > 0.01 {
		t.Fatalf("zoom %v after 150ms, want ~1.1", c.zoom)
	}
	for i := 0; i < 60; i++ {
		s.step(dt60, c, lim)
	}
	if c.zoom != 1.1 {
		t.Fatalf("final zoom %v, want exactly 1.1", c.zoom)
	}
}

func TestZoomStepsAccumulateAndClamp(t *testing.T) {
	var s camSmooth
	c := testCam()
	lim := testLimits()
	s.zoomAt(100, 400, 300)
	for i := 0; i < 120; i++ {
		s.step(dt60, c, lim)
	}
	if c.zoom != maxZoom {
		t.Fatalf("zoom %v, want max %v", c.zoom, maxZoom)
	}
	s.zoomAt(0.0001, 400, 300)
	for i := 0; i < 240; i++ {
		s.step(dt60, c, lim)
	}
	if c.zoom != lim.minZoom {
		t.Fatalf("zoom %v, want min %v", c.zoom, lim.minZoom)
	}
}

func TestCaptureCameraInstant(t *testing.T) {
	s := camSmooth{instant: true}
	c := testCam()
	lim := testLimits()
	s.zoomAt(1.1, 300, 200)
	s.pushPan(1, 0)
	s.step(dt60, c, lim)
	if math.Abs(c.zoom-1.1) > 1e-12 {
		t.Fatalf("instant zoom %v, want 1.1", c.zoom)
	}
	// pan applied at full speed in one frame, no residual velocity
	x := c.offsetX
	s.step(dt60, c, lim)
	if c.offsetX != x {
		t.Fatalf("instant mode kept coasting")
	}
}
