package game

import (
	"fmt"
	"testing"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/world/region"
	"rimworld-go/pkg/randx"
)

// regionQueries is every layer's path query as the game builds it.
func regionQueries(g *Game) [region.Layers]pathing.Query {
	return [region.Layers]pathing.Query{
		region.Colonist: colonyQuery{g},
		region.Hostile:  g.hostileMap(),
		region.Sapper:   sapperMap{hostileMap: g.hostileMap(), traps: g.buildingCells(construction.SpikeTrap)},
		region.Tame:     g.animalQuery(true, g.animalBase(true)),
		region.Wild:     g.animalQuery(false, g.animalBase(false)),
	}
}

// checkRegionQueries: every layer's bitset is its query's walkability,
// and Reach (through the query, as Find asks it) is what a plain A*
// finds.
func checkRegionQueries(t *testing.T, g *Game, r *randx.Rand, pairs int) {
	t.Helper()
	w, h := g.grid.Width(), g.grid.Height()
	for l, q := range regionQueries(g) {
		bits := q.(pathing.Bitset).BlockedBits()
		if bits == nil {
			t.Fatalf("layer %d: no bitset", l)
		}
		for i := 0; i < w*h; i++ {
			blocked := bits[i>>6]&(1<<(uint(i)&63)) != 0
			if blocked == q.Walkable(i%w, i/w) {
				t.Fatalf("layer %d cell (%d,%d): bitset blocked=%v, Walkable=%v", l, i%w, i/w, blocked, !blocked)
			}
		}
		rq := q.(pathing.Reacher)
		for k := 0; k < pairs; k++ {
			sx, sy, tx, ty := r.IntN(w), r.IntN(h), r.IntN(w), r.IntN(h)
			if !q.Walkable(tx, ty) || sx == tx && sy == ty {
				continue
			}
			_, want := pathing.Find(plainQuery{q}, sx, sy, tx, ty)
			if got := rq.Reach(sx, sy, tx, ty); got != want {
				t.Fatalf("layer %d (%d,%d)->(%d,%d): Reach=%v, A*=%v", l, sx, sy, tx, ty, got, want)
			}
		}
	}
}

// TestRegionReachMatchesQueries builds walls, doors, fences, gates and
// traps at random, mines and floods cells, and removes buildings again;
// after every batch each layer's region answers match its path query.
func TestRegionReachMatchesQueries(t *testing.T) {
	kinds := []string{"wall", "wall", "wall", "door", "fence", "fence", "fence gate", "spike trap"}
	for seed := uint64(1); seed <= 4; seed++ {
		r := randx.New(seed)
		g := NewColony(Config{Seed: seed, Width: 64, Height: 52})
		checkRegionQueries(t, g, r, 60)
		for batch := 0; batch < 12; batch++ {
			for k := 0; k < 25; k++ {
				x, y := r.IntN(64), r.IntN(52)
				switch v := r.IntN(10); {
				case v < 6:
					g.BuildNow(kinds[r.IntN(len(kinds))], "wood", "south", x, y)
				case v == 6:
					g.SetGround(x, y, x, y, "soil", "", "") // mined out / drained
				case v == 7:
					g.SetGround(x, y, x+1, y, "rock", "granite", "")
				case v == 8:
					if n := len(g.bldgIDs); n > 0 {
						g.removeBuilding(g.bldgIDs[r.IntN(n)])
					}
				default:
					// a wall line (a room's side)
					for d := 0; d < 6; d++ {
						g.BuildNow("wall", "wood", "south", x+d, y)
					}
				}
			}
			checkRegionQueries(t, g, r, 40)
		}
	}
}

// TestRegionsDoNotChangeState: a colony run with region answers is
// bit-identical to one with plain searches (with the self-check on, every
// region answer is also proven against A* along the way).
func TestRegionsDoNotChangeState(t *testing.T) {
	if testing.Short() {
		t.Skip("long run")
	}
	for _, c := range []struct {
		w, h  int
		seed  uint64
		ticks int
	}{{48, 40, 1, 20000}, {48, 40, 42, 20000}, {120, 120, 7, 12000}} {
		t.Run(fmt.Sprintf("%dx%d/seed%d", c.w, c.h, c.seed), func(t *testing.T) {
			t.Parallel()
			on := NewColony(Config{Seed: c.seed, Width: c.w, Height: c.h})
			off := NewColony(Config{Seed: c.seed, Width: c.w, Height: c.h})
			on.regs.exactLong = true // corridors change long paths on purpose
			off.regs.off = true
			for done := 0; done < c.ticks; done += 2000 {
				on.Step(2000)
				off.Step(2000)
				if a, b := on.StateHash(), off.StateHash(); a != b {
					t.Fatalf("diverged by tick %d: %#x vs %#x", done+2000, a, b)
				}
			}
		})
	}
}

// TestFenceReachableUsesLayers: an animal's food filter follows the
// fences (a tame area animal through the gate; wild animals and, PLAY-015,
// livestock not).
func TestFenceReachableUsesLayers(t *testing.T) {
	g := NewColony(Config{Seed: 2, Width: 40, Height: 30})
	g.SetGround(0, 0, 39, 29, "soil", "", "")
	g.ClearPlants(0, 0, 39, 29)
	for y := 0; y < 30; y++ {
		kind := "fence"
		if y == 15 {
			kind = "fence gate"
		}
		if !g.BuildNow(kind, "wood", "south", 20, y) {
			t.Fatalf("%s at 20,%d", kind, y)
		}
	}
	for _, tame := range []bool{true, false} {
		a := animal.State{Species: animal.Husky, Tame: tame}
		ok := g.fenceReachable(&a, Position{X: 5, Y: 5})
		if ok == nil {
			t.Fatal("fences stand: want a filter")
		}
		if !ok(10, 10) {
			t.Fatalf("tame=%v: own side unreachable", tame)
		}
		if got := ok(30, 10); got != tame {
			t.Fatalf("tame=%v: far side reachable=%v", tame, got)
		}
		if ok(20, 3) {
			t.Fatalf("tame=%v: food on a fence counted", tame)
		}
	}
	cow := animal.State{Species: animal.Cow, Tame: true}
	if ok := g.fenceReachable(&cow, Position{X: 5, Y: 5}); ok(30, 10) || !ok(10, 10) {
		t.Fatal("livestock feed on their own side of a gate")
	}
}
