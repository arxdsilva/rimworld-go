package ebiten

import (
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
// physical pixels; see TestUIDrawsThroughCanvas for the files allowed to.

// canvasTrace, when set (tests only, never in parallel), sees each
// rectangle the canvas fills, strokes or draws an image into, in
// physical pixels (unit tests cannot read GPU pixels back).
var canvasTrace func(kind string, x, y, w, h float32)

// uiFillRect is vector.FillRect in logical pixels.
func uiFillRect(dst *ebiten.Image, x, y, w, h float32, clr color.Color, aa bool) {
	s := float32(uiScale())
	if canvasTrace != nil {
		canvasTrace("fill", x*s, y*s, w*s, h*s)
	}
	vector.FillRect(dst, x*s, y*s, w*s, h*s, clr, aa)
}

// uiStrokeRect is vector.StrokeRect in logical pixels.
func uiStrokeRect(dst *ebiten.Image, x, y, w, h, width float32, clr color.Color, aa bool) {
	s := float32(uiScale())
	if canvasTrace != nil {
		canvasTrace("stroke", x*s, y*s, w*s, h*s)
	}
	vector.StrokeRect(dst, x*s, y*s, w*s, h*s, width*s, clr, aa)
}

// uiLine is vector.StrokeLine in logical pixels.
func uiLine(dst *ebiten.Image, x0, y0, x1, y1, width float32, clr color.Color, aa bool) {
	s := float32(uiScale())
	vector.StrokeLine(dst, x0*s, y0*s, x1*s, y1*s, width*s, clr, aa)
}

// uiFillCircle is vector.FillCircle in logical pixels.
func uiFillCircle(dst *ebiten.Image, cx, cy, r float32, clr color.Color, aa bool) {
	s := float32(uiScale())
	vector.FillCircle(dst, cx*s, cy*s, r*s, clr, aa)
}

// uiStrokeCircle is vector.StrokeCircle in logical pixels.
func uiStrokeCircle(dst *ebiten.Image, cx, cy, r, width float32, clr color.Color, aa bool) {
	s := float32(uiScale())
	vector.StrokeCircle(dst, cx*s, cy*s, r*s, width*s, clr, aa)
}

// uiFillPoly fills the closed polygon through pts (logical pixels).
func uiFillPoly(dst *ebiten.Image, pts [][2]float32, clr color.Color, aa bool) {
	if len(pts) < 3 {
		return
	}
	s := float32(uiScale())
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
	o := *op
	uiImageOp(&o)
	dst.DrawImage(img, &o)
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
	o.GeoM.Reset()
	o.GeoM.Scale(w/float64(b.Dx()), h/float64(b.Dy()))
	o.GeoM.Translate(x, y)
	uiImageOp(&o)
	if canvasTrace != nil {
		s := float32(uiScale())
		canvasTrace("image", float32(x)*s, float32(y)*s, float32(w)*s, float32(h)*s)
	}
	dst.DrawImage(img, &o)
}

// uiTriangles is dst.DrawTriangles for vertices in logical pixels (the
// caller's slice is not changed).
func uiTriangles(dst *ebiten.Image, vs []ebiten.Vertex, is []uint16, img *ebiten.Image, opts *ebiten.DrawTrianglesOptions) {
	dst.DrawTriangles(scaledVerts(vs), is, img, opts)
}

// uiTriangles32 is uiTriangles with 32-bit indices.
func uiTriangles32(dst *ebiten.Image, vs []ebiten.Vertex, is []uint32, img *ebiten.Image, opts *ebiten.DrawTrianglesOptions) {
	dst.DrawTriangles32(scaledVerts(vs), is, img, opts)
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
