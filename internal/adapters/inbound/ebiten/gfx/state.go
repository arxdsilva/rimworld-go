package gfx

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// cur is the canvas's process-wide state: the UI scale the free draw
// functions read, the world pass that queues draws aimed at the
// render-scaled map, and the test traces. There is one GPU canvas per
// process and the draw helpers are free functions by design (hundreds of
// call sites), so this is the one documented exception to SOLID_GO.md §9:
// main goroutine only, written by the App at the start of Update and
// Draw and only on a change, so parallel tests at the default scale only
// read it; tests that install a trace or another scale never run in
// parallel. It is listed in internal/arch/solid_budget.txt.
var cur = struct {
	scale Scale
	passK float64    // the render scale while a world pass draws into the map's offscreen, 0 otherwise
	pass  *WorldPass // the world pass drawing into an offscreen, nil otherwise
	// tracePass is the draw pass running now: "world" while the renderer
	// draws the map (camera-positioned, physical pixels), "hud" otherwise.
	// traceVia is "worldText" or "worldIcon" while those helpers draw.
	tracePass, traceVia string
	// canvasTrace, when set (tests only), sees each rectangle the canvas
	// fills, strokes or draws an image into, in physical pixels (unit
	// tests cannot read GPU pixels back). drawTrace sees EVERY draw the
	// canvas makes, tagged with the pass it was drawn in (PLAY-017); it is
	// a separate hook so canvasTrace's exact fill/stroke/image streams,
	// which older tests count, never change.
	canvasTrace func(kind string, x, y, w, h float32)
	drawTrace   func(TraceEvent)
}{scale: UnitScale, tracePass: "hud"}

// SetScale makes s the scale the draw helpers use; a no-op when unchanged.
func SetScale(s Scale) {
	if cur.scale != s {
		cur.scale = s
	}
}

// CurrentScale is the scale the draw helpers use.
func CurrentScale() Scale { return cur.scale }

// BeginWorldPass starts drawing into the map's offscreen: UIScale becomes
// s x k and the canvas queues what it is asked to draw into p's image.
func BeginWorldPass(p *WorldPass, k float64) { cur.pass, cur.passK = p, k }

// EndWorldPass restores the HUD scale and stops queueing.
func EndWorldPass() { cur.pass, cur.passK = nil, 0 }

// WorldPassK is the render scale of the world pass drawing now, 0 when none.
func WorldPassK() float64 { return cur.passK }

// CurrentPass is the world pass drawing into an offscreen now, nil when none.
func CurrentPass() *WorldPass { return cur.pass }

// FogDrawn ends the current world pass's fog check (see WorldPass.FogDrawn).
func FogDrawn() { cur.pass.FogDrawn() }

// SetPass names the draw pass running now ("world" or "hud"), only while a
// trace is installed: an unconditional write would race between parallel
// tests that draw frames.
func SetPass(p string) {
	if Traced() {
		cur.tracePass = p
	}
}

// ForcePass names the pass unconditionally (tests that build a frame by hand).
func ForcePass(p string) { cur.tracePass = p }

// Pass is the draw pass running now.
func Pass() string { return cur.tracePass }

// SetVia names the helper a world-anchored draw goes through ("" when none).
func SetVia(v string) { cur.traceVia = v }

// Via is the helper a world-anchored draw goes through.
func Via() string { return cur.traceVia }

// Traced reports a test trace is installed.
func Traced() bool { return cur.canvasTrace != nil || cur.drawTrace != nil }

// SetCanvasTrace installs (or with nil removes) the fill/stroke/image trace.
func SetCanvasTrace(f func(kind string, x, y, w, h float32)) { cur.canvasTrace = f }

// SetDrawTrace installs (or with nil removes) the every-draw trace.
func SetDrawTrace(f func(TraceEvent)) { cur.drawTrace = f }

// Flush draws the pass's queued labels and icons on the screen at the HUD
// scale, in the order they were made, each with its trace tags.
func (q *WorldPass) Flush(screen *ebiten.Image) {
	traced := Traced()
	pass, via := cur.tracePass, cur.traceVia
	for i := range q.ops {
		op := &q.ops[i]
		if traced {
			cur.tracePass, cur.traceVia = op.pass, op.via
		}
		op.draw(screen)
	}
	if traced {
		cur.tracePass, cur.traceVia = pass, via
	}
	clear(q.ops)
	q.ops, q.world = q.ops[:0], nil
}

// Len is the number of draws queued.
func (q *WorldPass) Len() int { return len(q.ops) }

// Begin points the pass at the offscreen img with the fog test for
// anchors (nil: none), reusing the queue's storage.
func (q *WorldPass) Begin(img *ebiten.Image, hidden func(lx, ly float32) bool) {
	clear(q.ops)
	q.world, q.ops, q.hidden = img, q.ops[:0], hidden
}

// Icon draws img scaled to size x size at the logical point (x, y).
func Icon(dst, img *ebiten.Image, x, y, size float64) {
	if img == nil {
		return
	}
	s := size / float64(img.Bounds().Dx())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear
	DrawImage(dst, img, op)
}
