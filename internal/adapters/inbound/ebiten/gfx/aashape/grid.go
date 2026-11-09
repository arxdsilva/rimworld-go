package aashape

import "math"

// span is the box-filtered 1-D profile of a solid interval (see band),
// centred at c.
type span struct{ c, in, out, peak float32 }

func spanOf(a, b float32) span {
	in, out, peak := band(b - a)
	return span{(a + b) / 2, in, out, peak}
}

// at is the profile's coverage at x.
func (s span) at(x float32) float32 {
	d := x - s.c
	if d < 0 {
		d = -d
	}
	switch {
	case d <= s.in:
		return s.peak
	case d >= s.out:
		return 0
	}
	return s.peak * (s.out - d) / (s.out - s.in)
}

// box is a rectangle [u] x [v] in a frame's coordinates whose coverage is
// added (sign 1) or cut out of the shape (sign -1). The box filter is
// separable, so a box's coverage is exactly the product of its two spans.
type box struct {
	u, v span
	sign float32
}

// frame is an origin and two unit axes in physical pixels.
type frame struct{ ox, oy, ux, uy, vx, vy float32 }

// knots are the sorted breakpoints of up to two boxes along one axis.
type knots struct {
	k [8]float32
	n int
}

func (ks *knots) add(s span) {
	for _, x := range [4]float32{s.c - s.out, s.c - s.in, s.c + s.in, s.c + s.out} {
		i := ks.n
		for i > 0 && ks.k[i-1] > x {
			ks.k[i] = ks.k[i-1]
			i--
		}
		ks.k[i] = x
		ks.n++
	}
}

// grid adds bs[:nb] on a tensor grid through every box's breakpoints:
// inside a cell each box's coverage is bilinear, so a cell whose corners
// fit a plane is two triangles and any other (a corner of a box) is a fan
// of four round the bilinear value at its centre.
func (m *Mesh) grid(f frame, bs [2]box, nb int, c Color) {
	var us, vs knots
	for i := range nb {
		us.add(bs[i].u)
		vs.add(bs[i].v)
	}
	cov := func(u, v float32) float32 {
		var a float32
		for i := range nb {
			a += bs[i].sign * bs[i].u.at(u) * bs[i].v.at(v)
		}
		return max(a, 0)
	}
	pos := func(u, v float32) (float32, float32) {
		return f.ox + u*f.ux + v*f.vx, f.oy + u*f.uy + v*f.vy
	}
	base := uint32(len(m.Verts))
	for j := range vs.n {
		for i := range us.n {
			x, y := pos(us.k[i], vs.k[j])
			m.vert(x, y, c, cov(us.k[i], vs.k[j]))
		}
	}
	nx := uint32(us.n)
	for j := 0; j < vs.n-1; j++ {
		if vs.k[j+1]-vs.k[j] < 1e-6 {
			continue
		}
		for i := 0; i < us.n-1; i++ {
			if us.k[i+1]-us.k[i] < 1e-6 {
				continue
			}
			i00 := base + uint32(j)*nx + uint32(i)
			i10, i01, i11 := i00+1, i00+nx, i00+nx+1
			a00, a10 := m.Verts[i00].ColorA, m.Verts[i10].ColorA
			a01, a11 := m.Verts[i01].ColorA, m.Verts[i11].ColorA
			if a00 == 0 && a10 == 0 && a01 == 0 && a11 == 0 {
				continue
			}
			if d := a00 + a11 - a10 - a01; d < 1e-6 && d > -1e-6 {
				m.tri(i00, i10, i11)
				m.tri(i00, i11, i01)
				continue
			}
			uc, vc := (us.k[i]+us.k[i+1])/2, (vs.k[j]+vs.k[j+1])/2
			x, y := pos(uc, vc)
			ic := m.vert(x, y, c, cov(uc, vc))
			m.tri(ic, i00, i10)
			m.tri(ic, i10, i11)
			m.tri(ic, i11, i01)
			m.tri(ic, i01, i00)
		}
	}
}

// Line adds a segment from (x0, y0) to (x1, y1), width wide, with butt
// caps. A zero-length segment adds nothing.
func (m *Mesh) Line(x0, y0, x1, y1, width float32, c Color) {
	dx, dy := x1-x0, y1-y0
	l := float32(math.Hypot(float64(dx), float64(dy)))
	if !(l > 1e-6) || !(width > 0) {
		return
	}
	ux, uy := dx/l, dy/l
	m.grid(frame{x0, y0, ux, uy, -uy, ux}, [2]box{{spanOf(0, l), spanOf(-width/2, width/2), 1}}, 1, c)
}

// Rect adds the filled rectangle (x, y, w, h).
func (m *Mesh) Rect(x, y, w, h float32, c Color) {
	if !(w > 0) || !(h > 0) {
		return
	}
	m.grid(frame{0, 0, 1, 0, 0, 1}, [2]box{{spanOf(x, x+w), spanOf(y, y+h), 1}}, 1, c)
}

// Frame adds the rectangle (x, y, w, h) stroked width wide, centred on its
// edges with square corners, like vector.StrokeRect: the outer box less
// the inner one, which is exact under the box filter. A stroke as wide as
// the box fills it.
func (m *Mesh) Frame(x, y, w, h, width float32, c Color) {
	if !(width > 0) || w < 0 || h < 0 {
		return
	}
	hw := width / 2
	bs := [2]box{{spanOf(x-hw, x+w+hw), spanOf(y-hw, y+h+hw), 1}}
	nb := 1
	if width < w && width < h {
		bs[1] = box{spanOf(x+hw, x+w-hw), spanOf(y+hw, y+h-hw), -1}
		nb = 2
	}
	m.grid(frame{0, 0, 1, 0, 0, 1}, bs, nb, c)
}
