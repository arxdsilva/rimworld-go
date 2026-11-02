package globe

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// depthBuckets is the resolution of the per-frame far-to-near tile order.
const depthBuckets = 256

// xform is one frame's camera: the world-to-view rotation, the horizon and
// the projection, plus the sun and the eye turned into planet space.
type xform struct {
	mat    [9]float32
	CamDir [3]float32
	sun    [3]float32 // the sun direction in planet space
	eye    [3]float32 // the eye in planet space
	Zh     float32
	limbR  float32
	cx, cy float32
	focal  float32
	dist   float32
}

// view rotates a world vector into view space.
func (x *xform) view(p *[3]float32) (vx, vy, vz float32) {
	m := &x.mat
	return m[0]*p[0] + m[1]*p[1] + m[2]*p[2],
		m[3]*p[0] + m[4]*p[1] + m[5]*p[2],
		m[6]*p[0] + m[7]*p[1] + m[8]*p[2]
}

// project maps a view-space point to the screen; points past the horizon
// are pushed onto the limb so a tile that straddles it keeps a sane shape.
func (x *xform) project(vx, vy, vz float32) (sx, sy float32) {
	if vz < x.Zh {
		r := float32(math.Hypot(float64(vx), float64(vy)))
		if r < 1e-6 {
			r = 1e-6
		}
		s := x.limbR / r
		vx, vy, vz = vx*s, vy*s, x.Zh
	}
	k := x.focal / (x.dist - vz)
	return x.cx + vx*k, x.cy - vy*k
}

// WorldPt projects a world point: screen position and view-space z.
func (x *xform) WorldPt(p *[3]float32) (sx, sy, vz float32) {
	vx, vy, vz := x.view(p)
	sx, sy = x.project(vx, vy, vz)
	return sx, sy, vz
}

// rimOf is the atmosphere share for a unit view-space z (grows toward the limb).
func rimOf(vz float32) float32 {
	t := 1 - vz
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return t * t * t * 0.45
}

// cornerCache holds this frame's screen position and rim of each unique
// corner, and the scratch of the far-to-near tile order.
type cornerCache struct {
	bf    uint32
	stamp []uint32
	sx    []float32
	sy    []float32
	rim   []float32
	list  []int32 // facing tiles, in index order
	key   []uint8 // their depth bucket
	order []int32 // facing tiles, far to near
}

func newCornerCache(corners, tiles int) cornerCache {
	return cornerCache{stamp: make([]uint32, corners), sx: make([]float32, corners), sy: make([]float32, corners),
		rim: make([]float32, corners), list: make([]int32, tiles), key: make([]uint8, tiles), order: make([]int32, tiles)}
}

// at makes sure unique corner id has this frame's screen position and rim.
func (c *cornerCache) at(id uint32, x *xform, geo *planetGeom) (sx, sy, rim float32) {
	if c.stamp[id] != c.bf {
		vx, vy, vz := x.view(&geo.cornerDisp[id])
		c.sx[id], c.sy[id] = x.project(vx, vy, vz)
		c.rim[id] = rimOf(vz / geo.cornerR[id])
		c.stamp[id] = c.bf
	}
	return c.sx[id], c.sy[id], c.rim[id]
}

// sortFarToNear orders the first n listed tiles by depth bucket, farthest first.
func (c *cornerCache) sortFarToNear(n int) []int32 {
	var cnt [depthBuckets + 1]int32
	for _, k := range c.key[:n] {
		cnt[int(k)+1]++
	}
	for b := 1; b <= depthBuckets; b++ {
		cnt[b] += cnt[b-1]
	}
	for j := 0; j < n; j++ {
		k := c.key[j]
		c.order[cnt[k]] = c.list[j]
		cnt[k]++
	}
	return c.order[:n]
}

// ovMesh is the overlay batch: lines and fills drawn over the tiles.
type ovMesh struct {
	Ov    []ebiten.Vertex
	Oi    []uint32
	ovMax int
}

// ovColour is a straight-alpha colour for the overlay mesh.
type ovColour struct{ r, g, b, a float32 }

func (m *ovMesh) vert(x, y float32, c ovColour) {
	m.Ov = append(m.Ov, ebiten.Vertex{DstX: x, DstY: y, SrcX: 1.5, SrcY: 1.5,
		ColorR: c.r, ColorG: c.g, ColorB: c.b, ColorA: c.a})
}

// line draws a world-space line piece when both ends face the eye.
func (m *ovMesh) line(x *xform, a, b *[3]float32, w float32, c ovColour) {
	ax, ay, az := x.WorldPt(a)
	bx, by, bz := x.WorldPt(b)
	lim := x.Zh + 0.01
	if az < lim || bz < lim {
		return
	}
	m.quad(ax, ay, bx, by, w, c)
}

// dashes draws the given pieces (fractions along s) of a segment.
func (m *ovMesh) dashes(x *xform, s *globeSeg, ds [][2]float32, w float32, c ovColour) {
	for _, d := range ds {
		a, b := globeLerp3(&s.a, &s.b, d[0]), globeLerp3(&s.a, &s.b, d[1])
		m.line(x, &a, &b, w, c)
	}
}

// taper adds a line from (ax, ay) wa wide to (bx, by) wb wide, its ends
// extended like quad's.
func (m *ovMesh) taper(ax, ay, bx, by, wa, wb float32, c ovColour) {
	if len(m.Ov)+4 > m.ovMax {
		return
	}
	dx, dy := bx-ax, by-ay
	l := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if l < 0.05 {
		return
	}
	dx, dy = dx/l, dy/l
	ax, ay, bx, by = ax-dx*wa/3, ay-dy*wa/3, bx+dx*wb/3, by+dy*wb/3
	b := uint32(len(m.Ov))
	m.vert(ax-dy*wa/2, ay+dx*wa/2, c)
	m.vert(ax+dy*wa/2, ay-dx*wa/2, c)
	m.vert(bx+dy*wb/2, by-dx*wb/2, c)
	m.vert(bx-dy*wb/2, by+dx*wb/2, c)
	m.Oi = append(m.Oi, b, b+1, b+2, b, b+2, b+3)
}

// quad adds a w-wide line from (ax, ay) to (bx, by), its ends extended
// by a third of the width so consecutive pieces overlap at bends.
func (m *ovMesh) quad(ax, ay, bx, by, w float32, c ovColour) {
	if len(m.Ov)+4 > m.ovMax {
		return
	}
	dx, dy := bx-ax, by-ay
	l := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if l < 0.05 {
		return
	}
	dx, dy = dx/l, dy/l
	e := w / 3
	ax, ay, bx, by = ax-dx*e, ay-dy*e, bx+dx*e, by+dy*e
	nx, ny := -dy*w/2, dx*w/2
	b := uint32(len(m.Ov))
	m.vert(ax+nx, ay+ny, c)
	m.vert(ax-nx, ay-ny, c)
	m.vert(bx-nx, by-ny, c)
	m.vert(bx+nx, by+ny, c)
	m.Oi = append(m.Oi, b, b+1, b+2, b, b+2, b+3)
}
