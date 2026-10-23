package pathing

import (
	"sync"
	"testing"

	"rimworld-go/internal/domain/world/grid"
	"rimworld-go/pkg/randx"
)

func oldFind(q Query, sx, sy, tx, ty int) ([]Point, bool) {
	if q == nil || !inBounds(q, sx, sy) || !q.Walkable(tx, ty) {
		return nil, false
	}
	if sx == tx && sy == ty {
		return nil, true
	}

	w, h := q.Width(), q.Height()
	idx := func(x, y int) int { return y*w + x }
	sc := oldGetScratch(w * h)
	defer oldScratchPool.Put(sc)

	start := idx(sx, sy)
	goal := idx(tx, ty)
	sc.touch(start)
	sc.g[start] = 0

	coster, _ := q.(Coster)
	sc.open.push(oldNode{f: octile(sx, sy, tx, ty), g: 0, x: sx, y: sy})

	for sc.open.len() > 0 {
		cur := sc.open.pop()
		ci := idx(cur.x, cur.y)
		if sc.closed[ci] {
			continue
		}
		if ci == goal {
			return oldReconstruct(sc.came, w, start, goal), true
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
			sc.open.push(oldNode{
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
type oldScratch struct {
	stamp  []uint32
	gen    uint32
	g      []int
	came   []int32
	closed []bool
	open   oldHeap
}

var oldScratchPool = sync.Pool{New: func() any { return new(oldScratch) }}

func oldGetScratch(n int) *oldScratch {
	sc := oldScratchPool.Get().(*oldScratch)
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
func (sc *oldScratch) touch(i int) {
	if sc.stamp[i] != sc.gen {
		sc.stamp[i] = sc.gen
		sc.g[i] = int(^uint(0) >> 1)
		sc.came[i] = -1
		sc.closed[i] = false
	}
}

func oldReconstruct(came []int32, w, start, goal int) []Point {
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

type oldNode struct {
	f, g, x, y int
}

// less reports whether a should be popped before b (stable A* tie-break).
func (a oldNode) less(b oldNode) bool {
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

type oldHeap []oldNode

func (h oldHeap) len() int { return len(h) }

func (h *oldHeap) push(n oldNode) {
	*h = append(*h, n)
	h.up(len(*h) - 1)
}

func (h *oldHeap) pop() oldNode {
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

func (h oldHeap) up(i int) {
	for i > 0 {
		p := (i - 1) / 2
		if !h[i].less(h[p]) {
			return
		}
		h[i], h[p] = h[p], h[i]
		i = p
	}
}

func (h oldHeap) down(i int) {
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

// TestFindMatchesPrevious: the packed-node, bitset search returns exactly
// the previous implementation's paths (same tie-breaks), with and
// without extra costs.
func TestFindMatchesPrevious(t *testing.T) {
	for seed := uint64(1); seed <= 300; seed++ {
		r := randx.New(seed)
		w, h := 4+r.IntN(40), 4+r.IntN(40)
		g := grid.New(w, h)
		extra := map[Point]int{}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				switch v := r.IntN(10); {
				case v < 3:
					g.SetWalkable(x, y, false)
				case v == 3:
					extra[Point{X: x, Y: y}] = r.IntN(40) - 5
				}
			}
		}
		var q Query = g
		if seed%2 == 0 {
			q = costed{Grid: g, extra: extra}
		}
		for k := 0; k < 20; k++ {
			sx, sy, tx, ty := r.IntN(w), r.IntN(h), r.IntN(w), r.IntN(h)
			a, oka := Find(q, sx, sy, tx, ty)
			b, okb := oldFind(q, sx, sy, tx, ty)
			if oka != okb || len(a) != len(b) {
				t.Fatalf("seed %d: new %v %v, old %v %v", seed, oka, a, okb, b)
			}
			for i := range a {
				if a[i] != b[i] {
					t.Fatalf("seed %d (%d,%d)->(%d,%d): paths differ at %d: %v vs %v", seed, sx, sy, tx, ty, i, a, b)
				}
			}
		}
	}
}
