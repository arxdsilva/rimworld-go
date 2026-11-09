package aashape

import (
	"fmt"
	"math"
	"testing"
)

// The coverage tests rasterise a shape's triangles on the CPU and compare
// them with the exact shape. Two measures, because a GPU samples each
// triangle at the pixel centre:
//   - per pixel, the composited alpha at the centre against the exact
//     fraction of the pixel the shape covers (16x16 supersampling of the
//     analytic shape near its edge): |Δ| <= 0.1;
//   - the total, the alpha field integrated by 16x16 supersampling against
//     the shape's exact area: within 0.5%.
// Supersampling the triangles per pixel would box-filter the feather a
// second time (the feather already is the box filter), so the per-pixel
// check samples the centre as the GPU does.

const ss = 16 // supersamples per pixel side

var white = Color{1, 1, 1, 1}

// shape is an analytic shape: inside reports a point in it, sd a lower bound
// of its signed distance (negative inside) that is exact enough to tell
// pixels wholly in or out, area its exact area.
type shape struct {
	name   string
	inside func(x, y float64) bool
	sd     func(x, y float64) float64
	area   float64
	x0, y0 int // the raster's origin and size (pixels)
	w, h   int
}

// raster is the mesh's alpha composited (source over) at sample points.
type raster struct {
	x0, y0 float64
	n, w   int // samples per pixel side, raster width in samples
	a      []float64
}

func rasterise(m *Mesh, x0, y0, w, h, n int) *raster {
	r := &raster{x0: float64(x0), y0: float64(y0), n: n, w: w * n, a: make([]float64, w*n*h*n)}
	keep := make([]float64, len(r.a)) // 1 - alpha so far
	for i := range keep {
		keep[i] = 1
	}
	step := 1 / float64(n)
	// a tiny irrational offset keeps samples off shared edges, which the
	// GPU's fill rule gives to one triangle
	ox, oy := step/2+1e-5*math.Sqrt2, step/2+1e-5*math.Sqrt(3)
	for t := 0; t+2 < len(m.Idx); t += 3 {
		v0, v1, v2 := m.Verts[m.Idx[t]], m.Verts[m.Idx[t+1]], m.Verts[m.Idx[t+2]]
		ax, ay, bx, by, cx, cy := float64(v0.DstX), float64(v0.DstY), float64(v1.DstX), float64(v1.DstY), float64(v2.DstX), float64(v2.DstY)
		d := (bx-ax)*(cy-ay) - (cx-ax)*(by-ay)
		if d == 0 {
			continue
		}
		minX, maxX := min(ax, bx, cx), max(ax, bx, cx)
		minY, maxY := min(ay, by, cy), max(ay, by, cy)
		i0 := max(0, int(math.Floor((minX-r.x0-ox)*float64(n))))
		i1 := min(r.w-1, int(math.Ceil((maxX-r.x0-ox)*float64(n))))
		j0 := max(0, int(math.Floor((minY-r.y0-oy)*float64(n))))
		j1 := min(h*n-1, int(math.Ceil((maxY-r.y0-oy)*float64(n))))
		for j := j0; j <= j1; j++ {
			py := r.y0 + float64(j)*step + oy
			for i := i0; i <= i1; i++ {
				px := r.x0 + float64(i)*step + ox
				l1 := ((px-ax)*(cy-ay) - (cx-ax)*(py-ay)) / d
				l2 := ((bx-ax)*(py-ay) - (px-ax)*(by-ay)) / d
				l0 := 1 - l1 - l2
				if l0 < 0 || l1 < 0 || l2 < 0 {
					continue
				}
				a := l0*float64(v0.ColorA) + l1*float64(v1.ColorA) + l2*float64(v2.ColorA)
				keep[j*r.w+i] *= 1 - a
			}
		}
	}
	for i := range keep {
		r.a[i] = 1 - keep[i]
	}
	return r
}

// coverage is the exact fraction of pixel (px, py) inside s.
func (s shape) coverage(px, py int) float64 {
	cx, cy := float64(px)+0.5, float64(py)+0.5
	switch d := s.sd(cx, cy); {
	case d > 0.75:
		return 0
	case d < -0.75:
		return 1
	}
	in := 0
	for j := range ss {
		for i := range ss {
			if s.inside(float64(px)+(float64(i)+0.5)/ss, float64(py)+(float64(j)+0.5)/ss) {
				in++
			}
		}
	}
	return float64(in) / (ss * ss)
}

// check compares m with s; it returns the worst pixel and the area error.
func check(t *testing.T, s shape, m *Mesh) {
	t.Helper()
	centre := rasterise(m, s.x0, s.y0, s.w, s.h, 1)
	worst, wx, wy, wgot, wwant := 0.0, 0, 0, 0.0, 0.0
	for j := range s.h {
		for i := range s.w {
			got := centre.a[j*s.w+i]
			want := s.coverage(s.x0+i, s.y0+j)
			if d := math.Abs(got - want); d > worst {
				worst, wx, wy, wgot, wwant = d, s.x0+i, s.y0+j, got, want
			}
		}
	}
	if worst > 0.1 {
		t.Errorf("%s: pixel (%d,%d) = %.3f, exact coverage %.3f (|Δ| %.3f > 0.1)", s.name, wx, wy, wgot, wwant, worst)
	}
	fine := rasterise(m, s.x0, s.y0, s.w, s.h, ss)
	var sum float64
	for _, a := range fine.a {
		sum += a
	}
	sum /= ss * ss
	if e := math.Abs(sum-s.area) / s.area; e > 0.005 {
		t.Errorf("%s: area %.4f, exact %.4f (%.2f%% > 0.5%%)", s.name, sum, s.area, 100*e)
	}
}

// around is the raster round the box (x0, y0)-(x1, y1) plus a 2-px margin.
func around(s shape, x0, y0, x1, y1 float64) shape {
	s.x0, s.y0 = int(math.Floor(x0))-2, int(math.Floor(y0))-2
	s.w, s.h = int(math.Ceil(x1))+2-s.x0, int(math.Ceil(y1))+2-s.y0
	return s
}

var offsets = [][2]float64{{0, 0}, {0.25, 0.5}, {0.37, 0.81}, {0.5, 0.5}}

func TestDiscCoverage(t *testing.T) {
	for _, r := range []float64{0.3, 0.5, 0.8, 1, 1.5, 3, 7.3, 20, 48} {
		for _, o := range offsets {
			cx, cy := 40+o[0], 40+o[1]
			s := around(shape{
				name:   fmt.Sprintf("disc r=%g at +%v", r, o),
				inside: func(x, y float64) bool { return math.Hypot(x-cx, y-cy) < r },
				sd:     func(x, y float64) float64 { return math.Hypot(x-cx, y-cy) - r },
				area:   math.Pi * r * r,
			}, cx-r, cy-r, cx+r, cy+r)
			var m Mesh
			m.Disc(float32(cx), float32(cy), float32(r), white)
			check(t, s, &m)
		}
	}
}

func TestRingCoverage(t *testing.T) {
	for _, r := range []float64{3, 8, 25, 48} {
		for _, w := range []float64{0.5, 1, 1.5, 2, 4} {
			for _, o := range offsets {
				cx, cy := 60+o[0], 60+o[1]
				s := around(shape{
					name:   fmt.Sprintf("ring r=%g w=%g at +%v", r, w, o),
					inside: func(x, y float64) bool { return math.Abs(math.Hypot(x-cx, y-cy)-r) < w/2 },
					sd:     func(x, y float64) float64 { return math.Abs(math.Hypot(x-cx, y-cy)-r) - w/2 },
					area:   2 * math.Pi * r * w,
				}, cx-r-w, cy-r-w, cx+r+w, cy+r+w)
				var m Mesh
				m.Ring(float32(cx), float32(cy), float32(r), float32(w), white)
				check(t, s, &m)
			}
		}
	}
}

// boxSD is a lower bound of the distance from (u, v) to the box
// [0, l] x [-w/2, w/2] (the larger axis distance).
func boxSD(u, v, l, w float64) float64 { return max(math.Abs(u-l/2)-l/2, math.Abs(v)-w/2) }

func TestLineCoverage(t *testing.T) {
	for _, w := range []float64{0.5, 1, 2, 4} {
		for _, l := range []float64{0.7, 3, 20, 57} {
			for _, deg := range []float64{0, 13, 30, 45, 77, 90, 200} {
				for _, o := range offsets {
					x0, y0 := 70+o[0], 70+o[1]
					ux, uy := math.Cos(deg*math.Pi/180), math.Sin(deg*math.Pi/180)
					x1, y1 := x0+l*ux, y0+l*uy
					local := func(x, y float64) (float64, float64) {
						return (x-x0)*ux + (y-y0)*uy, -(x-x0)*uy + (y-y0)*ux
					}
					s := around(shape{
						name:   fmt.Sprintf("line w=%g l=%g %g° at +%v", w, l, deg, o),
						inside: func(x, y float64) bool { u, v := local(x, y); return boxSD(u, v, l, w) < 0 },
						sd:     func(x, y float64) float64 { u, v := local(x, y); return boxSD(u, v, l, w) },
						area:   l * w,
					}, min(x0, x1)-w, min(y0, y1)-w, max(x0, x1)+w, max(y0, y1)+w)
					var m Mesh
					m.Line(float32(x0), float32(y0), float32(x1), float32(y1), float32(w), white)
					check(t, s, &m)
				}
			}
		}
	}
}

func TestRectAndFrameCoverage(t *testing.T) {
	for _, sz := range [][2]float64{{0.6, 3}, {5, 2.5}, {17.3, 9.8}} {
		for _, o := range offsets {
			x, y, w, h := 20+o[0], 20+o[1], sz[0], sz[1]
			rect := func(px, py float64) float64 { return max(math.Abs(px-x-w/2)-w/2, math.Abs(py-y-h/2)-h/2) }
			s := around(shape{
				name:   fmt.Sprintf("rect %gx%g at +%v", w, h, o),
				inside: func(px, py float64) bool { return rect(px, py) < 0 },
				sd:     rect,
				area:   w * h,
			}, x, y, x+w, y+h)
			var m Mesh
			m.Rect(float32(x), float32(y), float32(w), float32(h), white)
			check(t, s, &m)
			for _, sw := range []float64{0.5, 1, 2} {
				if sw >= w || sw >= h {
					continue
				}
				frame := func(px, py float64) float64 { return math.Abs(rect(px, py)) - sw/2 }
				f := around(shape{
					name:   fmt.Sprintf("frame %gx%g w=%g at +%v", w, h, sw, o),
					inside: func(px, py float64) bool { return frame(px, py) < 0 },
					sd:     frame,
					area:   (w+sw)*(h+sw) - (w-sw)*(h-sw),
				}, x-sw, y-sw, x+w+sw, y+h+sw)
				var fm Mesh
				fm.Frame(float32(x), float32(y), float32(w), float32(h), float32(sw), white)
				check(t, f, &fm)
			}
		}
	}
}

// polyShape is the analytic convex polygon through pts (either winding).
func polyShape(name string, pts [][2]float64) shape {
	n := len(pts)
	var area float64
	for i := range n {
		a, b := pts[i], pts[(i+1)%n]
		area += a[0]*b[1] - b[0]*a[1]
	}
	sgn := 1.0
	if area < 0 {
		sgn = -1
	}
	sd := func(x, y float64) float64 {
		d := math.Inf(-1)
		for i := range n {
			a, b := pts[i], pts[(i+1)%n]
			ex, ey := b[0]-a[0], b[1]-a[1]
			l := math.Hypot(ex, ey)
			d = max(d, sgn*((x-a[0])*ey-(y-a[1])*ex)/l)
		}
		return d
	}
	x0, y0, x1, y1 := pts[0][0], pts[0][1], pts[0][0], pts[0][1]
	for _, p := range pts {
		x0, y0, x1, y1 = min(x0, p[0]), min(y0, p[1]), max(x1, p[0]), max(y1, p[1])
	}
	return around(shape{name: name, inside: func(x, y float64) bool { return sd(x, y) < 0 }, sd: sd, area: math.Abs(area) / 2}, x0, y0, x1, y1)
}

func TestConvexPolyCoverage(t *testing.T) {
	reg := func(n int, r, rot float64, o [2]float64) [][2]float64 {
		var pts [][2]float64
		for i := range n {
			a := rot + 2*math.Pi*float64(i)/float64(n)
			pts = append(pts, [2]float64{30 + o[0] + r*math.Cos(a), 30 + o[1] + r*math.Sin(a)})
		}
		return pts
	}
	for _, o := range offsets {
		for _, c := range []struct {
			name string
			pts  [][2]float64
		}{
			{"square", reg(4, 8, 0.3, o)},
			{"axis square", reg(4, 7.07, math.Pi/4, o)},
			{"hexagon", reg(6, 12, 0.1, o)},
			{"triangle", reg(3, 10, -math.Pi/2, o)},
			{"small triangle", reg(3, 4, 0.4, o)},
			{"octagon cw", reverse(reg(8, 20, 0, o))},
			{"quad", [][2]float64{{20 + o[0], 20 + o[1]}, {35 + o[0], 22 + o[1]}, {33 + o[0], 40 + o[1]}, {18 + o[0], 31 + o[1]}}},
		} {
			s := polyShape(fmt.Sprintf("%s at +%v", c.name, o), c.pts)
			pts := make([][2]float32, len(c.pts))
			for i, p := range c.pts {
				pts[i] = [2]float32{float32(p[0]), float32(p[1])}
			}
			var m Mesh
			if !m.ConvexPoly(pts, 1, white) {
				t.Fatalf("%s: refused", s.name)
			}
			check(t, s, &m)
		}
	}
}

func reverse(p [][2]float64) [][2]float64 {
	for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
		p[i], p[j] = p[j], p[i]
	}
	return p
}

func TestConvexPolyRefuses(t *testing.T) {
	var m Mesh
	for name, pts := range map[string][][2]float32{
		"star":      {{0, -10}, {3, -3}, {10, 0}, {3, 3}, {0, 10}, {-3, 3}, {-10, 0}, {-3, -3}},
		"two":       {{0, 0}, {5, 5}},
		"repeated":  {{0, 0}, {5, 0}, {5, 0}, {0, 5}},
		"sub-pixel": {{0, 0}, {0.6, 0}, {0.3, 0.5}},
		"collapsed": {{0, 0}, {10, 0}, {20, 0}},
	} {
		if m.ConvexPoly(pts, 1, white) || len(m.Verts) != 0 || len(m.Idx) != 0 {
			t.Errorf("%s: accepted (%d verts)", name, len(m.Verts))
		}
		m.Reset()
	}
}

// A colour reaches every vertex premultiplied by its coverage.
func TestPremultipliedColour(t *testing.T) {
	var m Mesh
	c := Color{0.5, 0.25, 0, 0.5}
	m.Disc(10, 10, 5, c)
	for _, v := range m.Verts {
		a := v.ColorA / c.A
		if math.Abs(float64(v.ColorR-c.R*a)) > 1e-6 || math.Abs(float64(v.ColorG-c.G*a)) > 1e-6 || v.ColorB != 0 || v.SrcX != Texel || v.SrcY != Texel {
			t.Fatalf("vertex %+v is not %v x coverage %.3f on the centre texel", v, c, a)
		}
	}
}
