package ebiten

import (
	"math"
	"testing"
)

func testLimits() camLimits {
	return camLimits{V: viewport{0, 0, 800, 600}, MapW: 100, MapH: 100, MinZoom: 0.2}
}

func testCam() *camera {
	return &camera{Zoom: 1, Placed: true, OffsetX: 1000, OffsetY: 1000}
}

const dt60 = 1.0 / 60

func TestPanAcceleratesAndCoasts(t *testing.T) {
	var s camSmooth
	c := testCam()
	lim := testLimits()
	for i := 0; i < 30; i++ { // 0.5 s held
		s.PushPan(1, 0)
		s.Step(dt60, c, lim)
	}
	maxPerFrame := float64(panPixels)
	before := c.OffsetX
	s.PushPan(1, 0)
	s.Step(dt60, c, lim)
	if got := c.OffsetX - before; got < 0.95*maxPerFrame || got > maxPerFrame+0.01 {
		t.Fatalf("steady pan %v px/frame, want ~%v", got, maxPerFrame)
	}
	// release: coasts, then stops within ~200 ms (applyCamera clears the held direction per Update)
	s.DirX, s.DirY = 0, 0
	var last float64
	for i := 0; i < 12; i++ {
		last = c.OffsetX
		s.Step(dt60, c, lim)
	}
	if c.OffsetX <= before+maxPerFrame {
		t.Fatalf("expected coasting after release")
	}
	if d := c.OffsetX - last; d > 0.1 {
		t.Fatalf("still moving %v px/frame after 200ms", d)
	}
	if c.OffsetY != 1000 {
		t.Fatalf("y drifted: %v", c.OffsetY)
	}
}

func TestPanDoesNotOvershootBounds(t *testing.T) {
	var s camSmooth
	c := testCam()
	lim := testLimits()
	hi := 100*TileSize*c.Zoom - lim.V.X1 + edgeSlack*lim.V.W()
	for i := 0; i < 600; i++ {
		s.PushPan(1, 0)
		s.Step(dt60, c, lim)
		if c.OffsetX > hi+1e-9 {
			t.Fatalf("overshoot: %v > %v", c.OffsetX, hi)
		}
	}
	for i := 0; i < 30; i++ {
		s.Step(dt60, c, lim)
	}
	if math.Abs(c.OffsetX-hi) > 1e-6 {
		t.Fatalf("offset %v, want bound %v", c.OffsetX, hi)
	}
}

func TestZoomEasesToTargetAnchoredAtCursor(t *testing.T) {
	var s camSmooth
	c := testCam()
	lim := testLimits()
	ax, ay := 300.0, 200.0
	tile := float64(TileSize)
	wx, wy := (ax+c.OffsetX)/(tile*c.Zoom), (ay+c.OffsetY)/(tile*c.Zoom)
	s.ZoomAt(1.1, ax, ay)
	s.Step(dt60, c, lim)
	if c.Zoom <= 1 || c.Zoom >= 1.1 {
		t.Fatalf("first frame zoom %v should be partway to 1.1", c.Zoom)
	}
	for i := 0; i < 9; i++ { // ~150 ms total
		s.Step(dt60, c, lim)
		tp := tile * c.Zoom
		gx, gy := (ax+c.OffsetX)/tp, (ay+c.OffsetY)/tp
		if math.Abs(gx-wx) > 1e-6 || math.Abs(gy-wy) > 1e-6 {
			t.Fatalf("anchor drifted: (%v,%v) want (%v,%v)", gx, gy, wx, wy)
		}
	}
	if math.Abs(c.Zoom-1.1) > 0.01 {
		t.Fatalf("zoom %v after 150ms, want ~1.1", c.Zoom)
	}
	for i := 0; i < 60; i++ {
		s.Step(dt60, c, lim)
	}
	if c.Zoom != 1.1 {
		t.Fatalf("final zoom %v, want exactly 1.1", c.Zoom)
	}
}

func TestZoomStepsAccumulateAndClamp(t *testing.T) {
	var s camSmooth
	c := testCam()
	lim := testLimits()
	s.ZoomAt(100, 400, 300)
	for i := 0; i < 120; i++ {
		s.Step(dt60, c, lim)
	}
	if c.Zoom != maxZoom {
		t.Fatalf("zoom %v, want max %v", c.Zoom, maxZoom)
	}
	s.ZoomAt(0.0001, 400, 300)
	for i := 0; i < 240; i++ {
		s.Step(dt60, c, lim)
	}
	if c.Zoom != lim.MinZoom {
		t.Fatalf("zoom %v, want min %v", c.Zoom, lim.MinZoom)
	}
}

func TestCaptureCameraInstant(t *testing.T) {
	s := camSmooth{Instant: true}
	c := testCam()
	lim := testLimits()
	s.ZoomAt(1.1, 300, 200)
	s.PushPan(1, 0)
	s.Step(dt60, c, lim)
	if math.Abs(c.Zoom-1.1) > 1e-12 {
		t.Fatalf("instant zoom %v, want 1.1", c.Zoom)
	}
	// pan applied at full speed in one frame, no residual velocity
	x := c.OffsetX
	s.Step(dt60, c, lim)
	if c.OffsetX != x {
		t.Fatalf("instant mode kept coasting")
	}
}
