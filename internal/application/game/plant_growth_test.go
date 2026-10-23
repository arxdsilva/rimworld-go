package game

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/weather"
)

// TestFrostNoticesMergeWithinAnInterval: kills of one species in one
// climate interval read as one notice whose count adds up, whichever
// passes (staggered chunks) they came from.
func TestFrostNoticesMergeWithinAnInterval(t *testing.T) {
	g := New(Config{Seed: 1, Width: 40, Height: 16})
	bareGround(g)
	jumpTo(g, 45, 5) // midwinter night
	g.SetWeather(weather.Clear, 0)
	for x := 2; x < 4; x++ {
		_, _ = g.spawnPlant(plant.Corn, x, 3, 100)
	}
	g.frostCrops()
	n := len(g.messages)
	if m := g.messages[n-1].Text; m != "Frost killed the corn (2 plants)" {
		t.Fatalf("first pass: %q", m)
	}
	for x := 20; x < 23; x++ { // another chunk, the same interval
		_, _ = g.spawnPlant(plant.Corn, x, 3, 100)
	}
	g.frostCrops()
	if len(g.messages) != n || g.messages[n-1].Text != "Frost killed the corn (5 plants)" {
		t.Fatalf("second pass: %d messages, last %q", len(g.messages), g.messages[len(g.messages)-1].Text)
	}
	g.clock.Advance(climateInterval) // the next interval gets its own notice
	_, _ = g.spawnPlant(plant.Corn, 30, 3, 100)
	g.frostCrops()
	if len(g.messages) != n+1 || g.messages[n].Text != "Frost killed a corn plant" {
		t.Fatalf("next interval: %d messages, last %q", len(g.messages), g.messages[len(g.messages)-1].Text)
	}
	for _, s := range []plant.Species{plant.Corn, plant.JungleTree, plant.Pine} {
		for _, hot := range []bool{false, true} {
			for _, k := range []int{1, 2, 37} {
				if got, ok := frostCount(frostText(s, k, hot), s, hot); !ok || got != k {
					t.Fatalf("frostCount(frostText(%v, %d, %v)) = %d, %v", s, k, hot, got, ok)
				}
			}
		}
	}
	if _, ok := frostCount("Frost killed the corn (5 plants)", plant.Wheat, false); ok {
		t.Fatal("another species' notice matched")
	}
}

// TestStaggeredGrowthMatchesSpan: plants grow whichever chunk (phase)
// they stand in, and never more than GrowSpan at full rate over the
// windows they were grown for.
func TestStaggeredGrowthMatchesSpan(t *testing.T) {
	g := New(Config{Seed: 1, Width: 64, Height: 64})
	bareGround(g)
	g.SetWeather(weather.Clear, 0)
	jumpTo(g, 10, 6)                            // a summer morning
	a, _ := g.spawnPlant(plant.Tree, 1, 1, 0)   // chunk 0
	b, _ := g.spawnPlant(plant.Tree, 60, 60, 0) // the last chunk
	start := uint64(g.clock.Now())
	g.Step(4 * climateInterval)
	pa, _ := g.plants.Get(a)
	pb, _ := g.plants.Get(b)
	if pa.Growth <= 0 || pb.Growth <= 0 {
		t.Fatalf("no growth: %d, %d", pa.Growth, pb.Growth)
	}
	// four windows each, at most one rate per window: never more than
	// four windows of full-speed growth
	most := plant.GrowSpan(plant.Rate(plant.Tree, g.terr.At(1, 1).Fertility()), start-climateInterval, start+4*climateInterval)
	if pa.Growth > most || pb.Growth > most {
		t.Fatalf("growth %d, %d over %d ticks; at most %d", pa.Growth, pb.Growth, 4*climateInterval, most)
	}
	if uint64(clock.TicksPerHour) < 4*climateInterval {
		t.Fatal("test assumes the windows fall in daylight")
	}
}
