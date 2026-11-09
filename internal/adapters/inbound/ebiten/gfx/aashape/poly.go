package aashape

import "math"

// ConvexPoly adds the filled convex polygon through pts, each scaled by k,
// in either winding. A pixel's coverage is modelled as the product of its
// edges' 1-px ramps: the polygon inset by 0.5 px is solid, each edge has a
// ramp band, and each corner is the cell where its two edges' ramps meet,
// a fan of four round the corner itself (coverage: its angle over 2π, so
// 1/4 for a right angle, as the product gives), so a sharp tip
// fades instead of growing a mitre. It reports false, adding nothing, for a
// polygon that is not convex, has a repeated or collinear point or is under
// about 1 px across (the caller draws those another way).
func (m *Mesh) ConvexPoly(pts [][2]float32, k float32, c Color) bool {
	n := len(pts)
	if n < 3 {
		return false
	}
	p := func(i int) (float32, float32) { q := pts[(i+n)%n]; return q[0] * k, q[1] * k }
	var area float32
	for i := range n {
		x0, y0 := p(i)
		x1, y1 := p(i + 1)
		area += x0*y1 - x1*y0
	}
	if area > -1e-3 && area < 1e-3 {
		return false
	}
	sgn := float32(1)
	if area < 0 {
		sgn = -1
	}
	// outward unit normal of edge i (p(i) -> p(i+1))
	normal := func(i int) (float32, float32, bool) {
		x0, y0 := p(i)
		x1, y1 := p(i + 1)
		ex, ey := x1-x0, y1-y0
		l := float32(math.Hypot(float64(ex), float64(ey)))
		if l < 1e-4 {
			return 0, 0, false
		}
		return sgn * ey / l, -sgn * ex / l, true
	}
	base := uint32(len(m.Verts))
	refuse := func() bool { m.Verts = m.Verts[:base]; return false }
	// Corner i joins edge i-1 (normal a) and edge i (normal b). Its cell's
	// points sit where the two signed distances are ±0.5: q (-,-) inside,
	// e (+,-) on edge i-1's outer line, f (-,+) on edge i's, g (+,+).
	for i := range n {
		ax, ay, ok1 := normal(i - 1)
		bx, by, ok2 := normal(i)
		det := ax*by - ay*bx
		if !ok1 || !ok2 || sgn*det < 1e-3 { // a reflex, straight or repeated corner
			return refuse()
		}
		x, y := p(i)
		at := func(da, db float32) (float32, float32) {
			return x + (da*by-db*ay)/det, y + (db*ax-da*bx)/det
		}
		qx, qy := at(-0.5, -0.5)
		m.vert(qx, qy, c, 1)
		ex, ey := at(0.5, -0.5)
		m.vert(ex, ey, c, 0)
		fx, fy := at(-0.5, 0.5)
		m.vert(fx, fy, c, 0)
		gx, gy := at(0.5, 0.5)
		m.vert(gx, gy, c, 0)
		turn := math.Acos(float64(min(1, max(-1, ax*bx+ay*by))))
		m.vert(x, y, c, float32((math.Pi-turn)/(2*math.Pi)))
	}
	q := func(i int) uint32 { return base + uint32(5*((i+n)%n)) }
	for i := range n { // the inset keeps every edge's direction
		x0, y0 := p(i)
		x1, y1 := p(i + 1)
		q0, q1 := m.Verts[q(i)], m.Verts[q(i+1)]
		if (q1.DstX-q0.DstX)*(x1-x0)+(q1.DstY-q0.DstY)*(y1-y0) <= 0 {
			return refuse()
		}
	}
	for i := 1; i < n-1; i++ {
		m.tri(q(0), q(i), q(i+1))
	}
	for i := range n {
		e, f, g, pc := q(i)+1, q(i)+2, q(i)+3, q(i)+4
		m.tri(q(i), q(i+1), q(i+1)+1) // edge i's band: q(i), q(i+1), e(i+1), f(i)
		m.tri(q(i), q(i+1)+1, f)
		m.tri(pc, q(i), e)
		m.tri(pc, e, g)
		m.tri(pc, g, f)
		m.tri(pc, f, q(i))
	}
	return true
}
