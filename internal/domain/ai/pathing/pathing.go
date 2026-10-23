// Package pathing owns deterministic tile pathfinding.
package pathing

import "sync"

// Query is the walkability surface a search needs. grid.Grid satisfies it.
type Query interface {
	Width() int
	Height() int
	Walkable(x, y int) bool
}

// Coster is an optional Query extension: ExtraCost is added to the cost of
// entering (x, y). Negative values count as 0, so the octile heuristic stays
// admissible. Find type-asserts it; plain queries cost nothing extra.
type Coster interface {
	ExtraCost(x, y int) int
}

// Step costs: octile distance, a diagonal is ~sqrt(2) of a straight step.
const (
	StraightCost = 10
	DiagCost     = 14
)

// Point is a tile coordinate on a path.
type Point struct {
	X, Y int
}

// neighbors is clockwise from north: even indices are orthogonal, odd
// ones diagonal (Find relies on this layout).
var neighbors = [8][2]int{
	{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1},
}

// Reacher is an optional Query extension: Reach reports whether any
// walkable path from (sx, sy) to (tx, ty) can exist (a region-label
// check). Find fails at once when it reports false, instead of flooding
// the start's whole component. It must never report false when a path
// exists.
type Reacher interface {
	Reach(sx, sy, tx, ty int) bool
}

// Bitset is an optional Query extension: BlockedBits is the row-major
// bitset of cells that are not walkable (bit y*Width()+x), agreeing with
// Walkable on every in-bounds cell. Find reads it instead of calling
// Walkable per neighbour. A type that embeds a Bitset but narrows
// Walkable must not let BlockedBits be promoted. A slice shorter than the
// map is ignored.
type Bitset interface {
	BlockedBits() []uint64
}

// Find returns the cheapest walkable 8-connected path from (sx,sy) to
// (tx,ty): octile costs (10 straight, 14 diagonal, plus Coster extras), and
// a diagonal step needs both orthogonal neighbours walkable (no corner
// cutting). The slice excludes the start tile and includes the destination.
// An empty slice with ok=true means the pawn is already there.
// Tie-break: lower f, then lower g, then lower y, then lower x.
func Find(q Query, sx, sy, tx, ty int) ([]Point, bool) {
	if q == nil || !inBounds(q, sx, sy) || !q.Walkable(tx, ty) {
		return nil, false
	}
	if sx == tx && sy == ty {
		return nil, true
	}
	if r, ok := q.(Reacher); ok && !r.Reach(sx, sy, tx, ty) {
		return nil, false
	}
	return search(q, sx, sy, tx, ty, nil)
}

// FindIn is Find restricted to the cells set in allow (row-major bitset;
// the start always counts as allowed): a corridor search. It skips the
// Reacher check (the corridor was chosen from the region graph).
func FindIn(q Query, allow []uint64, sx, sy, tx, ty int) ([]Point, bool) {
	if q == nil || !inBounds(q, sx, sy) || !q.Walkable(tx, ty) {
		return nil, false
	}
	if sx == tx && sy == ty {
		return nil, true
	}
	return search(q, sx, sy, tx, ty, allow)
}

// search is the A* behind Find: allow, when non-nil, limits the cells it
// may enter.
func search(q Query, sx, sy, tx, ty int, allow []uint64) ([]Point, bool) {
	w, h := q.Width(), q.Height()
	n := w * h
	var bits []uint64
	if b, ok := q.(Bitset); ok {
		if bits = b.BlockedBits(); len(bits) < (n+63)/64 {
			bits = nil
		}
	}
	if allow != nil && len(allow) < (n+63)/64 {
		allow = nil
	}
	sc := getScratch(n)
	defer scratchPool.Put(sc)

	start := int32(sy*w + sx)
	goal := int32(ty*w + tx)
	sc.touch(start)
	sc.cells[start].g = 0

	coster, _ := q.(Coster)
	sc.open.push(node{fg: pack(octile(sx, sy, tx, ty), 0), i: start})

	cells := sc.cells
	for sc.open.len() > 0 {
		cur := sc.open.pop()
		ci := cur.i
		c := &cells[ci]
		if c.closed {
			continue
		}
		if ci == goal {
			return reconstruct(cells, w, start, goal), true
		}
		c.closed = true
		cx, cy := int(ci)%w, int(ci)/w
		cg := int(c.g) // the entry popped first for a cell carries its best g

		// one walkability probe per neighbour; a diagonal (odd k) also
		// needs both orthogonals beside it (k-1, k+1): no corner cutting
		var open8 [8]bool
		for k, d := range neighbors {
			nx, ny := cx+d[0], cy+d[1]
			if uint(nx) >= uint(w) || uint(ny) >= uint(h) {
				continue
			}
			ni := uint(ny*w + nx)
			if allow != nil && allow[ni>>6]&(1<<(ni&63)) == 0 {
				continue
			}
			if bits != nil {
				open8[k] = bits[ni>>6]&(1<<(ni&63)) == 0
			} else {
				open8[k] = q.Walkable(nx, ny)
			}
		}
		for k, d := range neighbors {
			if !open8[k] {
				continue
			}
			nx, ny := cx+d[0], cy+d[1]
			step := StraightCost
			if k&1 == 1 {
				if !open8[k-1] || !open8[(k+1)&7] {
					continue
				}
				step = DiagCost
			}
			ni := int32(ny*w + nx)
			sc.touch(ni)
			nc := &cells[ni]
			if nc.closed {
				continue
			}
			if coster != nil {
				if e := coster.ExtraCost(nx, ny); e > 0 {
					step += e
				}
			}
			ng := cg + step
			if ng >= int(nc.g) {
				continue
			}
			nc.g = int32(ng)
			nc.came = ci
			sc.open.push(node{fg: pack(ng+octile(nx, ny, tx, ty), ng), i: ni})
		}
	}
	return nil, false
}

// pack orders (f, g) in one key: f in the high half, g in the low.
func pack(f, g int) uint64 { return uint64(uint32(f))<<32 | uint64(uint32(g)) }

// cell is one tile's search state (kept together: one cache line touch
// per visit).
type cell struct {
	stamp  uint32
	g      int32
	came   int32
	closed bool
}

// scratch is reusable search state. A cell's state is valid only when its
// stamp equals gen, so a search starts without clearing O(n) arrays.
// Pooling never affects results: every search sees fresh state.
type scratch struct {
	cells []cell
	gen   uint32
	open  heap
}

var scratchPool = sync.Pool{New: func() any { return new(scratch) }}

func getScratch(n int) *scratch {
	sc := scratchPool.Get().(*scratch)
	if len(sc.cells) < n {
		sc.cells = make([]cell, n)
		sc.gen = 0
	}
	sc.gen++
	if sc.gen == 0 { // wrapped: invalidate every stamp
		for i := range sc.cells {
			sc.cells[i].stamp = 0
		}
		sc.gen = 1
	}
	sc.open = sc.open[:0]
	return sc
}

// touch lazily resets cell i for this search.
func (sc *scratch) touch(i int32) {
	c := &sc.cells[i]
	if c.stamp != sc.gen {
		*c = cell{stamp: sc.gen, g: 1<<31 - 1, came: -1}
	}
}

func inBounds(q Query, x, y int) bool {
	return x >= 0 && y >= 0 && x < q.Width() && y < q.Height()
}

// octile is the exact cost of an unobstructed 8-connected walk.
func octile(ax, ay, bx, by int) int {
	dx := ax - bx
	if dx < 0 {
		dx = -dx
	}
	dy := ay - by
	if dy < 0 {
		dy = -dy
	}
	if dx < dy {
		dx, dy = dy, dx
	}
	return StraightCost*(dx-dy) + DiagCost*dy
}

// Octile is the octile distance (10 per straight, 14 per diagonal step).
func Octile(ax, ay, bx, by int) int { return octile(ax, ay, bx, by) }

func chebyshev(ax, ay, bx, by int) int {
	dx := ax - bx
	if dx < 0 {
		dx = -dx
	}
	dy := ay - by
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

func reconstruct(cells []cell, w int, start, goal int32) []Point {
	n := 0
	for cur := goal; cur != start && cur >= 0; cur = cells[cur].came {
		n++
	}
	out := make([]Point, n)
	for cur := goal; cur != start && cur >= 0; cur = cells[cur].came {
		n--
		out[n] = Point{X: int(cur) % w, Y: int(cur) / w}
	}
	return out
}

// node is an open-list entry: fg packs (f, g), i is the cell index, so
// the order f, g, y, x is (fg, i).
type node struct {
	fg uint64
	i  int32
}

// less reports whether a should be popped before b (stable A* tie-break).
func (a node) less(b node) bool {
	if a.fg != b.fg {
		return a.fg < b.fg
	}
	return a.i < b.i
}

type heap []node

func (h heap) len() int { return len(h) }

func (h *heap) push(n node) {
	*h = append(*h, n)
	h.up(len(*h) - 1)
}

func (h *heap) pop() node {
	old := *h
	n := old[0]
	last := old[len(old)-1]
	*h = old[:len(old)-1]
	if len(*h) > 0 {
		(*h)[0] = last
		h.down(0)
	}
	return n
}

func (h heap) up(i int) {
	for i > 0 {
		p := (i - 1) / 2
		if !h[i].less(h[p]) {
			return
		}
		h[i], h[p] = h[p], h[i]
		i = p
	}
}

func (h heap) down(i int) {
	n := len(h)
	for {
		l := 2*i + 1
		if l >= n {
			return
		}
		min := l
		if r := l + 1; r < n && h[r].less(h[l]) {
			min = r
		}
		if !h[min].less(h[i]) {
			return
		}
		h[i], h[min] = h[min], h[i]
		i = min
	}
}

// Dist is Chebyshev distance, used for cheap job scoring.
func Dist(ax, ay, bx, by int) int {
	return chebyshev(ax, ay, bx, by)
}

// Adjacent reports whether two tiles share an edge or corner.
func Adjacent(ax, ay, bx, by int) bool {
	dx := ax - bx
	if dx < 0 {
		dx = -dx
	}
	dy := ay - by
	if dy < 0 {
		dy = -dy
	}
	return dx <= 1 && dy <= 1 && (dx+dy) > 0
}
