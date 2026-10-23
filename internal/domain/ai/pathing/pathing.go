// Package pathing owns deterministic tile pathfinding.
package pathing

import (
	"math/bits"
	"sync"
)

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
	return pooledSearch(q, sx, sy, tx, ty, nil)
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
	return pooledSearch(q, sx, sy, tx, ty, allow)
}

// Searcher is Find and FindIn with search state of its own, kept between
// searches: one owner (a simulation) reuses it instead of the shared pool,
// which the garbage collector empties (a 500² map's state is 4 MB). It is
// not safe for concurrent use. Results are identical to Find and FindIn:
// every search sees fresh state. The zero value is ready to use.
type Searcher struct {
	sc scratch
}

// Find is pathing.Find on s's state.
func (s *Searcher) Find(q Query, sx, sy, tx, ty int) ([]Point, bool) {
	if q == nil || !inBounds(q, sx, sy) || !q.Walkable(tx, ty) {
		return nil, false
	}
	if sx == tx && sy == ty {
		return nil, true
	}
	if r, ok := q.(Reacher); ok && !r.Reach(sx, sy, tx, ty) {
		return nil, false
	}
	return search(q, sx, sy, tx, ty, nil, s.sc.reset(q.Width()*q.Height()))
}

// FindIn is pathing.FindIn on s's state.
func (s *Searcher) FindIn(q Query, allow []uint64, sx, sy, tx, ty int) ([]Point, bool) {
	if q == nil || !inBounds(q, sx, sy) || !q.Walkable(tx, ty) {
		return nil, false
	}
	if sx == tx && sy == ty {
		return nil, true
	}
	return search(q, sx, sy, tx, ty, allow, s.sc.reset(q.Width()*q.Height()))
}

// pooledSearch is search on state from the shared pool.
func pooledSearch(q Query, sx, sy, tx, ty int, allow []uint64) ([]Point, bool) {
	sc := scratchPool.Get().(*scratch)
	defer scratchPool.Put(sc)
	return search(q, sx, sy, tx, ty, allow, sc.reset(q.Width()*q.Height()))
}

// search is the A* behind Find: allow, when non-nil, limits the cells it
// may enter.
func search(q Query, sx, sy, tx, ty int, allow []uint64, sc *scratch) ([]Point, bool) {
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
	// The open list orders (f, g, cell). Packed into one uint64 (narrow
	// keys: half the memory, one compare) while f and g fit their bit
	// fields; a search whose costs outgrow them is run again on the wide
	// list. Both orders are the same, so are the paths.
	if p, ok, fits := searchOpen(q, sx, sy, tx, ty, allow, bits, sc, newKeys(n)); fits {
		return p, ok
	}
	sc.reset(n)
	p, ok, _ := searchOpen(q, sx, sy, tx, ty, allow, bits, sc, keys{})
	return p, ok
}

// keys is the packing of an open-list entry into one uint64: f above g
// above the cell index. The zero value means "wide": the heap of nodes.
type keys struct {
	on            bool
	iBits, fShift uint
	fMax, gMax    int
	iMask         uint64
}

func newKeys(n int) keys {
	ib := uint(bits.Len(uint(n)))
	gb := (64 - ib) / 2
	fb := 64 - ib - gb
	return keys{on: true, iBits: ib, fShift: ib + gb, fMax: 1<<fb - 1, gMax: 1<<gb - 1, iMask: 1<<ib - 1}
}

// searchOpen is search's A* on the open list kp picks; fits is false when
// a packed key would overflow (the result is then meaningless).
func searchOpen(q Query, sx, sy, tx, ty int, allow, bits []uint64, sc *scratch, kp keys) (path []Point, found, fits bool) {
	w, h := q.Width(), q.Height()
	start := int32(sy*w + sx)
	goal := int32(ty*w + tx)
	sc.touch(start)
	sc.cells[start].g = 0

	coster, _ := q.(Coster)
	if f := octile(sx, sy, tx, ty); kp.on {
		if f > kp.fMax {
			return nil, false, false
		}
		sc.kopen.push(uint64(f)<<kp.fShift | uint64(start))
	} else {
		sc.open.push(node{fg: pack(f, 0), i: start})
	}

	cells := sc.cells
	for {
		var ci int32
		if kp.on {
			if len(sc.kopen) == 0 {
				break
			}
			ci = int32(sc.kopen.pop() & kp.iMask)
		} else {
			if sc.open.len() == 0 {
				break
			}
			ci = sc.open.pop().i
		}
		c := &cells[ci]
		if c.closed {
			continue
		}
		if ci == goal {
			return reconstruct(cells, w, start, goal), true, true
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
			f := ng + octile(nx, ny, tx, ty)
			if kp.on {
				if f > kp.fMax || ng > kp.gMax {
					return nil, false, false
				}
				sc.kopen.push(uint64(f)<<kp.fShift | uint64(ng)<<kp.iBits | uint64(ni))
			} else {
				sc.open.push(node{fg: pack(f, ng), i: ni})
			}
		}
	}
	return nil, false, true
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
	kopen keyHeap
}

var scratchPool = sync.Pool{New: func() any { return new(scratch) }}

// reset readies sc for a search over n cells.
func (sc *scratch) reset(n int) *scratch {
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
	sc.kopen = sc.kopen[:0]
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

// heap is a 4-ary min-heap of open nodes (shallower than a binary heap:
// fewer cache misses on the large open lists of long searches). Keys are
// unique (a cell is pushed again only with a lower g), so the pop order,
// and every path, is the same as with any other exact priority queue.
type heap []node

func (h heap) len() int { return len(h) }

func (h *heap) push(n node) {
	*h = append(*h, n)
	s := *h
	i := len(s) - 1
	for i > 0 {
		p := (i - 1) >> 2
		if !n.less(s[p]) {
			break
		}
		s[i] = s[p]
		i = p
	}
	s[i] = n
}

func (h *heap) pop() node {
	s := *h
	top := s[0]
	last := s[len(s)-1]
	s = s[:len(s)-1]
	*h = s
	n := len(s)
	if n == 0 {
		return top
	}
	i := 0
	for {
		c := 4*i + 1
		if c >= n {
			break
		}
		m := c
		for k, end := c+1, min(c+4, n); k < end; k++ {
			if s[k].less(s[m]) {
				m = k
			}
		}
		if !s[m].less(last) {
			break
		}
		s[i] = s[m]
		i = m
	}
	s[i] = last
	return top
}

// keyHeap is the 4-ary min-heap of packed keys (keys: f, g, cell in one
// uint64, so the order is heap's).
type keyHeap []uint64

func (h *keyHeap) push(v uint64) {
	*h = append(*h, v)
	s := *h
	i := len(s) - 1
	for i > 0 {
		p := (i - 1) >> 2
		if v >= s[p] {
			break
		}
		s[i] = s[p]
		i = p
	}
	s[i] = v
}

func (h *keyHeap) pop() uint64 {
	s := *h
	top := s[0]
	last := s[len(s)-1]
	s = s[:len(s)-1]
	*h = s
	n := len(s)
	i := 0
	for {
		c := 4*i + 1
		if c+3 < n { // all four children: pick the least without branching on order
			m, mv := c, s[c]
			if v := s[c+1]; v < mv {
				m, mv = c+1, v
			}
			if v := s[c+2]; v < mv {
				m, mv = c+2, v
			}
			if v := s[c+3]; v < mv {
				m, mv = c+3, v
			}
			if mv >= last {
				break
			}
			s[i] = mv
			i = m
			continue
		}
		if c >= n {
			break
		}
		m, mv := c, s[c]
		for k := c + 1; k < n; k++ {
			if s[k] < mv {
				m, mv = k, s[k]
			}
		}
		if mv >= last {
			break
		}
		s[i] = mv
		i = m
	}
	if n > 0 {
		s[i] = last
	}
	return top
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
