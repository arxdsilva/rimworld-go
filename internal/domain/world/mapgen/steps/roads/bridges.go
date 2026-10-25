package roads

import (
	"math"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/terrain"
)

// Crossings (MAPGEN-3, .cursor/PLAY014_PLAN.md §3.3): where a road crosses
// river or lake water it crosses on an old wooden bridge, a straight strip
// two cells wide from the last dry bank cell to the first, so it reads as
// road, planks, road. The strip runs along the map axis nearest the
// river's local normal (else the bank-to-bank chord); water, marsh and mud
// under it become bridge cells (terrain unchanged), dry cells Track.
const (
	mergeGap = 2 // wet runs this few dry path cells apart are one crossing
	linkBack = 6 // path cells searched on either bank for a straight link to the strip
	spanSlop = 6 // a strip may be this much longer than 3x its wet run
)

// strip is a straight 2-wide crossing: lanes p0 and p0+1 across the axis,
// from bank position s0 (the road's way in) to s1 along it.
type strip struct {
	vertical bool // runs along y (lanes are columns)
	p0       int
	s0, s1   int
}

// cell is the strip's cell at along-axis position s in lane p.
func (st strip) cell(w, s, p int) int {
	if st.vertical {
		return s*w + p
	}
	return p*w + s
}

// end is the centreline point between the lanes at position s.
func (st strip) end(s int) mapgen.PointF {
	if st.vertical {
		return mapgen.PointF{X: float64(st.p0) + 1, Y: float64(s) + 0.5}
	}
	return mapgen.PointF{X: float64(s) + 0.5, Y: float64(st.p0) + 1}
}

// piece is a stretch of a laid road: land cells to smooth, a straight
// strip, or a raw crossing (path cells kept as routed).
type piece struct {
	kind  uint8 // pieceLand, pieceStrip, pieceRaw
	cells []int32
	st    strip
}

const (
	pieceLand uint8 = iota
	pieceStrip
	pieceRaw
)

// lay lays path down (see Lay) and returns its centreline.
func (rt *router) lay(path []int32) []mapgen.PointF {
	clear(rt.cut)
	pieces := rt.plan(path)
	lines := make([][]mapgen.PointF, len(pieces))
	for k, p := range pieces {
		switch p.kind {
		case pieceLand:
			lines[k] = rt.smooth(p.cells)
		case pieceStrip:
			lines[k] = []mapgen.PointF{p.st.end(p.st.s0), p.st.end(p.st.s1)}
		default:
			for _, i := range p.cells {
				if rt.isRock(int(i)) {
					rt.cut[i] = true
				}
				lines[k] = append(lines[k], mapgen.PointF{X: float64(int(i)%rt.w) + 0.5, Y: float64(int(i)/rt.w) + 0.5})
			}
		}
	}
	// land meets a strip at its centreline end, between the lanes
	for k, p := range pieces {
		if p.kind != pieceLand {
			continue
		}
		l := lines[k]
		in := k > 0 && pieces[k-1].kind == pieceStrip
		out := k+1 < len(pieces) && pieces[k+1].kind == pieceStrip
		switch {
		case in && out && len(l) == 1:
			l = []mapgen.PointF{lines[k-1][1], lines[k+1][0]}
		default:
			if in {
				if len(l) == 1 {
					l = append([]mapgen.PointF{lines[k-1][1]}, l...)
				} else {
					l[0] = lines[k-1][1]
				}
			}
			if out {
				if len(l) == 1 {
					l = append(l, lines[k+1][0])
				} else {
					l[len(l)-1] = lines[k+1][0]
				}
			}
		}
		lines[k] = l
	}
	changed := make(map[int]bool) // lookup only, never iterated
	var land, raw [][]mapgen.PointF
	for k, p := range pieces {
		switch p.kind {
		case pieceStrip:
			rt.layStrip(p.st, changed)
		case pieceRaw:
			raw = append(raw, lines[k])
		default:
			land = append(land, lines[k])
		}
	}
	if len(raw) > 0 {
		rt.rasterise(raw, true, changed)
	}
	rt.rasterise(land, false, changed)
	rt.dropChunks(changed)
	var out []mapgen.PointF
	for _, l := range lines {
		for _, q := range l {
			if m := len(out); m > 0 && out[m-1] == q {
				continue
			}
			out = append(out, q)
		}
	}
	return out
}

// plan splits path into land pieces and crossings.
func (rt *router) plan(path []int32) []piece {
	var out []piece
	var land []int32
	lo := 0
	for _, r := range rt.runs(path) {
		if st, ka, kb, linkA, linkB, ok := rt.fitStrip(path, r, lo); ok {
			land = append(append(land, path[lo:ka+1]...), linkA...)
			out = append(out, piece{kind: pieceLand, cells: land}, piece{kind: pieceStrip, st: st})
			land = linkB
			lo = kb
			continue
		}
		a, b := max(lo, r[0]-1), min(len(path)-1, r[1]+1)
		land = append(land, path[lo:a+1]...)
		out = append(out, piece{kind: pieceLand, cells: land}, piece{kind: pieceRaw, cells: path[a : b+1]})
		land = nil
		lo = b
	}
	land = append(land, path[lo:]...)
	return append(out, piece{kind: pieceLand, cells: land})
}

// runs are the path's stretches over unbridged water, [first, last]
// index; stretches mergeGap or fewer dry cells apart are joined.
func (rt *router) runs(path []int32) [][2]int {
	var out [][2]int
	for k, i := range path {
		if !rt.wet[i] {
			continue
		}
		if m := len(out); m > 0 && k-out[m-1][1]-1 <= mergeGap {
			out[m-1][1] = k
		} else {
			out = append(out, [2]int{k, k})
		}
	}
	return out
}

// fitStrip finds a straight strip for wet run r of path, trying the axis
// nearest the river's normal, then the chord's, then the other, each with
// the lanes shifted up to 2 cells. It also returns the path indices ka
// (≥ lo) and kb where the road leaves and rejoins its route, and the
// straight link cells from path[ka] to the strip (excluding path[ka]) and
// from the strip to path[kb] (excluding path[kb]).
func (rt *router) fitStrip(path []int32, r [2]int, lo int) (st strip, ka, kb int, linkA, linkB []int32, ok bool) {
	a, b := r[0]-1, r[1]+1
	if a < lo || b >= len(path) {
		return st, 0, 0, nil, nil, false
	}
	var mx, my float64
	river := false
	for _, i := range path[r[0] : r[1]+1] {
		mx += float64(int(i) % rt.w)
		my += float64(int(i) / rt.w)
		river = river || rt.c.F.River[i]
	}
	cnt := float64(r[1] - r[0] + 1)
	mx, my = mx/cnt, my/cnt
	ax, ay := int(path[a])%rt.w, int(path[a])/rt.w
	bx, by := int(path[b])%rt.w, int(path[b])/rt.w
	chord := abs(by-ay) >= abs(bx-ax)
	axes := make([]bool, 0, 3)
	if n, found := rt.riverNormal(mx+0.5, my+0.5); river && found {
		axes = append(axes, math.Abs(n.Y) > math.Abs(n.X))
	}
	for _, v := range [2]bool{chord, !chord} {
		if !containsBool(axes, v) {
			axes = append(axes, v)
		}
	}
	maxSpan := 3*int(cnt) + spanSlop
	for _, vertical := range axes {
		ms, mp, sa, sb := int(math.Round(mx)), my, ax, bx
		if vertical {
			ms, mp, sa, sb = int(math.Round(my)), mx, ay, by
		}
		sgn := sign(sb - sa)
		if sgn == 0 {
			continue
		}
		base := int(math.Round(mp - 0.5))
		for _, o := range [5]int{0, -1, 1, -2, 2} {
			s := strip{vertical: vertical, p0: base + o}
			if !rt.bankWalk(&s, ms, sgn, maxSpan) {
				continue
			}
			ka, linkA, okA := rt.linkIn(path, s, a, lo)
			if !okA {
				continue
			}
			kb, linkB, okB := rt.linkOut(path, s, b)
			if !okB {
				continue
			}
			return s, ka, kb, linkA, linkB, true
		}
	}
	return st, 0, 0, nil, nil, false
}

func containsBool(s []bool, v bool) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// inMap reports whether along-axis position s, lane p is on the map.
func (rt *router) inMap(st strip, s, p int) bool {
	if st.vertical {
		return p >= 0 && p < rt.w && s >= 0 && s < rt.h
	}
	return p >= 0 && p < rt.h && s >= 0 && s < rt.w
}

// bankWalk walks from ms (which must have water in a lane) both ways
// along the axis to the first position where both lanes are dry, sets s0
// (on the -sgn side, the road's way in) and s1, and reports whether the
// strip is sound: within maxSpan, on the map, banks dry, buildable and
// passable, nothing under it rock, sea or vault.
func (rt *router) bankWalk(st *strip, ms, sgn, maxSpan int) bool {
	w := rt.w
	if !rt.inMap(*st, ms, st.p0) || !rt.inMap(*st, ms, st.p0+1) {
		return false
	}
	if !rt.wet[st.cell(w, ms, st.p0)] && !rt.wet[st.cell(w, ms, st.p0+1)] {
		return false
	}
	walk := func(dir int) (int, bool) {
		for s := ms; abs(s-ms) <= maxSpan; s += dir {
			if !rt.inMap(*st, s, st.p0) || !rt.inMap(*st, s, st.p0+1) {
				return 0, false
			}
			bank := true
			for p := st.p0; p <= st.p0+1; p++ {
				i := st.cell(w, s, p)
				if rt.isRock(i) || rt.cost[i] < 0 {
					return 0, false
				}
				bank = bank && !rt.isWater(i)
			}
			if bank {
				return s, true
			}
		}
		return 0, false
	}
	s0, ok0 := walk(-sgn)
	s1, ok1 := walk(sgn)
	if !ok0 || !ok1 || abs(s1-s0) > maxSpan {
		return false
	}
	st.s0, st.s1 = s0, s1
	return true
}

// target is the bank lane cell at position s nearer to cell i.
func (rt *router) target(st strip, s int, i int32) int32 {
	x, y := int(i)%rt.w, int(i)/rt.w
	a, b := st.cell(rt.w, s, st.p0), st.cell(rt.w, s, st.p0+1)
	da := abs(a%rt.w-x) + abs(a/rt.w-y)
	db := abs(b%rt.w-x) + abs(b/rt.w-y)
	if db < da {
		return int32(b)
	}
	return int32(a)
}

// linkIn finds the latest path index ka in [max(lo, a-linkBack), a] with a
// clear straight link from path[ka] to the strip's way-in bank.
func (rt *router) linkIn(path []int32, st strip, a, lo int) (int, []int32, bool) {
	for ka := a; ka >= max(lo, a-linkBack); ka-- {
		t := rt.target(st, st.s0, path[ka])
		if t == path[ka] {
			return ka, nil, true
		}
		if cells, ok := rt.link(path[ka], t); ok {
			return ka, cells, true
		}
	}
	return 0, nil, false
}

// linkOut finds the earliest path index kb in [b, b+linkBack] with a clear
// straight link from the strip's far bank to path[kb]; the link starts at
// the bank cell and excludes path[kb].
func (rt *router) linkOut(path []int32, st strip, b int) (int, []int32, bool) {
	for kb := b; kb <= min(len(path)-1, b+linkBack); kb++ {
		t := rt.target(st, st.s1, path[kb])
		if t == path[kb] {
			return kb, nil, true
		}
		if cells, ok := rt.link(t, path[kb]); ok {
			return kb, append([]int32{t}, cells[:len(cells)-1]...), true
		}
	}
	return 0, nil, false
}

// link is the Bresenham line from a (excluded) to b over passable dry
// ground (no water, rock or impassable cells).
func (rt *router) link(a, b int32) ([]int32, bool) {
	sum, cells := rt.line(int(a), int(b), nil)
	if sum < 0 {
		return nil, false
	}
	for _, i := range cells {
		if rt.isRock(int(i)) || rt.isWater(int(i)) {
			return nil, false
		}
	}
	return cells, true
}

// riverNormal is the unit normal of the river centreline segment nearest
// (x, y), if the river has one.
func (rt *router) riverNormal(x, y float64) (mapgen.PointF, bool) {
	line := rt.c.F.RiverLine
	best, bd := -1, math.Inf(1)
	for k := 0; k+1 < len(line); k++ {
		if d := segDist(x, y, line[k], line[k+1]); d < bd {
			best, bd = k, d
		}
	}
	if best < 0 {
		return mapgen.PointF{}, false
	}
	dx, dy := line[best+1].X-line[best].X, line[best+1].Y-line[best].Y
	l := math.Hypot(dx, dy)
	if l < 1e-9 {
		return mapgen.PointF{}, false
	}
	return mapgen.PointF{X: -dy / l, Y: dx / l}, true
}

// layStrip lays a strip: its bank rows and dry cells become Track, water,
// marsh and mud under it bridge.
func (rt *router) layStrip(st strip, changed map[int]bool) {
	c := rt.c
	dir := sign(st.s1 - st.s0)
	for s := st.s0; ; s += dir {
		for p := st.p0; p <= st.p0+1; p++ {
			i := st.cell(rt.w, s, p)
			if c.F.Road[i] {
				continue
			}
			bank := s == st.s0 || s == st.s1
			if !bank && (rt.isWater(i) || terrain.Bridgeable(c.R.Terrain.Cells[i])) {
				rt.addBridge(i)
				continue
			}
			rt.ground(i, true)
			changed[i] = true
		}
		if s == st.s1 {
			break
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
