package gfx

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// TriBatch is a reusable triangle batch (uint32 indices).
type TriBatch struct {
	Verts []ebiten.Vertex
	Idx   []uint32
	// k scales every vertex position (DstX/DstY, not texels); zero is 1.
	K float32
}

func (b *TriBatch) Reset() { b.Verts, b.Idx = b.Verts[:0], b.Idx[:0] }

// Vert adds a vertex at screen (x, y), texel (sx, sy), straight-alpha
// colour c at alpha a; it returns its index.
func (b *TriBatch) Vert(x, y, sx, sy float64, c color.RGBA, a float64) uint32 {
	dx, dy := float32(x), float32(y)
	if b.K != 0 {
		dx, dy = dx*b.K, dy*b.K
	}
	b.Verts = append(b.Verts, ebiten.Vertex{DstX: dx, DstY: dy, SrcX: float32(sx), SrcY: float32(sy),
		ColorR: float32(c.R) / 255, ColorG: float32(c.G) / 255, ColorB: float32(c.B) / 255, ColorA: float32(Clamp01(a))})
	return uint32(len(b.Verts) - 1)
}

func (b *TriBatch) Tri(i, j, k uint32) { b.Idx = append(b.Idx, i, j, k) }

// Rect is a solid axis-aligned rectangle (screen px) on the white texel,
// with per-corner alphas (top-left, top-right, bottom-left, bottom-right).
func (b *TriBatch) Rect(x, y, w, h float64, c color.RGBA, a00, a10, a01, a11 float64) {
	if w <= 0 || h <= 0 || a00+a10+a01+a11 <= 0 {
		return
	}
	i := b.Vert(x, y, 1.5, 1.5, c, a00)
	b.Vert(x+w, y, 1.5, 1.5, c, a10)
	b.Vert(x, y+h, 1.5, 1.5, c, a01)
	b.Vert(x+w, y+h, 1.5, 1.5, c, a11)
	b.Tri(i, i+1, i+2)
	b.Tri(i+1, i+3, i+2)
}

// Line is a solid stroke from (x0, y0) to (x1, y1), width px wide.
func (b *TriBatch) Line(x0, y0, x1, y1, width float64, c color.RGBA, a float64) {
	dx, dy := x1-x0, y1-y0
	n := math.Hypot(dx, dy)
	if n == 0 || a <= 0 {
		return
	}
	px, py := -dy/n*width/2, dx/n*width/2
	i := b.Vert(x0-px, y0-py, 1.5, 1.5, c, a)
	b.Vert(x0+px, y0+py, 1.5, 1.5, c, a)
	b.Vert(x1-px, y1-py, 1.5, 1.5, c, a)
	b.Vert(x1+px, y1+py, 1.5, 1.5, c, a)
	b.Tri(i, i+1, i+2)
	b.Tri(i+1, i+3, i+2)
}

func (b *TriBatch) Flush(dst, src *ebiten.Image, blend ebiten.Blend) {
	if len(b.Idx) > 0 {
		dst.DrawTriangles32(b.Verts, b.Idx, src, &ebiten.DrawTrianglesOptions{Blend: blend, Filter: ebiten.FilterLinear})
	}
	b.Reset()
}

// FitLayer is img when it is sw x sh, else a new image of that size (the
// old one deallocated).
func FitLayer(img *ebiten.Image, sw, sh int) *ebiten.Image {
	if img != nil && img.Bounds().Dx() == sw && img.Bounds().Dy() == sh {
		return img
	}
	if img != nil {
		img.Deallocate()
	}
	return ebiten.NewImage(sw, sh)
}

func Clamp01(t float64) float64 {
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}

func Abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
