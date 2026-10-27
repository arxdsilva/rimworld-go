package ebiten

import (
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
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

// worldPassK is the render scale while a world pass draws into the map's
// offscreen, 0 otherwise; uiScale reads it. Main goroutine only, written
// only when the render scale is reduced (as curUIScale is).
var worldPassK float64

// worldDefer is the queue of canvas draws aimed at the map's offscreen;
// nil unless a world pass is drawing into one.
var worldDefer *worldDeferQueue

// worldDeferQueue is one frame's queued canvas draws.
type worldDeferQueue struct {
	world  *ebiten.Image // the offscreen the map draws into
	ops    []worldDeferOp
	hidden func(lx, ly float32) bool // a fogged anchor (logical): dropped
}

// worldDeferOp is one queued draw and the trace tags it was made with.
type worldDeferOp struct {
	draw      func(dst *ebiten.Image)
	pass, via string
}

// worldRender is an App's render-scaled map: the offscreen and the queue.
type worldRender struct {
	img    *ebiten.Image
	q      worldDeferQueue
	hidden func(lx, ly float32) bool // a.fogHidesLabel, bound once
}

// deferring reports a canvas draw into dst must be queued: dst is the
// render-scaled map image.
func deferring(dst *ebiten.Image) bool {
	return worldDefer != nil && dst == worldDefer.world
}

// queueWorld queues draw, anchored at the logical point (lx, ly).
func queueWorld(lx, ly float32, draw func(*ebiten.Image)) {
	q := worldDefer
	if q.hidden != nil && q.hidden(lx, ly) {
		return
	}
	q.ops = append(q.ops, worldDeferOp{draw: draw, pass: tracePass, via: traceVia})
}

// fogDrawn ends the fog check: what a world pass draws after the fog shows
// over it.
func (q *worldDeferQueue) fogDrawn() {
	if q != nil {
		q.hidden = nil
	}
}

// The deferred forms of the canvas helpers. Each captures its arguments
// by value (slices and options copied: callers reuse their buffers) in a
// function of its own, so the helpers' own arguments never escape.

func deferFillRect(x, y, w, h float32, clr color.Color, aa bool) {
	queueWorld(x+w/2, y+h/2, func(d *ebiten.Image) { uiFillRect(d, x, y, w, h, clr, aa) })
}

func deferStrokeRect(x, y, w, h, width float32, clr color.Color, aa bool) {
	queueWorld(x+w/2, y+h/2, func(d *ebiten.Image) { uiStrokeRect(d, x, y, w, h, width, clr, aa) })
}

func deferLine(x0, y0, x1, y1, width float32, clr color.Color, aa bool) {
	queueWorld((x0+x1)/2, (y0+y1)/2, func(d *ebiten.Image) { uiLine(d, x0, y0, x1, y1, width, clr, aa) })
}

// deferCircle is a filled circle (width 0) or a stroked one.
func deferCircle(cx, cy, r, width float32, clr color.Color, aa bool) {
	queueWorld(cx, cy, func(d *ebiten.Image) {
		if width == 0 {
			uiFillCircle(d, cx, cy, r, clr, aa)
		} else {
			uiStrokeCircle(d, cx, cy, r, width, clr, aa)
		}
	})
}

func deferPoly(pts [][2]float32, clr color.Color, aa bool) {
	cp := append([][2]float32(nil), pts...)
	queueWorld(cp[0][0], cp[0][1], func(d *ebiten.Image) { uiFillPoly(d, cp, clr, aa) })
}

func deferDrawImage(img *ebiten.Image, op ebiten.DrawImageOptions) {
	b := img.Bounds()
	x, y := op.GeoM.Apply(float64(b.Dx())/2, float64(b.Dy())/2)
	queueWorld(float32(x), float32(y), func(d *ebiten.Image) { uiDrawImage(d, img, &op) })
}

func deferImage(img *ebiten.Image, x, y, w, h float64, op ebiten.DrawImageOptions) {
	queueWorld(float32(x+w/2), float32(y+h/2), func(d *ebiten.Image) { uiImage(d, img, x, y, w, h, &op) })
}

func deferTriangles(vs []ebiten.Vertex, is16 []uint16, is32 []uint32, img *ebiten.Image, opts *ebiten.DrawTrianglesOptions) {
	if len(vs) == 0 {
		return
	}
	cv := append([]ebiten.Vertex(nil), vs...)
	c16 := append([]uint16(nil), is16...)
	c32 := append([]uint32(nil), is32...)
	var o *ebiten.DrawTrianglesOptions
	if opts != nil {
		oc := *opts
		o = &oc
	}
	wide := is32 != nil
	queueWorld(cv[0].DstX, cv[0].DstY, func(d *ebiten.Image) {
		if wide {
			uiTriangles32(d, cv, c32, img, o)
		} else {
			uiTriangles(d, cv, c16, img, o)
		}
	})
}

func deferText(s string, x, y float64, st textStyle) {
	queueWorld(float32(x), float32(y), func(d *ebiten.Image) { drawText(d, s, x, y, st) })
}

// worldTarget is where the colony map draws this frame: the screen, or at
// a reduced render scale the map's offscreen, ceil(physical x world)
// pixels (true). The offscreen is freed when the scale returns to 1.
func (a *App) worldTarget(screen *ebiten.Image) (*ebiten.Image, bool) {
	w := &a.wr
	k := a.scale.worldK()
	if k == 1 {
		if w.img != nil {
			w.img.Deallocate()
			w.img = nil
		}
		return screen, false
	}
	b := screen.Bounds()
	w.img = fitLayer(w.img, int(math.Ceil(float64(b.Dx())*k)), int(math.Ceil(float64(b.Dy())*k)))
	return w.img, true
}

// beginWorldPass starts drawing into the offscreen img: uiScale() becomes
// s x world and the canvas queues what it is asked to draw into img.
func (a *App) beginWorldPass(img *ebiten.Image) {
	w := &a.wr
	if w.hidden == nil {
		w.hidden = a.fogHidesLabel
	}
	clear(w.q.ops)
	w.q.world, w.q.ops, w.q.hidden = img, w.q.ops[:0], w.hidden
	worldDefer, worldPassK = &w.q, a.scale.worldK()
}

// endWorldPass restores the HUD scale and stops queueing.
func (a *App) endWorldPass() {
	worldDefer, worldPassK = nil, 0
}

// compositeWorld draws the map's offscreen over the whole screen, scaled
// by exactly 1/k (the offscreen is rounded up, so its last partial pixel
// falls off the screen and a map pixel x maps to physical x/k), with
// linear filtering.
func compositeWorld(screen, world *ebiten.Image, k float64) {
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, Blend: ebiten.BlendCopy}
	op.GeoM.Scale(1/k, 1/k)
	screen.DrawImage(world, op)
}

// flushWorldOverlay draws the queued labels and icons on the screen at the
// HUD scale, in the order they were made, each with its trace tags.
func (a *App) flushWorldOverlay(screen *ebiten.Image) {
	q := &a.wr.q
	traced := canvasTrace != nil || drawTrace != nil
	pass, via := tracePass, traceVia
	for i := range q.ops {
		op := &q.ops[i]
		if traced {
			tracePass, traceVia = op.pass, op.via
		}
		op.draw(screen)
	}
	if traced {
		tracePass, traceVia = pass, via
	}
	clear(q.ops)
	q.ops, q.world = q.ops[:0], nil
}

// fogHidesLabel reports the logical point (lx, ly) is over an unexplored
// tile of the frame being drawn: the fog, drawn after the labels, would
// cover a label there.
func (a *App) fogHidesLabel(lx, ly float32) bool {
	r := a.renderer
	if r == nil || len(r.drawW.Fog) == 0 {
		return false
	}
	tx, ty := r.TileAt(int(math.Floor(float64(lx)*a.mapScale())), int(math.Floor(float64(ly)*a.mapScale())))
	return worldFogged(r.drawW, tx, ty)
}
