// Package gfx is the ebiten client's drawing primitives, shared by the HUD
// and the renderer: the UI canvas (logical pixels in, physical out, scaled
// by the UI scale), text, rectangles, the triangle batch, the world pass
// that queues canvas draws aimed at the render-scaled map, and the one
// process-wide canvas state (state.go). It knows nothing of the game.
package gfx

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// TraceEvent is one draw call the canvas saw, in physical pixels (the
// rectangle it covers; a line, circle, polygon or triangle batch reports
// its bounding box, a text run the box of its glyphs).
type TraceEvent struct {
	Kind string // fill, stroke, line, circle, poly, tris, image or text
	Text string // the string, for Kind "text"
	X, Y float32
	W, H float32
	Pass string // cur.tracePass when drawn: "world" or "hud"
	Via  string // "worldText" or "worldIcon" when drawn through those helpers
	// Clip is the destination's bounds (physical pixels): a draw into a
	// clipped sub-image (ClipTo) shows only inside it. Zero: not known.
	Clip image.Rectangle
	// Size is a text run's face size in physical pixels (0 for other
	// kinds): with it a test can tell the glyphs from the line box.
	Size float32
}

// emitTrace reports one event drawn into dst to cur.drawTrace (callers check
// it is set).
func emitTrace(dst *ebiten.Image, kind, txt string, x, y, w, h float32) {
	emitEvent(dst, TraceEvent{Kind: kind, Text: txt, X: x, Y: y, W: w, H: h})
}

// emitEvent reports e, drawn into dst, with the pass, route and clip.
func emitEvent(dst *ebiten.Image, e TraceEvent) {
	e.Pass, e.Via = cur.tracePass, cur.traceVia
	if dst != nil {
		e.Clip = dst.Bounds()
	}
	cur.drawTrace(e)
}

// emitBox reports the bounding box of the physical points (x0,y0), (x1,y1)
// grown by pad on every side.
func emitBox(dst *ebiten.Image, kind string, x0, y0, x1, y1, pad float32) {
	emitTrace(dst, kind, "", min(x0, x1)-pad, min(y0, y1)-pad, Abs32(x1-x0)+2*pad, Abs32(y1-y0)+2*pad)
}

// snapSpan is a logical span [x, x+w) in device pixels at scale s. At a
// whole s it is exact. At a fractional s (1.25, 1.75) the start rounds to
// the nearest device pixel and the length is max(1, ceil(w x s)), so equal
// spans get equal whole widths and an unblended edge (UIS-011).
func snapSpan(x, w, s float32) (float32, float32) {
	if s == float32(int(s)) || w <= 0 {
		return x * s, w * s
	}
	return float32(math.Round(float64(x * s))), max(1, float32(math.Ceil(float64(w*s-0.01))))
}

// keepIn is a snapped length l from p: a span whose true end lies on or
// inside the image's edge never rounds past it (the last tab at a
// fractional scale, which ends on the screen's right edge).
func keepIn(p, l, end float32, edge int) float32 {
	if e := float32(edge); end <= e+0.01 && p+l > e {
		return max(1, e-p)
	}
	return l
}

// snapWidth is a stroke width in device pixels at scale s: whole and at
// least 1 at a fractional s (UIS-011).
func snapWidth(width, s float32) float32 {
	if s == float32(int(s)) {
		return width * s
	}
	return max(1, float32(math.Round(float64(width*s))))
}

// FillRect is vector.FillRect in logical pixels. Without anti-aliasing its
// edges land on whole device pixels (snapSpan); with it, it is one
// triangle batch (aashape), as are every anti-aliased shape here.
func FillRect(dst *ebiten.Image, x, y, w, h float32, clr color.Color, aa bool) {
	if deferring(dst) { // world_scale.go: drawn at native size after the map
		deferFillRect(x, y, w, h, clr, aa)
		return
	}
	s := float32(UIScale())
	px, pw, py, ph := x*s, w*s, y*s, h*s
	if !aa {
		px, pw = snapSpan(x, w, s)
		py, ph = snapSpan(y, h, s)
		if dst != nil {
			pw = keepIn(px, pw, (x+w)*s, dst.Bounds().Dx())
			ph = keepIn(py, ph, (y+h)*s, dst.Bounds().Dy())
		}
	}
	if cur.canvasTrace != nil {
		cur.canvasTrace("fill", px, py, pw, ph)
	}
	if cur.drawTrace != nil {
		emitTrace(dst, "fill", "", px, py, pw, ph)
	}
	if aa {
		cur.aa.Rect(dst, px, py, pw, ph, clr)
		return
	}
	vector.FillRect(dst, px, py, pw, ph, clr, aa)
}

// StrokeRect is vector.StrokeRect in logical pixels. Without anti-aliasing
// its box and stroke land on whole device pixels (snapSpan, snapWidth).
func StrokeRect(dst *ebiten.Image, x, y, w, h, width float32, clr color.Color, aa bool) {
	if deferring(dst) {
		deferStrokeRect(x, y, w, h, width, clr, aa)
		return
	}
	s := float32(UIScale())
	px, pw, py, ph, pwid := x*s, w*s, y*s, h*s, width*s
	if !aa {
		px, pw = snapSpan(x, w, s)
		py, ph = snapSpan(y, h, s)
		pwid = snapWidth(width, s)
	}
	if cur.canvasTrace != nil {
		cur.canvasTrace("stroke", px, py, pw, ph)
	}
	if cur.drawTrace != nil {
		emitTrace(dst, "stroke", "", px, py, pw, ph)
	}
	if aa {
		cur.aa.Frame(dst, px, py, pw, ph, pwid, clr)
		return
	}
	vector.StrokeRect(dst, px, py, pw, ph, pwid, clr, aa)
}

// Line is vector.StrokeLine in logical pixels. A horizontal or vertical
// line without anti-aliasing has a whole width and lands on whole device
// pixels (UIS-011).
func Line(dst *ebiten.Image, x0, y0, x1, y1, width float32, clr color.Color, aa bool) {
	if deferring(dst) {
		deferLine(x0, y0, x1, y1, width, clr, aa)
		return
	}
	s := float32(UIScale())
	px0, py0, px1, py1, pw := x0*s, y0*s, x1*s, y1*s, width*s
	if !aa && s != float32(int(s)) && (x0 == x1 || y0 == y1) {
		pw = snapWidth(width, s)
		// the edge across the line on a whole pixel: centre = edge + pw/2
		across := func(v float32) float32 { return float32(math.Round(float64(v*s-pw/2))) + pw/2 }
		round := func(v float32) float32 { return float32(math.Round(float64(v * s))) }
		if y0 == y1 {
			py0 = across(y0)
			py1, px0, px1 = py0, round(x0), round(x1)
		} else {
			px0 = across(x0)
			px1, py0, py1 = px0, round(y0), round(y1)
		}
	}
	if cur.drawTrace != nil {
		emitBox(dst, "line", px0, py0, px1, py1, 0)
	}
	if aa {
		cur.aa.Line(dst, px0, py0, px1, py1, pw, clr)
		return
	}
	vector.StrokeLine(dst, px0, py0, px1, py1, pw, clr, aa)
}

// FillCircle is vector.FillCircle in logical pixels.
func FillCircle(dst *ebiten.Image, cx, cy, r float32, clr color.Color, aa bool) {
	if deferring(dst) {
		deferCircle(cx, cy, r, 0, clr, aa)
		return
	}
	s := float32(UIScale())
	if cur.drawTrace != nil {
		emitBox(dst, "circle", cx*s, cy*s, cx*s, cy*s, r*s)
	}
	if aa {
		cur.aa.Disc(dst, cx*s, cy*s, r*s, clr)
		return
	}
	vector.FillCircle(dst, cx*s, cy*s, r*s, clr, aa)
}

// StrokeCircle is vector.StrokeCircle in logical pixels.
func StrokeCircle(dst *ebiten.Image, cx, cy, r, width float32, clr color.Color, aa bool) {
	if deferring(dst) {
		deferCircle(cx, cy, r, width, clr, aa)
		return
	}
	s := float32(UIScale())
	if cur.drawTrace != nil {
		emitBox(dst, "circle", cx*s, cy*s, cx*s, cy*s, r*s)
	}
	if aa {
		cur.aa.Ring(dst, cx*s, cy*s, r*s, width*s, clr)
		return
	}
	vector.StrokeCircle(dst, cx*s, cy*s, r*s, width*s, clr, aa)
}

// FillPoly fills the closed polygon through pts (logical pixels). An
// anti-aliased convex one is a triangle batch; a concave one (a star)
// stays on vector's path.
func FillPoly(dst *ebiten.Image, pts [][2]float32, clr color.Color, aa bool) {
	if len(pts) < 3 {
		return
	}
	if deferring(dst) {
		deferPoly(pts, clr, aa)
		return
	}
	s := float32(UIScale())
	if cur.drawTrace != nil {
		x0, y0, x1, y1 := pts[0][0]*s, pts[0][1]*s, pts[0][0]*s, pts[0][1]*s
		for _, q := range pts[1:] {
			x0, y0 = min(x0, q[0]*s), min(y0, q[1]*s)
			x1, y1 = max(x1, q[0]*s), max(y1, q[1]*s)
		}
		emitBox(dst, "poly", x0, y0, x1, y1, 0)
	}
	if aa && cur.aa.ConvexPoly(dst, pts, s, clr) {
		return
	}
	var p vector.Path
	p.MoveTo(pts[0][0]*s, pts[0][1]*s)
	for _, q := range pts[1:] {
		p.LineTo(q[0]*s, q[1]*s)
	}
	p.Close()
	op := &vector.DrawPathOptions{AntiAlias: aa}
	op.ColorScale.ScaleWithColor(clr)
	vector.FillPath(dst, &p, nil, op)
}

// PxDisc fills the circle of radius r round (cx, cy), anti-aliased, in
// physical pixels: no UI scale, no world-pass deferral (the world layers'
// discs, at camera positions).
func PxDisc(dst *ebiten.Image, cx, cy, r float32, clr color.Color) {
	cur.aa.Disc(dst, cx, cy, r, clr)
}

// PxRing strokes the circle of radius r round (cx, cy), width wide,
// anti-aliased, in physical pixels.
func PxRing(dst *ebiten.Image, cx, cy, r, width float32, clr color.Color) {
	cur.aa.Ring(dst, cx, cy, r, width, clr)
}

// PxLine strokes (x0, y0)-(x1, y1), width wide with butt caps,
// anti-aliased, in physical pixels.
func PxLine(dst *ebiten.Image, x0, y0, x1, y1, width float32, clr color.Color) {
	cur.aa.Line(dst, x0, y0, x1, y1, width, clr)
}

// PxFrame strokes the rectangle (x, y, w, h), width wide and centred on its
// edges, anti-aliased, in physical pixels.
func PxFrame(dst *ebiten.Image, x, y, w, h, width float32, clr color.Color) {
	cur.aa.Frame(dst, x, y, w, h, width, clr)
}

// ImageOp appends the UI scale to op's GeoM, for code that positions
// an image in logical pixels with its own GeoM. It changes op in place:
// call it once, after the GeoM is built.
func ImageOp(op *ebiten.DrawImageOptions) {
	if s := UIScale(); s != 1 {
		op.GeoM.Scale(s, s)
	}
}

// DrawImage is dst.DrawImage for an op whose GeoM is in logical
// pixels; op is not changed.
func DrawImage(dst, img *ebiten.Image, op *ebiten.DrawImageOptions) {
	if op == nil {
		op = &ebiten.DrawImageOptions{}
	}
	if deferring(dst) {
		deferDrawImage(img, *op)
		return
	}
	o := *op
	ImageOp(&o)
	if cur.canvasTrace != nil || cur.drawTrace != nil {
		traceImage(dst, img, o.GeoM)
	}
	dst.DrawImage(img, &o)
}

// traceImage reports the physical bounding box of img drawn through m.
func traceImage(dst, img *ebiten.Image, m ebiten.GeoM) {
	b := img.Bounds()
	x0, y0 := m.Apply(0, 0)
	x1, y1 := m.Apply(float64(b.Dx()), float64(b.Dy()))
	traceImageRect(dst, float32(min(x0, x1)), float32(min(y0, y1)), float32(abs64(x1-x0)), float32(abs64(y1-y0)))
}

// traceImageRect reports an image draw to both hooks.
func traceImageRect(dst *ebiten.Image, x, y, w, h float32) {
	if cur.canvasTrace != nil {
		cur.canvasTrace("image", x, y, w, h)
	}
	if cur.drawTrace != nil {
		emitTrace(dst, "image", "", x, y, w, h)
	}
}

func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// WorldText draws s at a world-pass position (physical pixels, e.g. from
// worldToScreen) with st scaled with the UI like any label: Text takes
// logical pixels, so the position is divided by the UI scale once here.
// Every world-anchored label goes through this (or WorldIcon), never
// Text with physical coordinates (PLAY-017; TestUIDrawsThroughCanvas).
func WorldText(dst *ebiten.Image, s string, physX, physY float64, st TextStyle) {
	k := 1 / UIScale()
	if cur.drawTrace != nil {
		defer func(old string) { cur.traceVia = old }(cur.traceVia)
		cur.traceVia = "worldText"
	}
	Text(dst, s, physX*k, physY*k, st)
}

// WorldIcon is Icon at a world-pass position (physical pixels); size
// stays logical, so the icon grows with the UI scale like the HUD's.
func WorldIcon(dst, img *ebiten.Image, physX, physY, size float64) {
	k := 1 / UIScale()
	if cur.drawTrace != nil {
		defer func(old string) { cur.traceVia = old }(cur.traceVia)
		cur.traceVia = "worldIcon"
	}
	Icon(dst, img, physX*k, physY*k, size)
}

// Image draws img stretched into the logical rectangle (x, y, w, h).
// opts supplies colour, blend and filter; its GeoM is replaced.
func Image(dst, img *ebiten.Image, x, y, w, h float64, opts *ebiten.DrawImageOptions) {
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return
	}
	var o ebiten.DrawImageOptions
	if opts != nil {
		o = *opts
	}
	if deferring(dst) {
		deferImage(img, x, y, w, h, o)
		return
	}
	o.GeoM.Reset()
	o.GeoM.Scale(w/float64(b.Dx()), h/float64(b.Dy()))
	o.GeoM.Translate(x, y)
	ImageOp(&o)
	if cur.canvasTrace != nil || cur.drawTrace != nil {
		s := float32(UIScale())
		traceImageRect(dst, float32(x)*s, float32(y)*s, float32(w)*s, float32(h)*s)
	}
	dst.DrawImage(img, &o)
}

// Triangles is dst.DrawTriangles for vertices in logical pixels (the
// caller's slice is not changed).
func Triangles(dst *ebiten.Image, vs []ebiten.Vertex, is []uint16, img *ebiten.Image, opts *ebiten.DrawTrianglesOptions) {
	if deferring(dst) {
		deferTriangles(vs, is, nil, img, opts)
		return
	}
	sv := ScaledVerts(vs)
	if cur.drawTrace != nil {
		traceVerts(dst, sv)
	}
	dst.DrawTriangles(sv, is, img, opts)
}

// Triangles32 is Triangles with 32-bit indices.
func Triangles32(dst *ebiten.Image, vs []ebiten.Vertex, is []uint32, img *ebiten.Image, opts *ebiten.DrawTrianglesOptions) {
	if deferring(dst) {
		deferTriangles(vs, nil, is, img, opts)
		return
	}
	sv := ScaledVerts(vs)
	if cur.drawTrace != nil {
		traceVerts(dst, sv)
	}
	dst.DrawTriangles32(sv, is, img, opts)
}

// traceVerts reports the physical bounding box of a triangle batch.
func traceVerts(dst *ebiten.Image, vs []ebiten.Vertex) {
	if len(vs) == 0 {
		return
	}
	x0, y0, x1, y1 := vs[0].DstX, vs[0].DstY, vs[0].DstX, vs[0].DstY
	for _, v := range vs[1:] {
		x0, y0 = min(x0, v.DstX), min(y0, v.DstY)
		x1, y1 = max(x1, v.DstX), max(y1, v.DstY)
	}
	emitBox(dst, "tris", x0, y0, x1, y1, 0)
}

func ScaledVerts(vs []ebiten.Vertex) []ebiten.Vertex {
	s := float32(UIScale())
	if s == 1 {
		return vs
	}
	out := make([]ebiten.Vertex, len(vs))
	for i, v := range vs {
		v.DstX, v.DstY = v.DstX*s, v.DstY*s
		out[i] = v
	}
	return out
}

// PhysRect is a logical rectangle in physical pixels (for code that
// draws physical content, such as the globe or the minimap image, into a
// UI-laid-out box).
func PhysRect(r Rect) Rect {
	s := float32(UIScale())
	return Rect{r.X * s, r.Y * s, r.W * s, r.H * s}
}

// String is the event for a failure message: kind, text, box, pass and via.
func (e TraceEvent) String() string {
	txt := ""
	if e.Text != "" {
		txt = " " + strconv.Quote(e.Text)
	}
	via := ""
	if e.Via != "" {
		via = " via " + e.Via
	}
	return fmt.Sprintf("%s%s %.1f,%.1f %.1fx%.1f [%s%s]", e.Kind, txt, e.X, e.Y, e.W, e.H, e.Pass, via)
}
