package game_test

import (
	"runtime"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/scenario"
)

// Quiet-tick budgets for World() on the 200² colony below, view cache on.
// Measured 2026-10-27 (PERF-002): 157 allocs (178 under -race) and ~192 KB
// a tick. Nearly all of it is the wild animals' list (animalViews: a fresh
// []AnimalView every tick, ~175 KB and ~70 allocs with trainViews, as the
// animals move), then pen labels and the one-per-tick small lists
// (rooms, smoothing marks, ...). Before PERF-002 the same read rebuilt
// every item, frame, building and plant view: ~3,400 allocs here. The
// budgets leave headroom over the measurement, not room for a list to go
// back to being rebuilt every tick (that alone is hundreds of KB).
const (
	quietWorldAllocs = 250
	quietWorldBytes  = 256 << 10
)

// TestWorldViewQuietTickAllocs: on a quiet night tick (no plant growing)
// of a 200² colony, World() with the view cache on (the renderer's
// setting) allocates little, and the plant, building, item and zone lists
// keep their slices (PERF-002: the renderer's identity memos hold).
func TestWorldViewQuietTickAllocs(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 200² colony")
	}
	game.SetIndexSelfCheck(false) // the self-check's scans allocate; this measures the read model
	t.Cleanup(func() { game.SetIndexSelfCheck(true) })
	g, _, err := scenario.Build(benchSeed, 200, 200)
	if err != nil {
		t.Fatal(err)
	}
	g.SkipTo(2, 22) // night: the plants rest
	g.SetViewCache(true)
	g.Step(60)
	_ = g.World()
	const runs = 20
	var allocs, bytes uint64
	var m0, m1 runtime.MemStats
	prev := g.World()
	kept := map[string]int{}
	for range runs {
		g.Step(1)
		runtime.ReadMemStats(&m0)
		w := g.World()
		runtime.ReadMemStats(&m1)
		if len(w.Plants) == 0 || len(w.Buildings) == 0 {
			t.Fatal("empty read model")
		}
		allocs += m1.Mallocs - m0.Mallocs
		bytes += m1.TotalAlloc - m0.TotalAlloc
		for name, same := range map[string]bool{
			"Plants": &w.Plants[0] == &prev.Plants[0], "Buildings": &w.Buildings[0] == &prev.Buildings[0],
			"Items": &w.Items[0] == &prev.Items[0], "Zones": &w.Zones[0] == &prev.Zones[0],
		} {
			if same {
				kept[name]++
			}
		}
		prev = w
	}
	stepWorld := testing.AllocsPerRun(runs, func() {
		g.Step(1)
		_ = g.World()
	})
	t.Logf("World(): %d allocs, %d B per tick; Step+World: %.0f allocs per tick; lists kept (of %d ticks): %v",
		allocs/runs, bytes/runs, stepWorld, runs, kept)
	if stepWorld > quietWorldAllocs {
		t.Errorf("Step+World: %.0f allocs a quiet tick, budget %d", stepWorld, quietWorldAllocs)
	}
	if b := bytes / runs; b > quietWorldBytes {
		t.Errorf("World(): %d B a quiet tick, budget %d", b, quietWorldBytes)
	}
	for _, name := range []string{"Plants", "Buildings", "Items", "Zones"} {
		if kept[name] != runs {
			t.Errorf("%s kept its slice on %d of %d quiet ticks", name, kept[name], runs)
		}
	}
}
