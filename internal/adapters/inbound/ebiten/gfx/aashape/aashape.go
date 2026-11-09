// Package aashape tessellates anti-aliased shapes into triangles: discs,
// rings, lines with butt caps, filled and stroked rectangles and filled
// convex polygons. Each shape's triangles carry a 1-px alpha feather (full
// alpha 0.5 px inside the true edge, none 0.5 px outside it), the box-filter
// coverage a pixel centre sees, so one DrawTriangles32 from a white texel
// draws it without Ebitengine's stencil path (vector.*(…, true) defers to a
// per-destination stencil atlas, which costs CPU and breaks draw merging).
// Below 1 px of width or radius the feather keeps its 1-px ramp and the
// alpha drops to the covered area instead.
//
// Positions are physical pixels. The package holds no state: a Mesh is the
// caller's scratch and a Painter (gfx's canvas state owns the one the
// client uses) draws a shape immediately, in call order.
package aashape

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// Color is a premultiplied colour, each channel in [0, 1].
type Color struct{ R, G, B, A float32 }

// ColorOf is c premultiplied.
func ColorOf(c color.Color) Color {
	r, g, b, a := c.RGBA()
	return Color{float32(r) / 0xffff, float32(g) / 0xffff, float32(b) / 0xffff, float32(a) / 0xffff}
}

// Texel is the source position every vertex samples: the centre texel of
// a 3x3 white image.
const Texel = 1.5

// maxSegs caps a circle's side count.
const maxSegs = 1024

// sagitta is a circle's chord's largest distance from its arc (px). The
// polygon's vertices then sit a third of it outside the circle and the
// chords' midpoints two thirds inside, within 0.05 px.
const sagitta = 0.075

// Mesh is a reusable vertex and index buffer the shapes append to.
type Mesh struct {
	Verts []ebiten.Vertex
	Idx   []uint32
}

// Reset empties the mesh, keeping its storage.
func (m *Mesh) Reset() { m.Verts, m.Idx = m.Verts[:0], m.Idx[:0] }

// vert adds a vertex at (x, y) of colour c at coverage a; it returns its index.
func (m *Mesh) vert(x, y float32, c Color, a float32) uint32 {
	m.Verts = append(m.Verts, ebiten.Vertex{DstX: x, DstY: y, SrcX: Texel, SrcY: Texel,
		ColorR: c.R * a, ColorG: c.G * a, ColorB: c.B * a, ColorA: c.A * a})
	return uint32(len(m.Verts) - 1)
}

func (m *Mesh) tri(i, j, k uint32) { m.Idx = append(m.Idx, i, j, k) }

// band is the box-filtered profile across a solid span w long: peak within
// in of its centre, zero beyond out, linear between (out-in is always 1).
// A span under 1 px keeps the 1-px ramp and its peak drops to w, so the
// covered area stays w.
func band(w float32) (in, out, peak float32) {
	if w >= 1 {
		return (w - 1) / 2, (w + 1) / 2, 1
	}
	return (1 - w) / 2, (1 + w) / 2, w
}

// segments is the side count of a circle of radius r whose chords stay
// within sagitta of it.
func segments(r float32) int {
	if r <= 2*sagitta {
		return 8
	}
	n := int(math.Ceil(math.Pi / math.Acos(1-sagitta/float64(r))))
	return min(max(n, 8), maxSegs)
}

// rad is one ring of a radial shape: its radius and coverage.
type rad struct{ r, a float32 }

// radial adds rings rs[:nr] (innermost first) around (cx, cy), joined by
// quads, with a centre vertex of coverage ca fanned to the first ring. The
// polygon's vertices sit a little outside each radius, so each ring's
// polygon has its circle's area.
func (m *Mesh) radial(cx, cy float32, rs [4]rad, nr int, ca float32, c Color) {
	n := segments(rs[nr-1].r)
	step := 2 * math.Pi / float64(n)
	k := math.Sqrt(step / math.Sin(step)) // polygon area = circle area
	sn, cs := math.Sincos(step)
	centre := m.vert(cx, cy, c, ca)
	base := uint32(len(m.Verts))
	x, y := 1.0, 0.0
	for range n {
		for q := range nr {
			r := float64(rs[q].r) * k
			m.vert(cx+float32(x*r), cy+float32(y*r), c, rs[q].a)
		}
		x, y = x*cs-y*sn, x*sn+y*cs
	}
	at := func(j, q int) uint32 { return base + uint32((j%n)*nr+q) }
	for j := range n {
		if ca > 0 || rs[0].a > 0 {
			m.tri(centre, at(j, 0), at(j+1, 0))
		}
		for q := 0; q < nr-1; q++ {
			if rs[q].a == 0 && rs[q+1].a == 0 {
				continue
			}
			a, b, cc, d := at(j, q), at(j+1, q), at(j+1, q+1), at(j, q+1)
			m.tri(a, b, cc)
			m.tri(a, cc, d)
		}
	}
}

// Disc adds a filled circle of radius r. Its feather is shifted inward by
// about 1/(24r) px so the ramp's area equals the circle's; below r = 0.58
// px it is a 1-px cone whose volume is the disc's area.
func (m *Mesh) Disc(cx, cy, r float32, c Color) {
	if !(r > 0) {
		return
	}
	if r*r < 1.0/3 {
		m.radial(cx, cy, [4]rad{{1, 0}}, 1, 3*r*r, c)
		return
	}
	cr := float32(math.Sqrt(float64(r*r) - 1.0/12))
	if cr-0.5 < 1e-3 {
		m.radial(cx, cy, [4]rad{{cr + 0.5, 0}}, 1, 1, c)
		return
	}
	m.radial(cx, cy, [4]rad{{cr - 0.5, 1}, {cr + 0.5, 0}}, 2, 1, c)
}

// Ring adds a circle of radius r stroked width wide (centred on it). A
// stroke as wide as the circle fills it, like vector.StrokeCircle.
func (m *Mesh) Ring(cx, cy, r, width float32, c Color) {
	if !(width > 0) || !(r > 0) {
		return
	}
	if width >= 2*r {
		m.Disc(cx, cy, r+width/2, c)
		return
	}
	in, out, peak := band(width)
	rs := [4]rad{{r - out, 0}, {r - in, peak}, {r + in, peak}, {r + out, 0}}
	for q := range 2 { // a hole under a pixel: the profile's value at the centre
		if rs[q].r < 0 {
			rs[q] = rad{0, peak * min(1, out-r)}
		}
	}
	m.radial(cx, cy, rs, 4, rs[0].a, c)
}
