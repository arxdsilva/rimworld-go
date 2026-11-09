package game

import "testing"

// TestBucketIndexRebuildsBounded: however many animals, searches and
// sensors ask, the creature index is built at most twice in a tick (the
// first query of the tick, then phaseAnimals once the jobs have run: the
// second build is behaviour the goldens pin) and the food index at most
// once.
func TestBucketIndexRebuildsBounded(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 7, Width: 120, Height: 120})
	var creatures, food uint64
	for tick := range 1500 {
		c0, f0 := g.wl.idx.builds, g.wl.food.builds
		g.Step(1)
		if d := g.wl.idx.builds - c0; d > 2 {
			t.Fatalf("tick %d: creature index rebuilt %d times", tick, d)
		}
		if d := g.wl.food.builds - f0; d > 1 {
			t.Fatalf("tick %d: food index rebuilt %d times", tick, d)
		}
		creatures = g.wl.idx.builds
		if tick%50 == 0 { // ask for the food index repeatedly within one tick
			f1 := g.wl.food.builds
			g.food()
			g.food()
			if g.wl.food.builds-f1 > 1 {
				t.Fatalf("tick %d: two food queries rebuilt the index %d times", tick, g.wl.food.builds-f1)
			}
		}
	}
	food = g.wl.food.builds
	if creatures == 0 || food == 0 {
		t.Fatalf("indexes never built (creatures %d, food %d): the test proves nothing", creatures, food)
	}
}
