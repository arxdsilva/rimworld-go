package ebiten

import (
	"image"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The UI canvas (PLAY-012 B.2): every screen-layout draw (panels,
// buttons, text, icons, HUD, menus, tooltips) goes through these helpers.
// They take logical coordinates and draw at physical resolution, x the UI
// scale (curUIScale.s). Signatures mirror the vector package, so a raw
// vector.FillRect(...) becomes uiFillRect(...) with the same arguments.
// Stroke widths scale too. At s = 1 every call is exactly the raw one, so
// captures stay byte-identical.
//
// World-space drawing (anything the camera positions) stays raw, in
// physical pixels; see TestUIDrawsThroughCanvas for the functions allowed to,
// and for the rule that world-anchored labels use worldText / worldIcon.
// With a reduced map render scale (world_scale.go) a canvas draw aimed at
// the map's offscreen is queued and drawn on the screen after the map is
// composited, so labels and icons stay at native resolution.

// canvasTrace, when set (tests only, never in parallel), sees each
// rectangle the canvas fills, strokes or draws an image into, in
// physical pixels (unit tests cannot read GPU pixels back).
var canvasTrace func(kind string, x, y, w, h float32)

// tracePass is the draw pass running now: "world" while the renderer
// draws the map (camera-positioned, physical pixels), "hud" otherwise
// (App.Draw sets it; the visual tests read it, PLAY-017).
var tracePass = "hud"

// setTracePass sets tracePass, only while a test trace is installed (those
// tests never run in parallel; an unconditional write would race between
// parallel tests that draw frames).
func setTracePass(p string) {
	if canvasTrace != nil || drawTrace != nil {
		tracePass = p
	}
}

// traceEvent is one draw call the canvas saw, in physical pixels (the
// rectangle it covers; a line, circle, polygon or triangle batch reports
// its bounding box, a text run the box of its glyphs).
type traceEvent struct {
	Kind string // fill, stroke, line, circle, poly, tris, image or text
	Text string // the string, for Kind "text"
	X, Y float32
	W, H float32
	Pass string // tracePass when drawn: "world" or "hud"
	Via  string // "worldText" or "worldIcon" when drawn through those helpers
	// Clip is the destination's bounds (physical pixels): a draw into a
	// clipped sub-image (clipTo) shows only inside it. Zero: not known.
	Clip image.Rectangle
}

// drawTrace, when set (tests only, never in parallel), sees EVERY draw the
// canvas makes: everything canvasTrace sees plus text, lines, circles,
// polygons and triangle batches, tagged with the pass they were drawn in
// (PLAY-017 visual tests). It is a separate hook so canvasTrace's exact
// fill/stroke/image streams, which older tests count, never change. It
// costs one nil check per draw when unset.
var drawTrace func(e traceEvent)

// traceVia is set by worldText/worldIcon while they draw (see traceEvent.Via).
var traceVia string

// emitTrace reports one event drawn into dst to drawTrace (callers check
// it is set).
func emitTrace(dst *ebiten.Image, kind, txt string, x, y, w, h float32) {
	e := traceEvent{Kind: kind, Text: txt, X: x, Y: y, W: w, H: h, Pass: tracePass, Via: traceVia}
	if dst != nil {
		e.Clip = dst.Bounds()
	}
	drawTrace(e)
}

// emitBox reports the bounding box of the physical points (x0,y0), (x1,y1)
// grown by pad on every side.
func emitBox(dst *ebiten.Image, kind string, x0, y0, x1, y1, pad float32) {
	emitTrace(dst, kind, "", min(x0, x1)-pad, min(y0, y1)-pad, abs32(x1-x0)+2*pad, abs32(y1-y0)+2*pad)
}

// uiFillRect is vector.FillRect in logical pixels.
func uiFillRect(dst *ebiten.Image, x, y, w, h float32, clr color.Color, aa bool) {
	if deferring(dst) { // world_scale.go: drawn at native size after the map
		deferFillRect(x, y, w, h, clr, aa)
		return
	}
	s := float32(uiScale())
	if canvasTrace != nil {
		canvasTrace("fill", x*s, y*s, w*s, h*s)
	}
	if drawTrace != nil {
		emitTrace(dst, "fill", "", x*s, y*s, w*s, h*s)
	}
	vector.FillRect(dst, x*s, y*s, w*s, h*s, clr, aa)
}

// uiStrokeRect is vector.StrokeRect in logical pixels.
func uiStrokeRect(dst *ebiten.Image, x, y, w, h, width float32, clr color.Color, aa bool) {
	if deferring(dst) {
		deferStrokeRect(x, y, w, h, width, clr, aa)
		return
	}
	s := float32(uiScale())
	if canvasTrace != nil {
		canvasTrace("stroke", x*s, y*s, w*s, h*s)
	}
	if drawTrace != nil {
		emitTrace(dst, "stroke", "", x*s, y*s, w*s, h*s)
	}
	vector.StrokeRect(dst, x*s, y*s, w*s, h*s, width*s, clr, aa)
}

// uiLine is vector.StrokeLine in logical pixels.
func uiLine(dst *ebiten.Image, x0, y0, x1, y1, width float32, clr color.Color, aa bool) {
	if deferring(dst) {
		deferLine(x0, y0, x1, y1, width, clr, aa)
		return
	}
	s := float32(uiScale())
	if drawTrace != nil {
		emitBox(dst, "line", x0*s, y0*s, x1*s, y1*s, 0)
	}
	vector.StrokeLine(dst, x0*s, y0*s, x1*s, y1*s, width*s, clr, aa)
}

// uiFillCircle is vector.FillCircle in logical pixels.
func uiFillCircle(dst *ebiten.Image, cx, cy, r float32, clr color.Color, aa bool) {
	if deferring(dst) {
		deferCircle(cx, cy, r, 0, clr, aa)
		return
	}
	s := float32(uiScale())
	if drawTrace != nil {
		emitBox(dst, "circle", cx*s, cy*s, cx*s, cy*s, r*s)
	}
	vector.FillCircle(dst, cx*s, cy*s, r*s, clr, aa)
}

// uiStrokeCircle is vector.StrokeCircle in logical pixels.
func uiStrokeCircle(dst *ebiten.Image, cx, cy, r, width float32, clr color.Color, aa bool) {
	if deferring(dst) {
		deferCircle(cx, cy, r, width, clr, aa)
		return
	}
	s := float32(uiScale())
	if drawTrace != nil {
		emitBox(dst, "circle", cx*s, cy*s, cx*s, cy*s, r*s)
	}
	vector.StrokeCircle(dst, cx*s, cy*s, r*s, width*s, clr, aa)
}

// uiFillPoly fills the closed polygon through pts (logical pixels).
func uiFillPoly(dst *ebiten.Image, pts [][2]float32, clr color.Color, aa bool) {
	if len(pts) < 3 {
		return
	}
	if deferring(dst) {
		deferPoly(pts, clr, aa)
		return
	}
	s := float32(uiScale())
	if drawTrace != nil {
		x0, y0, x1, y1 := pts[0][0]*s, pts[0][1]*s, pts[0][0]*s, pts[0][1]*s
		for _, q := range pts[1:] {
			x0, y0 = min(x0, q[0]*s), min(y0, q[1]*s)
			x1, y1 = max(x1, q[0]*s), max(y1, q[1]*s)
		}
		emitBox(dst, "poly", x0, y0, x1, y1, 0)
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

// uiImageOp appends the UI scale to op's GeoM, for code that positions
// an image in logical pixels with its own GeoM. It changes op in place:
// call it once, after the GeoM is built.
func uiImageOp(op *ebiten.DrawImageOptions) {
	if s := uiScale(); s != 1 {
		op.GeoM.Scale(s, s)
	}
}

// uiDrawImage is dst.DrawImage for an op whose GeoM is in logical
// pixels; op is not changed.
func uiDrawImage(dst, img *ebiten.Image, op *ebiten.DrawImageOptions) {
	if op == nil {
		op = &ebiten.DrawImageOptions{}
	}
	if deferring(dst) {
		deferDrawImage(img, *op)
		return
	}
	o := *op
	uiImageOp(&o)
	if canvasTrace != nil || drawTrace != nil {
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
	if canvasTrace != nil {
		canvasTrace("image", x, y, w, h)
	}
	if drawTrace != nil {
		emitTrace(dst, "image", "", x, y, w, h)
	}
}

func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// worldText draws s at a world-pass position (physical pixels, e.g. from
// worldToScreen) with st scaled with the UI like any label: drawText takes
// logical pixels, so the position is divided by the UI scale once here.
// Every world-anchored label goes through this (or worldIcon), never
// drawText with physical coordinates (PLAY-017; TestUIDrawsThroughCanvas).
func worldText(dst *ebiten.Image, s string, physX, physY float64, st textStyle) {
	k := 1 / uiScale()
	if drawTrace != nil {
		defer func(old string) { traceVia = old }(traceVia)
		traceVia = "worldText"
	}
	drawText(dst, s, physX*k, physY*k, st)
}

// worldIcon is drawIcon at a world-pass position (physical pixels); size
// stays logical, so the icon grows with the UI scale like the HUD's.
func worldIcon(dst, img *ebiten.Image, physX, physY, size float64) {
	k := 1 / uiScale()
	if drawTrace != nil {
		defer func(old string) { traceVia = old }(traceVia)
		traceVia = "worldIcon"
	}
	drawIcon(dst, img, physX*k, physY*k, size)
}

// uiImage draws img stretched into the logical rectangle (x, y, w, h).
// opts supplies colour, blend and filter; its GeoM is replaced.
func uiImage(dst, img *ebiten.Image, x, y, w, h float64, opts *ebiten.DrawImageOptions) {
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
	uiImageOp(&o)
	if canvasTrace != nil || drawTrace != nil {
		s := float32(uiScale())
		traceImageRect(dst, float32(x)*s, float32(y)*s, float32(w)*s, float32(h)*s)
	}
	dst.DrawImage(img, &o)
}

// uiTriangles is dst.DrawTriangles for vertices in logical pixels (the
// caller's slice is not changed).
func uiTriangles(dst *ebiten.Image, vs []ebiten.Vertex, is []uint16, img *ebiten.Image, opts *ebiten.DrawTrianglesOptions) {
	if deferring(dst) {
		deferTriangles(vs, is, nil, img, opts)
		return
	}
	sv := scaledVerts(vs)
	if drawTrace != nil {
		traceVerts(dst, sv)
	}
	dst.DrawTriangles(sv, is, img, opts)
}

// uiTriangles32 is uiTriangles with 32-bit indices.
func uiTriangles32(dst *ebiten.Image, vs []ebiten.Vertex, is []uint32, img *ebiten.Image, opts *ebiten.DrawTrianglesOptions) {
	if deferring(dst) {
		deferTriangles(vs, nil, is, img, opts)
		return
	}
	sv := scaledVerts(vs)
	if drawTrace != nil {
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

func scaledVerts(vs []ebiten.Vertex) []ebiten.Vertex {
	s := float32(uiScale())
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

// uiPhysRect is a logical rectangle in physical pixels (for code that
// draws physical content, such as the globe or the minimap image, into a
// UI-laid-out box).
func uiPhysRect(r rect) rect {
	s := float32(uiScale())
	return rect{r.X * s, r.Y * s, r.W * s, r.H * s}
}
