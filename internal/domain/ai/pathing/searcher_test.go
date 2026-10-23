package pathing

import (
	"testing"

	"rimworld-go/internal/domain/world/grid"
	"rimworld-go/pkg/randx"
)

// openMap is a w×h grid with about pct% of cells blocked (open ground
// with scattered obstacles: the wide equal-cost plateaus of real maps).
func openMap(seed uint64, w, h, pct int) *grid.Grid {
	r := randx.New(seed)
	g := grid.New(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if r.IntN(100) < pct {
				g.SetWalkable(x, y, false)
			}
		}
	}
	return g
}

// TestSearcherMatchesPrevious: a reused Searcher (and FindIn with every
// cell allowed) returns exactly the previous implementation's paths,
// search after search, on small mazes and on large open maps.
func TestSearcherMatchesPrevious(t *testing.T) {
	var s Searcher
	check := func(name string, q Query, sx, sy, tx, ty int) {
		t.Helper()
		want, okw := oldFind(q, sx, sy, tx, ty)
		for _, v := range []struct {
			how string
			fn  func() ([]Point, bool)
		}{
			{"Find", func() ([]Point, bool) { return Find(q, sx, sy, tx, ty) }},
			{"Searcher.Find", func() ([]Point, bool) { return s.Find(q, sx, sy, tx, ty) }},
			{"Searcher.FindIn", func() ([]Point, bool) {
				n := q.Width() * q.Height()
				all := make([]uint64, (n+63)/64)
				for i := range all {
					all[i] = ^uint64(0)
				}
				return s.FindIn(q, all, sx, sy, tx, ty)
			}},
		} {
			got, ok := v.fn()
			if ok != okw || len(got) != len(want) {
				t.Fatalf("%s %s (%d,%d)->(%d,%d): got %v len %d, want %v len %d", name, v.how, sx, sy, tx, ty, ok, len(got), okw, len(want))
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("%s %s (%d,%d)->(%d,%d): paths differ at %d", name, v.how, sx, sy, tx, ty, i)
				}
			}
		}
	}
	for seed := uint64(1); seed <= 120; seed++ {
		r := randx.New(seed)
		w, h := 4+r.IntN(40), 4+r.IntN(40)
		g := openMap(seed, w, h, 30)
		extra := map[Point]int{}
		for k := 0; k < w*h/10; k++ {
			extra[Point{X: r.IntN(w), Y: r.IntN(h)}] = r.IntN(60) - 5
		}
		var q Query = g
		if seed%2 == 0 {
			q = costed{Grid: g, extra: extra}
		}
		for k := 0; k < 10; k++ {
			check("maze", q, r.IntN(w), r.IntN(h), r.IntN(w), r.IntN(h))
		}
	}
	// costs too big for the packed keys: the search falls back to the
	// wide open list, same paths
	for seed := uint64(1); seed <= 40; seed++ {
		r := randx.New(seed)
		w, h := 4+r.IntN(30), 4+r.IntN(30)
		g := openMap(seed, w, h, 20)
		extra := map[Point]int{}
		for k := 0; k < w*h/8; k++ {
			extra[Point{X: r.IntN(w), Y: r.IntN(h)}] = r.IntN(1 << 28)
		}
		for k := 0; k < 10; k++ {
			check("huge", costed{Grid: g, extra: extra}, r.IntN(w), r.IntN(h), r.IntN(w), r.IntN(h))
		}
	}
	for seed := uint64(1); seed <= 6; seed++ {
		r := randx.New(seed)
		g := openMap(seed, 200, 160, 4+int(seed))
		extra := map[Point]int{}
		for k := 0; k < 400; k++ {
			extra[Point{X: r.IntN(200), Y: r.IntN(160)}] = r.IntN(50)
		}
		var q Query = g
		if seed%2 == 0 {
			q = costed{Grid: g, extra: extra}
		}
		for k := 0; k < 4; k++ {
			check("open", q, r.IntN(200), r.IntN(160), r.IntN(200), r.IntN(160))
		}
	}
}

// BenchmarkSearchOpen500: corner to corner over a 500² open map (the
// long-path worst case: a wide plateau of equal-cost cells).
func BenchmarkSearchOpen500(b *testing.B) {
	g := openMap(1, 500, 500, 5)
	var s Searcher
	g.SetWalkable(2, 2, true)
	g.SetWalkable(497, 490, true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := s.Find(g, 2, 2, 497, 490); !ok {
			b.Fatal("no path")
		}
	}
}

// BenchmarkSearchOpen120: a 100-tile walk on open ground.
func BenchmarkSearchOpen120(b *testing.B) {
	g := openMap(2, 120, 120, 5)
	var s Searcher
	g.SetWalkable(10, 15, true)
	g.SetWalkable(110, 70, true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := s.Find(g, 10, 15, 110, 70); !ok {
			b.Fatal("no path")
		}
	}
}
