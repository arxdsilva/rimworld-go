package ebiten

import (
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"

	"github.com/hajimehoshi/ebiten/v2"
)

// The world render scale (PERF-002 hot spot 6, Graphics.RenderScale). On a
// 2x screen the map, its lighting, water and shadows fill four times the
// pixels; "World detail" draws the map into an offscreen at world x the
// physical size (3/4 or 1/2) and composites it to the screen with linear
// filtering. The HUD, the world-anchored labels and all text stay native.
//
// One coordinate space: the camera works in map pixels (physical x world;
// screenDev() = dev x world), so everything inside Renderer.Draw is
// unchanged, and the crossings scale by world once: mapCursor,
// mapViewport, mapScale (logical -> map pixels), the probe's framing.
//
// Labels: a world pass draws its labels and icons through the canvas
// (worldText, worldIcon, uiFillRect, drawText ...) with logical
// coordinates. While the map draws into the offscreen, uiScale() is
// s x world (so a pass's map pixels / uiScale() is still logical) and each
// canvas helper aimed at the offscreen queues its draw instead
// (deferring); flushWorldOverlay replays the queue on the screen at the
// HUD scale after the composite. Before the fog is drawn, a label whose
// anchor sits on an unexplored tile is dropped, as the fog would cover it.
// At world 1 nothing here runs: the map draws straight to the screen.

// worldRender is an App's render-scaled map: the offscreen and the queue.
type worldRender struct {
	img    *ebiten.Image
	q      gfx.WorldPass
	hidden func(lx, ly float32) bool // a.fogHidesLabel, bound once
}

// The deferred forms of the canvas helpers. Each captures its arguments
// by value (slices and options copied: callers reuse their buffers) in a
// function of its own, so the helpers' own arguments never escape.

// worldTarget is where the colony map draws this frame: the screen, or at
// a reduced render scale the map's offscreen, ceil(physical x world)
// pixels (true). The offscreen is freed when the scale returns to 1.
func (a *App) worldTarget(screen *ebiten.Image) (*ebiten.Image, bool) {
	w := &a.wr
	k := a.scale.WorldK()
	if k == 1 {
		if w.img != nil {
			w.img.Deallocate()
			w.img = nil
		}
		return screen, false
	}
	b := screen.Bounds()
	w.img = gfx.FitLayer(w.img, int(math.Ceil(float64(b.Dx())*k)), int(math.Ceil(float64(b.Dy())*k)))
	return w.img, true
}

// beginWorldPass starts drawing into the offscreen img: uiScale() becomes
// s x world and the canvas queues what it is asked to draw into img.
func (a *App) beginWorldPass(img *ebiten.Image) {
	w := &a.wr
	if w.hidden == nil {
		w.hidden = a.fogHidesLabel
	}
	w.q.Begin(img, w.hidden)
	gfx.BeginWorldPass(&w.q, a.scale.WorldK())
}

// endWorldPass restores the HUD scale and stops queueing.
func (a *App) endWorldPass() {
	gfx.EndWorldPass()
}

// fogHidesLabel reports the logical point (lx, ly) is over an unexplored
// tile of the frame being drawn: the fog, drawn after the labels, would
// cover a label there.
func (a *App) fogHidesLabel(lx, ly float32) bool {
	r := a.renderer
	if r == nil || len(r.view.W.Fog) == 0 {
		return false
	}
	tx, ty := r.TileAt(int(math.Floor(float64(lx)*a.mapScale())), int(math.Floor(float64(ly)*a.mapScale())))
	return overlay.WorldFogged(r.view.W, tx, ty)
}
