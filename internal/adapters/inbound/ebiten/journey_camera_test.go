package ebiten

import (
	"fmt"
	"math"
	"testing"

	"rimworld-go/internal/platform/input"
)

// Camera journeys: the view moves on Draws after the Update that read the
// key (PERF-002 steps the smoothing per Draw), so every one of these holds
// or presses through Update and then Draws on the pinned clock.

func panSign(v float64) float64 {
	switch {
	case v > 0.5:
		return 1
	case v < -0.5:
		return -1
	}
	return 0
}

// TestJourneyCameraHeldPanKey: the regression that started this suite. A
// held pan key moves the view on every Draw until the next Update; letting
// go clears the push there and the pan coasts to a stop without reversing.
func TestJourneyCameraHeldPanKey(t *testing.T) {
	for _, c := range []struct {
		name   string
		hold   func(j *journey)
		dx, dy float64
	}{
		{"D pans right", func(j *journey) { j.hold("D") }, 1, 0},
		{"ArrowRight pans right", func(j *journey) { j.hold("ArrowRight") }, 1, 0},
		{"S pans down", func(j *journey) { j.hold("S") }, 0, 1},
		{"ArrowDown pans down", func(j *journey) { j.hold("ArrowDown") }, 0, 1},
		{"S and D pan diagonally", func(j *journey) { j.hold("S", "D") }, 1, 1},
		{"the right edge band scrolls", func(j *journey) {
			j.a.renderer.edgeScroll = true
			j.f.cur.mx, j.f.cur.my = j.a.sw-1, j.a.sh/2
		}, 1, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			j := newJourney(t)
			r := j.a.renderer
			j.draw(1) // primes the per-Draw step
			x0, y0 := r.cam.offsetX, r.cam.offsetY
			c.hold(j)
			j.update(1) // the key is read here
			j.draw(3)
			dx, dy := r.cam.offsetX-x0, r.cam.offsetY-y0
			if panSign(dx) != c.dx || panSign(dy) != c.dy {
				t.Fatalf("after a held Update and 3 Draws the camera moved (%v, %v), want the signs (%v, %v)", dx, dy, c.dx, c.dy)
			}
			j.release()
			j.f.cur.mx, j.f.cur.my = j.a.sw/2, j.a.sh/2
			j.update(1)
			if r.smooth.dirX != 0 || r.smooth.dirY != 0 {
				t.Fatalf("direction (%v, %v) after the release, want none", r.smooth.dirX, r.smooth.dirY)
			}
			lx, ly := r.cam.offsetX, r.cam.offsetY
			for i := 0; i < 400 && !r.smooth.idle(); i++ {
				j.draw(1)
				if (r.cam.offsetX-lx)*c.dx < 0 || (r.cam.offsetY-ly)*c.dy < 0 {
					t.Fatalf("the coast reversed at draw %d", i)
				}
				lx, ly = r.cam.offsetX, r.cam.offsetY
			}
			if !r.smooth.idle() {
				t.Fatalf("the pan never stopped: v (%v, %v)", r.smooth.vx, r.smooth.vy)
			}
		})
	}
}

// TestJourneyCameraPanLeftAndUp: A/W and the left/up arrows pan the other
// way (the camera starts away from the map's corner).
func TestJourneyCameraPanLeftAndUp(t *testing.T) {
	for _, c := range []struct {
		key    string
		dx, dy float64
	}{{"A", -1, 0}, {"ArrowLeft", -1, 0}, {"W", 0, -1}, {"ArrowUp", 0, -1}} {
		t.Run(c.key, func(t *testing.T) {
			t.Parallel()
			j := newJourney(t)
			j.lookAt(12, 12)
			r := j.a.renderer
			j.draw(1)
			x0, y0 := r.cam.offsetX, r.cam.offsetY
			j.hold(c.key)
			j.update(1)
			j.draw(3)
			if dx, dy := r.cam.offsetX-x0, r.cam.offsetY-y0; panSign(dx) != c.dx || panSign(dy) != c.dy {
				t.Fatalf("%s moved the camera (%v, %v), want the signs (%v, %v)", c.key, dx, dy, c.dx, c.dy)
			}
		})
	}
}

// TestJourneyCameraShiftPansFaster: Shift with a held key covers more
// ground in the same Draws.
func TestJourneyCameraShiftPansFaster(t *testing.T) {
	t.Parallel()
	run := func(mods input.Mod) float64 {
		j := newJourney(t)
		j.draw(1)
		x0 := j.a.renderer.cam.offsetX
		j.hold("D")
		j.f.cur.mods = mods
		j.update(1)
		j.draw(4)
		return j.a.renderer.cam.offsetX - x0
	}
	plain, fast := run(0), run(input.ModShift)
	if plain <= 0 || fast <= plain*1.2 {
		t.Fatalf("plain pan %v, Shift pan %v: want Shift clearly faster", plain, fast)
	}
}

// TestJourneyCameraZoomKeys: the zoom keys read in Update ease the zoom on
// the following Draws.
func TestJourneyCameraZoomKeys(t *testing.T) {
	for _, c := range []struct {
		key string
		in  bool
	}{{"PageDown", true}, {"Equal", true}, {"PageUp", false}, {"Minus", false}} {
		t.Run(c.key, func(t *testing.T) {
			t.Parallel()
			j := newJourney(t)
			j.draw(1)
			z0 := j.a.renderer.cam.zoom
			j.key(c.key, 0)
			j.draw(3)
			if z := j.a.renderer.cam.zoom; (z > z0) != c.in || z == z0 {
				t.Fatalf("%s: zoom %v from %v, want %s", c.key, z, z0, map[bool]string{true: "in", false: "out"}[c.in])
			}
		})
	}
}

// TestJourneyCameraWheelZoomsAtCursor: the wheel zooms about the cursor,
// so the tile under it stays put.
func TestJourneyCameraWheelZoomsAtCursor(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	j.draw(1)
	r := j.a.renderer
	j.f.moveTo(j.a, 12, 5)
	mx, my := j.f.cur.mx, j.f.cur.my
	wx0, wy0 := r.screenToWorld(mx, my)
	z0 := r.cam.zoom
	j.f.cur.wheelY = 1
	j.update(1)
	j.draw(6)
	if r.cam.zoom <= z0 {
		t.Fatalf("zoom %v after a wheel step, want more than %v", r.cam.zoom, z0)
	}
	if wx, wy := r.screenToWorld(mx, my); math.Abs(wx-wx0) > 0.5 || math.Abs(wy-wy0) > 0.5 {
		t.Fatalf("the tile under the cursor moved from (%.2f, %.2f) to (%.2f, %.2f)", wx0, wy0, wx, wy)
	}
}

// TestJourneyCameraMiddleDrag: a middle-button drag moves the map with the
// hand, at once.
func TestJourneyCameraMiddleDrag(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	r := j.a.renderer
	x0 := r.cam.offsetX
	j.f.cur.mx, j.f.cur.my = j.a.sw/2, j.a.sh/2
	j.f.cur.press[mbMiddle], j.f.cur.down[mbMiddle] = true, true
	j.update(1)
	j.f.cur.mx += 40
	j.update(1)
	j.f.cur.down[mbMiddle], j.f.cur.release[mbMiddle] = false, true
	j.update(1)
	if moved := r.cam.offsetX - x0; math.Abs(moved+40) > 0.5 {
		t.Fatalf("a 40 px drag right moved the camera by %v, want -40", moved)
	}
}

// TestJourneyCameraMinimap: Shift+M toggles the minimap on a big map and a
// click on it recentres the camera there.
func TestJourneyCameraMinimap(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(160, 160), jPawns([2]int{80, 80}))
	r := j.a.renderer
	if !r.minimapOn() {
		t.Fatal("a 160² map should show the minimap")
	}
	v := colonyViewport(j.a.sw, j.a.sh)
	_, _, mx, my, mw, mh := r.minimapBox(v)
	j.f.cur.mx, j.f.cur.my = int(mx+mw*0.25), int(my+mh*0.75)
	j.f.click(j.t, j.a, mbLeft, 0)
	mv := j.a.mapViewport()
	tile := r.tilePx()
	cx := (r.cam.offsetX + (mv.x0+mv.x1)/2) / tile
	cy := (r.cam.offsetY + (mv.y0+mv.y1)/2) / tile
	if math.Abs(cx-40) > 1.5 || math.Abs(cy-120) > 1.5 {
		t.Fatalf("camera centred on (%.1f, %.1f) after the click, want (40, 120)", cx, cy)
	}
	j.key("M", input.ModShift)
	if r.minimapOn() {
		t.Fatal("Shift+M did not hide the minimap")
	}
	j.key("M", input.ModShift)
	if !r.minimapOn() {
		t.Fatal("Shift+M did not bring the minimap back")
	}
}

// TestJourneyCameraCycleColonists: Period and Comma select the next and
// previous colonist and bring the camera to them.
func TestJourneyCameraCycleColonists(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{7, 2}, [2]int{18, 8}))
	r := j.a.renderer
	j.key("Period", 0)
	if !r.HasSelect() {
		t.Fatal("Period selected nothing")
	}
	first := r.Selected()
	x1 := r.cam.offsetX
	j.key("Period", 0)
	if r.Selected() == first {
		t.Fatal("a second Period did not move to the next colonist")
	}
	if r.cam.offsetX == x1 {
		t.Fatal("the camera did not follow the next colonist")
	}
	j.key("Comma", 0)
	if r.Selected() != first {
		t.Fatal("Comma did not go back to the previous colonist")
	}
}

// TestJourneyCameraBarDoubleClickCentres: two quick clicks on a colonist
// bar card select the colonist and centre the view on them.
func TestJourneyCameraBarDoubleClickCentres(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{18, 8}))
	r := j.a.renderer
	id := j.g.Snapshot()[0].ID
	j.lookAt(2, 2)
	bar := fmt.Sprintf("bar.pawn.%d", id)
	j.clickID(bar)
	j.clickID(bar)
	if !r.HasSelect() || r.Selected() != id {
		t.Fatalf("the bar click did not select the colonist (select %v %v)", r.HasSelect(), r.Selected())
	}
	p := j.pawn(uint64(id))
	mv := j.a.mapViewport()
	sx, sy := r.worldToScreenF(float64(p.X)+0.5, float64(p.Y)+0.5)
	if math.Abs(float64(sx)-(mv.x0+mv.x1)/2) > r.tilePx() || math.Abs(float64(sy)-(mv.y0+mv.y1)/2) > r.tilePx() {
		t.Fatalf("after the double click the colonist is at (%v, %v), not the viewport centre (%v, %v)", sx, sy, (mv.x0+mv.x1)/2, (mv.y0+mv.y1)/2)
	}
}
