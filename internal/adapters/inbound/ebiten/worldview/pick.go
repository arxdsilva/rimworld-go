package worldview

import (
	"math"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/globemath"
)

// Markers and picking: every visible object gets a marker on the relief
// (globe.Surface), placed once per Update from the camera; a click takes
// the nearest marker within its radius.

// Marker sizes, physical pixels (the caravan badge's, AX-9).
const (
	markMin, markMax = 14, 40
	markSlack        = 4 // logical px past a marker's disc that still pick it
)

// mark is one object's marker this frame, in logical pixels.
type mark struct {
	obj     int
	x, y, r float32
}

// markSize is a marker's side in physical pixels at the globe's zoom.
func markSize(g *globe.View) float64 { return globe.Clamp(g.TilePx()*2, markMin, markMax) }

// placeMarks projects the objects onto the facing side of g (logical
// pixels). now places pods along their arc.
func placeMarks(dst []mark, objs []Object, g *globe.View, now uint64) []mark {
	dst = dst[:0]
	if g == nil {
		return dst
	}
	pj := newProjector(g)
	r := float32(markSize(g)*0.56) / pj.k
	for i := range objs {
		o := &objs[i]
		p := g.Surface(o.Tile)
		if o.Kind == KindPod {
			p = podPoint(g, o, now)
		}
		if x, y, ok := pj.at(p); ok {
			dst = append(dst, mark{obj: i, x: x, y: y, r: r})
		}
	}
	return dst
}

// projector maps points on the relief to logical screen pixels for one
// camera state (the globe's own transform is only set while it draws).
type projector struct {
	q        globemath.Quat
	vp       globemath.Viewport
	dist, zh float64
	k        float32 // physical pixels per logical one
}

func newProjector(g *globe.View) projector {
	return projector{q: g.Cam.Orientation(), vp: g.Viewport(), dist: g.Cam.Dist,
		zh: globemath.HorizonZ(g.Cam.Dist) + 0.04, k: float32(gfx.UIScale())}
}

// at is p on the screen (logical), and whether it faces the eye.
func (pj projector) at(p [3]float32) (x, y float32, ok bool) {
	v := pj.q.Rotate(globemath.Vec3{float64(p[0]), float64(p[1]), float64(p[2])})
	if v[2] < pj.zh {
		return 0, 0, false
	}
	sx, sy, _ := pj.vp.ProjectView(pj.dist, v)
	return float32(sx) / pj.k, float32(sy) / pj.k, true
}

// podPoint is where pod group o is along its arc at now (by the time
// left of its flight).
func podPoint(g *globe.View, o *Object, now uint64) [3]float32 {
	a, b := g.Surface(int(o.Pod.From)), g.Surface(int(o.Pod.To))
	t := 1.0
	if total := float64(max(1, o.flight)); o.Pod.Arrive > now {
		t = math.Max(0, 1-float64(o.Pod.Arrive-now)/total)
	}
	return slerpish(a, b, t)
}

// slerpish is the point t of the way from a to b, at the radius between
// theirs (a good enough great-circle step for a marker).
func slerpish(a, b [3]float32, t float64) [3]float32 {
	var o [3]float32
	l, ra, rb := 0.0, 0.0, 0.0
	for i := range o {
		v := float64(a[i]) + (float64(b[i])-float64(a[i]))*t
		o[i] = float32(v)
		l += v * v
		ra += float64(a[i]) * float64(a[i])
		rb += float64(b[i]) * float64(b[i])
	}
	if l = math.Sqrt(l); l > 0 {
		want := math.Sqrt(ra) + (math.Sqrt(rb)-math.Sqrt(ra))*t
		for i := range o {
			o[i] = float32(float64(o[i]) / l * want)
		}
	}
	return o
}

// nearest is the mark nearest (mx, my) within its radius (plus slack),
// -1 none.
func nearest(marks []mark, mx, my int) int {
	best, bd := -1, float32(math.MaxFloat32)
	for i := range marks {
		m := &marks[i]
		dx, dy := float32(mx)-m.x, float32(my)-m.y
		d := dx*dx + dy*dy
		if lim := m.r + markSlack; d <= lim*lim && d < bd {
			best, bd = i, d
		}
	}
	return best
}

// markRect is mark m's square hit area (logical).
func markRect(m mark) gfx.Rect {
	s := m.r + markSlack
	return gfx.Rect{X: m.x - s, Y: m.y - s, W: 2 * s, H: 2 * s}
}
