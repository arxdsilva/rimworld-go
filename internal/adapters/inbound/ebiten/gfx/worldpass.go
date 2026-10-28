package gfx

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// WorldPass is one frame's queued canvas draws.
type WorldPass struct {
	world  *ebiten.Image // the offscreen the map draws into
	ops    []worldOp
	hidden func(lx, ly float32) bool // a fogged anchor (logical): dropped
}

// worldOp is one queued draw and the trace tags it was made with.
type worldOp struct {
	draw      func(dst *ebiten.Image)
	pass, via string
}

// deferring reports a canvas draw into dst must be queued: dst is the
// render-scaled map image.
func deferring(dst *ebiten.Image) bool {
	return cur.pass != nil && dst == cur.pass.world
}

// queueWorld queues draw, anchored at the logical point (lx, ly).
func queueWorld(lx, ly float32, draw func(*ebiten.Image)) {
	q := cur.pass
	if q.hidden != nil && q.hidden(lx, ly) {
		return
	}
	q.ops = append(q.ops, worldOp{draw: draw, pass: cur.tracePass, via: cur.traceVia})
}

// FogDrawn ends the fog check: what a world pass draws after the fog shows
// over it.
func (q *WorldPass) FogDrawn() {
	if q != nil {
		q.hidden = nil
	}
}

func deferFillRect(x, y, w, h float32, clr color.Color, aa bool) {
	queueWorld(x+w/2, y+h/2, func(d *ebiten.Image) { FillRect(d, x, y, w, h, clr, aa) })
}

func deferStrokeRect(x, y, w, h, width float32, clr color.Color, aa bool) {
	queueWorld(x+w/2, y+h/2, func(d *ebiten.Image) { StrokeRect(d, x, y, w, h, width, clr, aa) })
}

func deferLine(x0, y0, x1, y1, width float32, clr color.Color, aa bool) {
	queueWorld((x0+x1)/2, (y0+y1)/2, func(d *ebiten.Image) { Line(d, x0, y0, x1, y1, width, clr, aa) })
}

// deferCircle is a filled circle (width 0) or a stroked one.
func deferCircle(cx, cy, r, width float32, clr color.Color, aa bool) {
	queueWorld(cx, cy, func(d *ebiten.Image) {
		if width == 0 {
			FillCircle(d, cx, cy, r, clr, aa)
		} else {
			StrokeCircle(d, cx, cy, r, width, clr, aa)
		}
	})
}

func deferPoly(pts [][2]float32, clr color.Color, aa bool) {
	cp := append([][2]float32(nil), pts...)
	queueWorld(cp[0][0], cp[0][1], func(d *ebiten.Image) { FillPoly(d, cp, clr, aa) })
}

func deferDrawImage(img *ebiten.Image, op ebiten.DrawImageOptions) {
	b := img.Bounds()
	x, y := op.GeoM.Apply(float64(b.Dx())/2, float64(b.Dy())/2)
	queueWorld(float32(x), float32(y), func(d *ebiten.Image) { DrawImage(d, img, &op) })
}

func deferImage(img *ebiten.Image, x, y, w, h float64, op ebiten.DrawImageOptions) {
	queueWorld(float32(x+w/2), float32(y+h/2), func(d *ebiten.Image) { Image(d, img, x, y, w, h, &op) })
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
			Triangles32(d, cv, c32, img, o)
		} else {
			Triangles(d, cv, c16, img, o)
		}
	})
}

func deferText(s string, x, y float64, st TextStyle) {
	queueWorld(float32(x), float32(y), func(d *ebiten.Image) { Text(d, s, x, y, st) })
}

// CompositeWorld draws the map's offscreen over the whole screen, scaled
// by exactly 1/k (the offscreen is rounded up, so its last partial pixel
// falls off the screen and a map pixel x maps to physical x/k), with
// linear filtering.
func CompositeWorld(screen, world *ebiten.Image, k float64) {
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, Blend: ebiten.BlendCopy}
	op.GeoM.Scale(1/k, 1/k)
	screen.DrawImage(world, op)
}
