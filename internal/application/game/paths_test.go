package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/randx"
)

// openColony is a w×h colony on open soil with rock blobs and a walled
// hut or two.
func openColony(t *testing.T, seed uint64, w, h int) *Game {
	t.Helper()
	g := NewColony(Config{Seed: seed, Width: w, Height: h})
	g.ClearPlants(0, 0, w-1, h-1)
	g.SetGround(0, 0, w-1, h-1, "soil", "", "")
	r := randx.New(seed)
	for k := 0; k < w*h/400; k++ {
		x, y := r.IntN(w), r.IntN(h)
		g.SetGround(x, y, x+r.IntN(6), y+r.IntN(6), "rock", "granite", "")
	}
	for k := 0; k < 3; k++ {
		x0, y0 := r.IntN(w-12), r.IntN(h-12)
		for y := y0; y <= y0+8; y++ {
			for x := x0; x <= x0+10; x++ {
				if x == x0 || x == x0+10 || y == y0 || y == y0+8 {
					kind := "wall"
					if y == y0+4 && x == x0 {
						kind = "door"
					}
					g.BuildNow(kind, "wood", "south", x, y)
				}
			}
		}
	}
	return g
}

func routeCostOf(sx, sy int, p []pathing.Point) int {
	c := 0
	for _, s := range p {
		c += pathing.Octile(sx, sy, s.X, s.Y)
		sx, sy = s.X, s.Y
	}
	return c
}

// TestPathsExactBelowLongThreshold: a colonist's path is exactly A*'s
// when the ends are at most longPath apart, and a legal path to the same
// target costing at most 10% more beyond it.
func TestPathsExactBelowLongThreshold(t *testing.T) {
	g := openColony(t, 3, 260, 90)
	id := g.pawnIDs[0]
	r := randx.New(9)
	w, h := g.grid.Width(), g.grid.Height()
	long, short := 0, 0
	for k := 0; k < 300; k++ {
		sx, sy, tx, ty := r.IntN(w), r.IntN(h), r.IntN(w), r.IntN(h)
		if !g.grid.Walkable(sx, sy) || !g.grid.Walkable(tx, ty) {
			continue
		}
		g.positions.Set(id, Position{X: sx, Y: sy})
		got, ok := g.pathTo(id, tx, ty)
		want, wok := pathing.Find(plainQuery{g.grid}, sx, sy, tx, ty)
		if ok != wok {
			t.Fatalf("(%d,%d)->(%d,%d): found %v, A* %v", sx, sy, tx, ty, ok, wok)
		}
		if !ok {
			continue
		}
		if pathing.Octile(sx, sy, tx, ty) <= longPath {
			short++
			if len(got) != len(want) {
				t.Fatalf("short path differs from A*: %v vs %v", got, want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("short path differs from A* at %d", i)
				}
			}
			continue
		}
		long++
		px, py := sx, sy
		for _, s := range got {
			if !g.grid.Walkable(s.X, s.Y) || !pathing.Adjacent(px, py, s.X, s.Y) ||
				s.X != px && s.Y != py && (!g.grid.Walkable(s.X, py) || !g.grid.Walkable(px, s.Y)) {
				t.Fatalf("illegal step (%d,%d)->(%d,%d)", px, py, s.X, s.Y)
			}
			px, py = s.X, s.Y
		}
		if px != tx || py != ty {
			t.Fatal("long path misses its target")
		}
		if c, o := routeCostOf(sx, sy, got), routeCostOf(sx, sy, want); c*10 > o*11 {
			t.Fatalf("(%d,%d)->(%d,%d): corridor path %d, optimal %d", sx, sy, tx, ty, c, o)
		}
	}
	if long < 20 || short < 20 {
		t.Fatalf("sampled %d long and %d short paths", long, short)
	}
}

// TestLongSearchBudgetDeterministic: in one tick only longSearchBudget
// long job searches run, in the order the jobs start (the AI's entity-id
// order); the rest fail as deferred (not unreachable, not postponed) and
// get their search on a later tick. Two identical games agree exactly.
func TestLongSearchBudgetDeterministic(t *testing.T) {
	run := func() ([]job.Record, uint64) {
		g := openColony(t, 5, 240, 40)
		var ids []entity.EntityID
		for k := 0; k < 10; k++ {
			id, err := g.SpawnNamedPawn(2, 2+3*k, "Walker")
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		started := 0
		for k, id := range ids {
			ok := g.startJob(id, job.Record{Kind: job.Move, TX: 236, TY: 2 + 3*k})
			rec, _ := g.jobs.Get(id)
			if ok {
				started++
				continue
			}
			if rec.Fail != job.Reserved || k < longSearchBudget {
				t.Fatalf("walker %d: fail %v (started %d)", k, rec.Fail, started)
			}
		}
		if started != longSearchBudget {
			t.Fatalf("%d long searches in one tick, budget %d", started, longSearchBudget)
		}
		// the next tick serves the deferred ones (lowest ids first)
		g.clock.Advance(1)
		for k, id := range ids[longSearchBudget:] {
			if !g.startJob(id, job.Record{Kind: job.Move, TX: 236, TY: 2 + 3*(k+longSearchBudget)}) {
				t.Fatalf("deferred walker %d not served next tick", k+longSearchBudget)
			}
		}
		var recs []job.Record
		for _, id := range ids {
			rec, _ := g.jobs.Get(id)
			recs = append(recs, rec)
		}
		g.Step(200)
		return recs, g.StateHash()
	}
	a, ha := run()
	b, hb := run()
	if ha != hb {
		t.Fatal("identical games diverged")
	}
	for i := range a {
		if len(a[i].Path) != len(b[i].Path) {
			t.Fatalf("walker %d: paths differ between identical games", i)
		}
	}
}
