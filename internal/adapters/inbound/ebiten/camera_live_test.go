package ebiten

import (
	"math"
	"testing"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/input"
)

// The camera as the player drives it: a key held through App.Update and
// the view moving on App.Draw (PERF-002 hot spot 5 steps the smoothing per
// Draw). These go through the real input path because the unit tests that
// stepped the pieces let a dead pan key through once.

// liveCameraColony is a paused stub colony on an injected clock with the
// window focused and Reduced Motion off, one Draw done so the camera step
// has a previous frame.
func liveCameraColony(t *testing.T) (*App, *fakeInput, *time.Time, *ebiten.Image) {
	t.Helper()
	a, _, f := inputColony(t, [2]int{8, 4})
	clock := time.Unix(1000, 0)
	a.now = func() time.Time { return clock }
	f.cur.focused = true
	a.renderer.smooth.instant = false
	img := ebiten.NewImage(a.sw, a.sh) // the first Draw sets the physical size from it
	a.Draw(img)
	return a, f, &clock, img
}

// draws runs n Draws 8 ms apart (a 120 Hz display).
func draws(a *App, clock *time.Time, img *ebiten.Image, n int) {
	for i := 0; i < n; i++ {
		*clock = clock.Add(8 * time.Millisecond)
		a.Draw(img)
	}
}

func panSign(v float64) float64 {
	switch {
	case v > 0.5:
		return 1
	case v < -0.5:
		return -1
	}
	return 0
}

// TestHeldPanKeyMovesTheCamera: a held pan key (and the edge band) moves
// the view on the Draws after one Update, and letting go coasts to a stop
// without reversing.
func TestHeldPanKeyMovesTheCamera(t *testing.T) {
	for _, c := range []struct {
		name   string
		hold   func(a *App, f *fakeInput)
		dx, dy float64
	}{
		{"D pans right", func(a *App, f *fakeInput) { f.cur.held = []string{"D"} }, 1, 0},
		{"ArrowDown pans down", func(a *App, f *fakeInput) { f.cur.held = []string{"ArrowDown"} }, 0, 1},
		{"S and D pan diagonally", func(a *App, f *fakeInput) { f.cur.held = []string{"S", "D"} }, 1, 1},
		{"the right edge band scrolls", func(a *App, f *fakeInput) {
			a.renderer.edgeScroll = true
			f.cur.mx, f.cur.my = a.pw-1, a.ph/2
		}, 1, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, f, clock, img := liveCameraColony(t)
			r := a.renderer
			x0, y0 := r.cam.offsetX, r.cam.offsetY
			c.hold(a, f)
			f.frame(t, a) // one real Update: the key is read here
			draws(a, clock, img, 3)
			dx, dy := r.cam.offsetX-x0, r.cam.offsetY-y0
			if panSign(dx) != c.dx || panSign(dy) != c.dy {
				t.Fatalf("after a held Update and 3 Draws the camera moved (%v, %v), want the signs (%v, %v)", dx, dy, c.dx, c.dy)
			}
			// let go: the next Update clears the push; the pan coasts
			// forward to a stop and never reverses
			f.cur.held = nil
			f.cur.mx, f.cur.my = a.pw/2, a.ph/2
			f.frame(t, a)
			if r.smooth.dirX != 0 || r.smooth.dirY != 0 {
				t.Fatalf("direction (%v, %v) after the release, want none", r.smooth.dirX, r.smooth.dirY)
			}
			lx, ly := r.cam.offsetX, r.cam.offsetY
			for i := 0; i < 400 && !r.smooth.idle(); i++ {
				draws(a, clock, img, 1)
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

// TestShiftPansFaster: Shift with a held key covers more ground in the
// same Draws.
func TestShiftPansFaster(t *testing.T) {
	run := func(mods input.Mod) float64 {
		a, f, clock, img := liveCameraColony(t)
		x0 := a.renderer.cam.offsetX
		f.cur.held, f.cur.mods = []string{"D"}, mods
		f.frame(t, a)
		draws(a, clock, img, 4)
		return a.renderer.cam.offsetX - x0
	}
	plain, fast := run(0), run(input.ModShift)
	if plain <= 0 || fast <= plain*1.2 {
		t.Fatalf("plain pan %v, Shift pan %v: want Shift clearly faster", plain, fast)
	}
}

// TestLiveZoomStepsInDraw: a zoom key and the wheel, read in Update, ease
// the zoom on the following Draws (no Reduced Motion).
func TestLiveZoomStepsInDraw(t *testing.T) {
	for _, c := range []struct {
		name string
		do   func(t *testing.T, a *App, f *fakeInput)
	}{
		{"PageDown zooms in", func(t *testing.T, a *App, f *fakeInput) { f.key(t, a, "PageDown", 0) }},
		{"the wheel zooms in", func(t *testing.T, a *App, f *fakeInput) {
			f.cur.mx, f.cur.my, f.cur.wheelY = a.pw/2, a.ph/2, 1
			f.frame(t, a)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, f, clock, img := liveCameraColony(t)
			z0 := a.renderer.cam.zoom
			c.do(t, a, f)
			draws(a, clock, img, 3)
			if a.renderer.cam.zoom <= z0 {
				t.Fatalf("zoom %v after the input and 3 Draws, want more than %v", a.renderer.cam.zoom, z0)
			}
		})
	}
}

// TestMinimapClickRecentres: a click on the minimap through App.Update
// moves the camera to that spot of the map.
func TestMinimapClickRecentres(t *testing.T) {
	sim := &stubSim{w: 200, h: 200, world: game.WorldView{TerrainW: 200, Terrain: make([]uint8, 200*200)}}
	a, f := globalColony(t, sim, 100, 1280, 800)
	r := a.renderer
	if !r.minimapOn() {
		t.Fatal("a 200² map should show the minimap")
	}
	v := colonyViewport(a.sw, a.sh)
	_, _, mx, my, mw, mh := r.minimapBox(v)
	f.cur.mx, f.cur.my = int(mx+mw*0.25), int(my+mh*0.75)
	f.click(t, a, mbLeft, 0)
	mv := a.mapViewport()
	tile := r.tilePx()
	cx := (r.cam.offsetX + (mv.x0+mv.x1)/2) / tile
	cy := (r.cam.offsetY + (mv.y0+mv.y1)/2) / tile
	if math.Abs(cx-50) > 1.5 || math.Abs(cy-150) > 1.5 {
		t.Fatalf("camera centred on (%.1f, %.1f) after the click, want (50, 150)", cx, cy)
	}
}

// TestBoxSelectAtHalfRenderScale: a left drag around a pawn selects it at
// Half render scale as it does at Full (the drag box is read in map
// pixels through mapCursor).
func TestBoxSelectAtHalfRenderScale(t *testing.T) {
	for _, k := range []float64{1, 0.5} {
		a, f := renderScaleColony(t, 2, k)
		x0, y0 := a.renderer.worldToScreenF(20.2, 18.2) // map pixels around the pawn at (21, 19)
		x1, y1 := a.renderer.worldToScreenF(22.8, 20.8)
		raw := func(x, y float32) (int, int) { return int(float64(x) / k), int(float64(y) / k) }
		f.cur.mx, f.cur.my = raw(x0, y0)
		f.cur.press[mbLeft], f.cur.down[mbLeft] = true, true
		f.frame(t, a)
		f.cur.mx, f.cur.my = raw(x1, y1)
		f.frame(t, a)
		f.cur.down[mbLeft], f.cur.release[mbLeft] = false, true
		f.frame(t, a)
		if !a.renderer.HasSelect() || a.renderer.Selected() != 7 {
			t.Fatalf("world %v: the drag around the pawn selected %v %v, want pawn 7", k, a.renderer.HasSelect(), a.renderer.Selected())
		}
	}
}
