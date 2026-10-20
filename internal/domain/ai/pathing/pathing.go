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

	w, h := q.Width(), q.Height()
	idx := func(x, y int) int { return y*w + x }
	sc := getScratch(w * h)
	defer scratchPool.Put(sc)

	start := idx(sx, sy)
	goal := idx(tx, ty)
	sc.touch(start)
	sc.g[start] = 0

	coster, _ := q.(Coster)
	sc.open.push(node{f: octile(sx, sy, tx, ty), g: 0, x: sx, y: sy})

	for sc.open.len() > 0 {
		cur := sc.open.pop()
		ci := idx(cur.x, cur.y)
		if sc.closed[ci] {
			continue
		}
		if ci == goal {
			return reconstruct(sc.came, w, start, goal), true
		}
		sc.closed[ci] = true

		// one walkability probe per neighbour; a diagonal (odd k) also
		// needs both orthogonals beside it (k-1, k+1): no corner cutting
		var open8 [8]bool
		for k, d := range neighbors {
			open8[k] = q.Walkable(cur.x+d[0], cur.y+d[1])
		}
		for k, d := range neighbors {
			if !open8[k] {
				continue
			}
			nx, ny := cur.x+d[0], cur.y+d[1]
			step := StraightCost
			if k&1 == 1 {
				if !open8[k-1] || !open8[(k+1)&7] {
					continue
				}
				step = DiagCost
			}
			ni := idx(nx, ny)
			sc.touch(ni)
			if sc.closed[ni] {
				continue
			}
			if coster != nil {
				if e := coster.ExtraCost(nx, ny); e > 0 {
					step += e
				}
			}
			ng := cur.g + step
			if ng >= sc.g[ni] {
				continue
			}
			sc.g[ni] = ng
			sc.came[ni] = int32(ci)
			sc.open.push(node{
				f: ng + octile(nx, ny, tx, ty),
				g: ng,
				x: nx,
				y: ny,
			})
		}
	}
	return nil, false
}

// scratch is reusable search state. A cell's g/came/closed are valid only
// when its stamp equals gen, so a search starts without clearing O(n)
// arrays. Pooling never affects results: every search sees fresh state.
type scratch struct {
	stamp  []uint32
	gen    uint32
	g      []int
	came   []int32
	closed []bool
	open   heap
}

var scratchPool = sync.Pool{New: func() any { return new(scratch) }}

func getScratch(n int) *scratch {
	sc := scratchPool.Get().(*scratch)
	if len(sc.stamp) < n {
		sc.stamp = make([]uint32, n)
		sc.g = make([]int, n)
		sc.came = make([]int32, n)
		sc.closed = make([]bool, n)
		sc.gen = 0
	}
	sc.gen++
	if sc.gen == 0 { // wrapped: invalidate every stamp
		clear(sc.stamp)
		sc.gen = 1
	}
	sc.open = sc.open[:0]
	return sc
}

// touch lazily resets cell i for this search.
func (sc *scratch) touch(i int) {
	if sc.stamp[i] != sc.gen {
		sc.stamp[i] = sc.gen
		sc.g[i] = int(^uint(0) >> 1)
		sc.came[i] = -1
		sc.closed[i] = false
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

func reconstruct(came []int32, w, start, goal int) []Point {
	cur := goal
	var rev []Point
	for cur != start && cur >= 0 {
		rev = append(rev, Point{X: cur % w, Y: cur / w})
		cur = int(came[cur])
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

type node struct {
	f, g, x, y int
}

// less reports whether a should be popped before b (stable A* tie-break).
func (a node) less(b node) bool {
	if a.f != b.f {
		return a.f < b.f
	}
	if a.g != b.g {
		return a.g < b.g
	}
	if a.y != b.y {
		return a.y < b.y
	}
	return a.x < b.x
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
