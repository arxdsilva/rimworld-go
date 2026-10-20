package pathing

import (
	"testing"

	"rimworld-go/internal/domain/world/grid"
	"rimworld-go/pkg/randx"
)

// costed wraps a grid with per-cell extra costs.
type costed struct {
	*grid.Grid
	extra map[Point]int
}

func (c costed) ExtraCost(x, y int) int { return c.extra[Point{X: x, Y: y}] }

// pathCost is the octile cost of walking p from (sx, sy), plus extras.
func pathCost(q Query, sx, sy int, p []Point) int {
	cost := 0
	px, py := sx, sy
	c, _ := q.(Coster)
	for _, s := range p {
		if s.X != px && s.Y != py {
			cost += DiagCost
		} else {
			cost += StraightCost
		}
		if c != nil {
			cost += c.ExtraCost(s.X, s.Y)
		}
		px, py = s.X, s.Y
	}
	return cost
}

// legal checks every step is a walkable 8-neighbour with no corner cut.
func legal(t *testing.T, q Query, sx, sy int, p []Point) {
	t.Helper()
	px, py := sx, sy
	for i, s := range p {
		dx, dy := s.X-px, s.Y-py
		if dx < -1 || dx > 1 || dy < -1 || dy > 1 || dx == 0 && dy == 0 {
			t.Fatalf("step %d %v is not a neighbour of (%d,%d)", i, s, px, py)
		}
		if !q.Walkable(s.X, s.Y) {
			t.Fatalf("step %d %v is not walkable", i, s)
		}
		if dx != 0 && dy != 0 && (!q.Walkable(px+dx, py) || !q.Walkable(px, py+dy)) {
			t.Fatalf("step %d %v cuts a corner from (%d,%d)", i, s, px, py)
		}
		px, py = s.X, s.Y
	}
}

func TestOctileCostPrefersStraightWhenEqual(t *testing.T) {
	g := grid.New(5, 3)
	p, ok := Find(g, 0, 1, 2, 1)
	if !ok {
		t.Fatal("expected a path")
	}
	want := []Point{{1, 1}, {2, 1}}
	if len(p) != len(want) || p[0] != want[0] || p[1] != want[1] {
		t.Fatalf("path = %v, want straight %v", p, want)
	}
}

func TestOctileUsesDiagonalWhenShorter(t *testing.T) {
	g := grid.New(6, 6)
	p, ok := Find(g, 0, 0, 3, 3)
	if !ok || len(p) != 3 {
		t.Fatalf("path = %v ok=%v, want 3 diagonal steps", p, ok)
	}
	if c := pathCost(g, 0, 0, p); c != 3*DiagCost {
		t.Fatalf("cost = %d, want %d", c, 3*DiagCost)
	}
}

func TestNoCornerCutting(t *testing.T) {
	// . # .
	// . . .   (0,0) -> (2,0) must not slip diagonally past the wall
	g := grid.New(3, 2)
	g.SetWalkable(1, 0, false)
	p, ok := Find(g, 0, 0, 2, 0)
	if !ok {
		t.Fatal("expected a path around the wall")
	}
	legal(t, g, 0, 0, p)
	if len(p) != 4 {
		t.Fatalf("path = %v, want 4 orthogonal steps", p)
	}

	// two walls touching at a corner seal the diagonal completely
	h := grid.New(2, 2)
	h.SetWalkable(1, 0, false)
	h.SetWalkable(0, 1, false)
	if p, ok := Find(h, 0, 0, 1, 1); ok {
		t.Fatalf("diagonal squeeze between walls allowed: %v", p)
	}
}

func TestCosterAddsCost(t *testing.T) {
	g := grid.New(5, 3)
	plain, ok := Find(g, 0, 1, 4, 1)
	if !ok || plain[1] != (Point{X: 2, Y: 1}) {
		t.Fatalf("plain path = %v, want through (2,1)", plain)
	}
	c := costed{Grid: g, extra: map[Point]int{{2, 1}: 40}}
	p, ok := Find(c, 0, 1, 4, 1)
	if !ok {
		t.Fatal("expected a path")
	}
	legal(t, c, 0, 1, p)
	for _, s := range p {
		if s == (Point{X: 2, Y: 1}) {
			t.Fatalf("costly cell not avoided: %v", p)
		}
	}
	if pathCost(c, 0, 1, p) >= pathCost(c, 0, 1, plain) {
		t.Fatalf("detour %v not cheaper than %v", p, plain)
	}
	// a small extra cost is not worth a detour
	c.extra[Point{X: 2, Y: 1}] = 2
	p, _ = Find(c, 0, 1, 4, 1)
	if p[1] != (Point{X: 2, Y: 1}) {
		t.Fatalf("cheap extra caused a detour: %v", p)
	}
}

func TestCosterNegativeIgnored(t *testing.T) {
	g := grid.New(5, 3)
	c := costed{Grid: g, extra: map[Point]int{{2, 0}: -100}}
	p, ok := Find(c, 0, 1, 4, 1)
	if !ok || len(p) != 4 {
		t.Fatalf("path = %v, negative cost must count as 0", p)
	}
}

// bruteCost is the optimal octile cost from (sx,sy) to (tx,ty) by
// relaxing every edge until nothing changes (Bellman-Ford).
func bruteCost(q Query, sx, sy, tx, ty int) (int, bool) {
	w, h := q.Width(), q.Height()
	const inf = 1 << 30
	d := make([]int, w*h)
	for i := range d {
		d[i] = inf
	}
	d[sy*w+sx] = 0
	c, _ := q.(Coster)
	for changed := true; changed; {
		changed = false
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if d[y*w+x] == inf {
					continue
				}
				for _, n := range neighbors {
					nx, ny := x+n[0], y+n[1]
					if !inBounds(q, nx, ny) || !q.Walkable(nx, ny) {
						continue
					}
					step := StraightCost
					if n[0] != 0 && n[1] != 0 {
						if !q.Walkable(x+n[0], y) || !q.Walkable(x, y+n[1]) {
							continue
						}
						step = DiagCost
					}
					if c != nil {
						if e := c.ExtraCost(nx, ny); e > 0 {
							step += e
						}
					}
					if nd := d[y*w+x] + step; nd < d[ny*w+nx] {
						d[ny*w+nx] = nd
						changed = true
					}
				}
			}
		}
	}
	return d[ty*w+tx], d[ty*w+tx] < inf
}

func TestOctileOptimal(t *testing.T) {
	for seed := uint64(1); seed <= 40; seed++ {
		r := randx.New(seed)
		g := grid.New(12, 12)
		extra := map[Point]int{}
		for y := 0; y < 12; y++ {
			for x := 0; x < 12; x++ {
				switch v := r.IntN(10); {
				case v < 3:
					g.SetWalkable(x, y, false)
				case v == 3 && seed%2 == 0:
					extra[Point{X: x, Y: y}] = 5 + r.IntN(40)
				}
			}
		}
		var q Query = g
		if seed%2 == 0 {
			q = costed{Grid: g, extra: extra}
		}
		for k := 0; k < 6; k++ {
			sx, sy, tx, ty := r.IntN(12), r.IntN(12), r.IntN(12), r.IntN(12)
			if !g.Walkable(sx, sy) || !g.Walkable(tx, ty) {
				continue
			}
			want, reach := bruteCost(q, sx, sy, tx, ty)
			p, ok := Find(q, sx, sy, tx, ty)
			if ok != reach {
				t.Fatalf("seed %d (%d,%d)->(%d,%d): ok=%v, reachable=%v", seed, sx, sy, tx, ty, ok, reach)
			}
			if !ok {
				continue
			}
			legal(t, q, sx, sy, p)
			if got := pathCost(q, sx, sy, p); got != want {
				t.Fatalf("seed %d (%d,%d)->(%d,%d): cost %d, optimal %d; path %v", seed, sx, sy, tx, ty, got, want, p)
			}
		}
	}
}

func TestDistancesAndAdjacency(t *testing.T) {
	if d := Octile(0, 0, 3, 1); d != 2*StraightCost+DiagCost {
		t.Fatalf("Octile = %d", d)
	}
	if d := Octile(3, 1, 0, 0); d != 2*StraightCost+DiagCost {
		t.Fatalf("Octile reversed = %d", d)
	}
	if d := Dist(0, 5, 3, 1); d != 4 {
		t.Fatalf("Dist = %d, want chebyshev 4", d)
	}
	if d := Dist(3, 1, 0, 0); d != 3 {
		t.Fatalf("Dist = %d, want 3", d)
	}
	if !Adjacent(1, 1, 2, 2) || !Adjacent(2, 2, 1, 1) || Adjacent(1, 1, 1, 1) || Adjacent(0, 0, 2, 0) {
		t.Fatal("Adjacent wrong")
	}
}
