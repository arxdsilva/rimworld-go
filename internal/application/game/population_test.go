package game

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
)

// wildMapOf is a bare wild map (open ground, no colony) of biome b.
func wildMapOf(seed uint64, w, h int, b biome.ID) *Game {
	g := New(Config{Seed: seed, Width: w, Height: h, Biome: b})
	g.wild = true
	return g
}

// hoursFrom runs the hourly capacity check alone (no animal moves or
// dies) for n hours from day 3, calling after each.
func hoursFrom(g *Game, n int, after func(now uint64)) {
	for h := 1; h <= n; h++ {
		now := uint64(3*clock.TicksPerDay + h*clock.TicksPerHour)
		g.migrate(now)
		if after != nil {
			after(now)
		}
	}
}

func onEdge(g *Game, p Position, slack int) bool {
	return min(p.X, p.Y, g.grid.Width()-1-p.X, g.grid.Height()-1-p.Y) <= slack
}

func TestWildlifeCapacity(t *testing.T) {
	g := wildMapOf(1, 100, 100, biome.TemperateForest)
	summer := uint64(seasonDay(t, "Summer", 0)*clock.TicksPerDay + 12*clock.TicksPerHour)
	winter := uint64(seasonDay(t, "Winter", 0)*clock.TicksPerDay + 12*clock.TicksPerHour)
	g.clock = clock.NewAt(clock.Tick(summer))
	if c := g.wildlifeCap(); c != 100 {
		t.Fatalf("temperate summer capacity %d, want 100 per 10,000 tiles", c)
	}
	g.clock = clock.NewAt(clock.Tick(winter))
	if c := g.wildlifeCap(); c != 80 {
		t.Fatalf("temperate winter capacity %d, want 80%%", c)
	}
	b := wildMapOf(1, 100, 100, biome.BorealForest) // yearly mean 0 °C
	b.clock = clock.NewAt(clock.Tick(winter))
	if c := b.wildlifeCap(); c != 80*60/100 {
		t.Fatalf("boreal winter capacity %d, want 60%%", c)
	}
	// event animals, tame and leaving ones are not counted
	for _, s := range []string{"deer", "deer", "hive_beetle", "glimmerstag", "timbergnaw", "cow"} {
		_, _ = g.SpawnAnimal(s, 50, 50)
	}
	setAnimal(g, g.animalIDs[1], func(a *animal.State) { a.Leaving = true })
	setAnimal(g, g.animalIDs[5], func(a *animal.State) { a.Tame = true })
	if n := g.recountWild(uint64(g.clock.Now())); n != 1 {
		t.Fatalf("counted %d wild animals, want the one deer", n)
	}
}

func TestWildlifeRefillsBelowCap(t *testing.T) {
	g := wildMapOf(5, 60, 60, biome.TemperateForest)
	capacity := g.wildlifeCap()
	seen := map[entity.EntityID]bool{}
	arrivals := 0
	hoursFrom(g, 24*30, func(uint64) {
		fresh := false
		for _, id := range g.animalIDs {
			if seen[id] {
				continue
			}
			seen[id], fresh = true, true
			if p, _ := g.positions.Get(id); !onEdge(g, p, 2) {
				t.Fatalf("a newcomer at %v, not at an edge", p)
			}
		}
		if fresh {
			arrivals++
		}
		if n := g.wildCount(); n > capacity {
			t.Fatalf("refilled past capacity: %d of %d", n, capacity)
		}
	})
	if n := g.wildCount(); n*100 < capacity*capLowPct {
		t.Fatalf("after 30 days %d of %d (%d arrivals): the map refills to %d%%", n, capacity, arrivals, capLowPct)
	}
	if arrivals < 3 {
		t.Fatalf("%d arrivals: groups come one at a time", arrivals)
	}
}

func TestOverCapHerdLeaves(t *testing.T) {
	g := wildMapOf(2, 40, 40, biome.TemperateForest)
	capacity := g.wildlifeCap() // 16
	var herds [][]entity.EntityID
	for i := 0; i < 6; i++ {
		herds = append(herds, g.spawnHerd(animal.Deer, 8+i*5, 20, 4))
	}
	n := g.recountWild(uint64(g.clock.Now()))
	if n*100 <= capacity*capHighPct {
		t.Fatalf("fixture: %d of %d is not over capacity", n, capacity)
	}
	var leaving []entity.EntityID
	for h := 0; h < 24*5 && len(leaving) == 0; h++ {
		g.clock.Advance(clock.TicksPerHour - int(uint64(g.clock.Now())%clock.TicksPerHour))
		g.migrate(uint64(g.clock.Now()))
		for _, id := range g.animalIDs {
			if a, _ := g.animals.Get(id); a.Leaving {
				leaving = append(leaving, id)
			}
		}
	}
	newest := herds[len(herds)-1]
	if len(leaving) != len(newest) {
		t.Fatalf("leaving %v, want the newest herd %v", leaving, newest)
	}
	for i, id := range newest {
		if leaving[i] != id {
			t.Fatalf("leaving %v, want the newest herd %v", leaving, newest)
		}
	}
	if len(g.logs[newest[0]]) == 0 || g.logs[newest[0]][0].Kind != logLeaving {
		t.Fatal("the leaver's log says so")
	}
	for i := 0; i < 6*clock.TicksPerHour && len(g.wl.Departs) > 0; i++ {
		g.Step(1)
	}
	for _, id := range newest {
		if _, ok := g.animals.Get(id); ok {
			t.Fatalf("animal %d never walked off", id)
		}
	}
	if len(g.animalIDs) != 20 {
		t.Fatalf("%d animals left, want the 20 that stayed", len(g.animalIDs))
	}
	found := false
	for _, m := range g.messages {
		found = found || m.Text == "A group of 4 deer has left the area." && !m.Danger
	}
	if !found {
		t.Fatalf("no quiet leaving message in %+v", g.messages)
	}
}

// TestCapacityNoFlap: with nothing dying or breeding, a map just under the
// low threshold only ever gains (and stops at capacity), one just over the
// high threshold only ever loses (and stops above the low one): arrivals
// and departures never answer each other.
func TestCapacityNoFlap(t *testing.T) {
	for _, tc := range []struct {
		name string
		pct  int // starting count, percent of capacity
		gain bool
	}{{"below", capLowPct - 2, true}, {"above", capHighPct + 30, false}} {
		t.Run(tc.name, func(t *testing.T) {
			g := wildMapOf(9, 80, 80, biome.TemperateForest)
			capacity := g.wildlifeCap()
			for i := 0; i < capacity*tc.pct/100; i++ {
				g.spawnHerd(animal.Deer, 10+i%60, 10+i/60*3, 1)
			}
			prev, changes := g.recountWild(uint64(g.clock.Now())), 0
			hoursFrom(g, 24*30, func(uint64) {
				n := g.wildCount()
				switch {
				case n > prev && !tc.gain, n < prev && tc.gain:
					t.Fatalf("count went %d -> %d: flapped", prev, n)
				case n > capacity && prev <= capacity:
					t.Fatalf("arrivals overshot capacity: %d of %d", n, capacity)
				case n*100 < capacity*capLowPct && !tc.gain:
					t.Fatalf("departures undershot: %d of %d", n, capacity)
				}
				if n != prev {
					changes++
				}
				prev = n
			})
			if changes == 0 {
				t.Fatal("the count never moved")
			}
			if tc.gain && prev*100 < capacity*capLowPct || !tc.gain && prev*100 > capacity*capHighPct {
				t.Fatalf("settled at %d of %d", prev, capacity)
			}
		})
	}
}

func TestStartWildlifeNearTarget(t *testing.T) {
	for _, b := range []biome.ID{biome.TemperateForest, biome.Tundra, biome.TropicalRainforest, biome.Desert} {
		g := NewColony(Config{Seed: 4, Width: 100, Height: 100, Biome: b})
		target, n := g.startTarget(), g.wildCount()
		if n < target*3/4 || n > target+animal.RosterOf(animal.Deer).HerdMax*2 {
			t.Errorf("%s: %d wild animals at the start, target %d", biome.DefOf(b).Key, n, target)
		}
		hx, hy := g.home()
		for _, id := range g.animalIDs {
			a, _ := g.animals.Get(id)
			p, _ := g.positions.Get(id)
			if isPredator(a.Species) && chebyshev(p, Position{X: hx, Y: hy}) < wolfStartRadius {
				t.Errorf("%s: a %s starts %v, within %d of home", biome.DefOf(b).Key, a.Species, p, wolfStartRadius)
			}
		}
	}
}

// populationCurve runs a generated wild map of biome b (terrain, plants,
// grass and start wildlife; no colonists, whose own work dominates the
// cost of a big map) for days and returns the daily (count, capacity)
// pairs.
func populationCurve(b biome.ID, size, days int) [][2]int {
	g := New(Config{Seed: 21, Width: size, Height: size, Biome: b})
	g.generate(21)
	g.seedGrass()
	g.spawnWildlife()
	out := [][2]int{{g.wildCount(), g.wildlifeCap()}}
	for d := 0; d < days; d++ {
		g.Step(clock.TicksPerDay)
		out = append(out, [2]int{g.wildCount(), g.wildlifeCap()})
	}
	return out
}

// popTestSize is the side of a square map of biome b carrying about want
// wild animals, within 50..100 tiles.
func popTestSize(b biome.ID, want int) int {
	w := max(biome.DefOf(b).Wildlife, 1)
	return min(max(int(math.Sqrt(float64(want*10000/w))), 50), 100)
}

// TestPopulationStableTenDays: every biome but the ice ones keeps its wild
// population within half and one and a half times capacity for ten days
// (animals feeding, hunting, starving, resting, arriving, crossing and
// leaving). The default run sizes each biome's map for a capacity of
// about 30 (50..100 tiles a side, so every biome counts at least 10);
// RIMWORLD_POP_FULL=1 runs them all on 100x100 (the plan's measurement,
// ~20 s a biome); -short and -race keep three biomes at about 20.
func TestPopulationStableTenDays(t *testing.T) {
	biomes := []biome.ID{biome.TemperateForest, biome.BorealForest, biome.Tundra, biome.ColdBog, biome.TemperateSwamp,
		biome.TropicalSwamp, biome.TropicalRainforest, biome.AridShrubland, biome.Desert, biome.ExtremeDesert}
	want, full := 30, os.Getenv("RIMWORLD_POP_FULL") == "1"
	if !full && (testing.Short() || raceOn) {
		biomes, want = []biome.ID{biome.TemperateForest, biome.Tundra, biome.Desert}, 20
	}
	for _, b := range biomes {
		size := popTestSize(b, want)
		if full {
			size = 100
		}
		t.Run(biome.DefOf(b).Key, func(t *testing.T) {
			t.Parallel()
			curve := populationCurve(b, size, 10)
			var line strings.Builder
			for d, c := range curve {
				fmt.Fprintf(&line, " d%d %d/%d", d, c[0], c[1])
				if d > 0 && (c[0]*2 < c[1] || c[0]*2 > c[1]*3) {
					t.Errorf("day %d: %d wild of capacity %d", d, c[0], c[1])
				}
			}
			t.Logf("%s %dx%d:%s", biome.DefOf(b).Key, size, size, line.String())
		})
	}
}
