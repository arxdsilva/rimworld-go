package aashape

import (
	"image/color"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

// Painter draws each shape at once as one DrawTriangles32 from a 3x3 white
// source, in call order, so consecutive shapes merge into one draw command.
// Its mesh is reused (no allocation per shape once warm). The zero value is
// ready; the mutex keeps parallel tests that draw frames from sharing the
// scratch mid-shape.
type Painter struct {
	mu    sync.Mutex
	mesh  Mesh
	white *ebiten.Image
	op    ebiten.DrawTrianglesOptions
}

// Disc fills the circle of radius r round (cx, cy).
func (p *Painter) Disc(dst *ebiten.Image, cx, cy, r float32, clr color.Color) {
	c := p.begin(clr)
	p.mesh.Disc(cx, cy, r, c)
	p.flush(dst)
}

// Ring strokes the circle of radius r round (cx, cy), width wide.
func (p *Painter) Ring(dst *ebiten.Image, cx, cy, r, width float32, clr color.Color) {
	c := p.begin(clr)
	p.mesh.Ring(cx, cy, r, width, c)
	p.flush(dst)
}

// Line strokes (x0, y0)-(x1, y1), width wide, with butt caps.
func (p *Painter) Line(dst *ebiten.Image, x0, y0, x1, y1, width float32, clr color.Color) {
	c := p.begin(clr)
	p.mesh.Line(x0, y0, x1, y1, width, c)
	p.flush(dst)
}

// Rect fills the rectangle (x, y, w, h).
func (p *Painter) Rect(dst *ebiten.Image, x, y, w, h float32, clr color.Color) {
	c := p.begin(clr)
	p.mesh.Rect(x, y, w, h, c)
	p.flush(dst)
}

// Frame strokes the rectangle (x, y, w, h), width wide, centred on its edges.
func (p *Painter) Frame(dst *ebiten.Image, x, y, w, h, width float32, clr color.Color) {
	c := p.begin(clr)
	p.mesh.Frame(x, y, w, h, width, c)
	p.flush(dst)
}

// ConvexPoly fills the convex polygon through pts scaled by k; false (and
// nothing drawn) when Mesh.ConvexPoly refuses it.
func (p *Painter) ConvexPoly(dst *ebiten.Image, pts [][2]float32, k float32, clr color.Color) bool {
	c := p.begin(clr)
	ok := p.mesh.ConvexPoly(pts, k, c)
	p.flush(dst)
	return ok
}

// begin takes the painter and empties its mesh; flush releases it.
func (p *Painter) begin(clr color.Color) Color {
	p.mu.Lock()
	p.mesh.Reset()
	return ColorOf(clr)
}

// flush draws the mesh into dst and releases the painter.
func (p *Painter) flush(dst *ebiten.Image) {
	defer p.mu.Unlock()
	if len(p.mesh.Idx) == 0 {
		return
	}
	if p.white == nil {
		p.white = ebiten.NewImage(3, 3)
		pix := make([]byte, 4*3*3)
		for i := range pix {
			pix[i] = 0xff
		}
		p.white.WritePixels(pix) // packs onto an atlas, unlike Fill
		p.op.ColorScaleMode = ebiten.ColorScaleModePremultipliedAlpha
	}
	dst.DrawTriangles32(p.mesh.Verts, p.mesh.Idx, p.white, &p.op)
}
