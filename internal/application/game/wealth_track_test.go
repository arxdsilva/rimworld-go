package game

import (
	"math"
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/wealth"
)

// fullWealth is one full count of the current map (the scan the tracker
// replaces on the tick).
func fullWealth(g *Game) int {
	sum, _ := g.WealthOf(0, math.MaxInt)
	return sum
}

// stepWithWealth steps g n ticks, stepping the current map's wealth
// tracker after each as its phase does (a repeat at one tick is a no-op).
func stepWithWealth(g *Game, n int) {
	for i := 0; i < n; i++ {
		g.Step(1)
		g.wealth.Step(uint64(g.clock.Now()))
	}
}

// TestWealthTrackerMatchesFullScan (EAI-014): one pass of the chunked
// sweep over an hour counts exactly what a full scan counts, every part
// kind included (goods, buildings, tame animals, carried gear), and the
// game's Wealth is that value.
func TestWealthTrackerMatchesFullScan(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 5, Width: 60, Height: 60})
	for x := 2; x < 12; x++ {
		g.BuildNow("wall", "wood", "south", x, 2)
	}
	if id, err := g.SpawnAnimal("cow", 30, 30); err == nil {
		g.animals.Ref(id).Tame = true
	}
	g.Step(300) // a colony at work: goods hauled
	tame := 0
	for _, id := range g.animalIDs {
		if a, ok := g.animals.Get(id); ok && a.Tame {
			tame++
		}
	}
	if len(g.itemIDs) == 0 || len(g.bldgIDs) == 0 || tame == 0 {
		t.Fatalf("a colony of %d stacks, %d buildings, %d tame animals: the fixture needs each", len(g.itemIDs), len(g.bldgIDs), tame)
	}
	want := fullWealth(g)
	if want <= 0 || g.Wealth() != want {
		t.Fatalf("before any pass: Wealth %d, full scan %d", g.Wealth(), want)
	}
	now := uint64(g.clock.Now())
	h0 := (now/wealth.Pass + 1) * wealth.Pass
	for tick := h0; tick < h0+wealth.Pass; tick += wealth.Stride {
		g.wealth.Step(tick)
		if tick == h0+wealth.Pass/2 && g.wealth.Export().Cursor >= len(g.itemIDs)+len(g.bldgIDs)+len(g.animalIDs) {
			t.Fatal("half an hour in, the sweep is already done: it is not spread over the pass")
		}
	}
	if got := g.wealth.Value(); got != want || g.Wealth() != want {
		t.Fatalf("the sweep counts %d (Wealth %d), the full scan %d", got, g.Wealth(), want)
	}
	if h := g.WealthHistory(); len(h) != 1 || h[0].Value != want {
		t.Fatalf("history %v, want the day's first pass", h)
	}
	if d := g.StoryDebug(); d.Wealth != want {
		t.Fatalf("the story panel shows wealth %d, want %d", d.Wealth, want)
	}
}

// TestWealthSaveMidSweepHash (EAI-014): a game saved halfway through a
// sweep loads into the same state hash, and both games end the pass on
// the same value and hash.
func TestWealthSaveMidSweepHash(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 9, Width: 50, Height: 50})
	now := uint64(g.clock.Now())
	stepWithWealth(g, int((now/wealth.Pass+1)*wealth.Pass-now)+wealth.Pass/2) // into the next hour's pass, halfway
	s := g.wealth.Export()
	if s == nil || s.Hour == 0 || s.Cursor == 0 {
		t.Fatalf("not mid-sweep: %+v", s)
	}
	h := saveLoad(t, g)
	if g.StateHash() != h.StateHash() {
		t.Fatal("the loaded game hashes differently")
	}
	if !reflect.DeepEqual(h.wealth.Export(), s) {
		t.Fatalf("loaded tracker %+v, saved %+v", h.wealth.Export(), s)
	}
	stepWithWealth(g, wealth.Pass)
	stepWithWealth(h, wealth.Pass)
	if g.wealth.Value() != h.wealth.Value() || g.StateHash() != h.StateHash() {
		t.Fatalf("after the pass: values %d and %d, hashes differ: %v", g.wealth.Value(), h.wealth.Value(), g.StateHash() != h.StateHash())
	}
	if !g.wealth.Export().Counted {
		t.Fatal("the pass never finished")
	}
}
