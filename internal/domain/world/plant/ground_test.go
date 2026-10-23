package plant

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

// TestGroundPlantDefs: the PLAY-010 wild ground plants are wild, low,
// woodless and yield nothing, with our own numbers.
func TestGroundPlantDefs(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		s                  Species
		key                string
		grow, cut, walk    int
		browse             bool
		coldKill, heatKill int
	}{
		{Bramble, "bramble", 40000, 30, 150, true, -300, 450},
		{Moss, "moss", 30000, 10, 0, true, -500, 350},
		{Agave, "agave", 50000, 25, 0, false, -50, 650},
		{BallCactus, "ball_cactus", 60000, 20, 0, false, -80, 700},
		{ElephantEar, "elephant_ear", 30000, 15, 110, true, 50, 500},
		{FireLily, "fire_lily", 30000, 10, 0, true, 0, 450},
		{Dandelion, "dandelion", 15000, 5, 0, true, -200, 400},
	} {
		d := DefOf(c.s)
		if d.Species != c.s || c.s.String() != c.key {
			t.Errorf("%v: key %q, want %q", c.s, c.s.String(), c.key)
		}
		if s, ok := Parse(c.key); !ok || s != c.s {
			t.Errorf("parse %q", c.key)
		}
		if d.GrowTicks != c.grow || d.CutWork != c.cut || d.WalkCost != c.walk || d.Browse != c.browse {
			t.Errorf("%s: %+v", c.key, d)
		}
		if d.ColdKill != c.coldKill || d.HeatKill != c.heatKill {
			t.Errorf("%s: kills %d/%d, want %d/%d", c.key, d.ColdKill, d.HeatKill, c.coldKill, c.heatKill)
		}
		if !d.Ground || d.Crop || d.Sowable || d.ZoneSowable() || d.Trunk || d.Wood != 0 || d.Yield != 0 || d.Regrows {
			t.Errorf("%s: must be a wild, woodless, yieldless ground plant: %+v", c.key, d)
		}
	}
	for s := Species(0); s < numSpecies; s++ {
		if d := DefOf(s); d.Ground && (d.Trunk || d.Crop) {
			t.Errorf("%s: a ground plant is never a tree or crop", s)
		}
	}
}

// TestGrassFillsInThreeDays: shoots (GrassShoots) fill out to full in
// GrassFillTicks of mild weather, the same however the time is split.
func TestGrassFillsInThreeDays(t *testing.T) {
	t.Parallel()
	if GrassShoots != 15 || GrassFillTicks != 3*clock.TicksPerDay {
		t.Fatalf("shoots %d, fill %d ticks", GrassShoots, GrassFillTicks)
	}
	mild := TempFactor(150)
	if mild != 1000 {
		t.Fatalf("15 C factor %d", mild)
	}
	const step = 250
	g, now := GrassShoots, uint64(1234567)
	ticks := uint64(0)
	for g < GrassFull {
		g += GrassGrowSpan(mild, now, now+step)
		now += step
		ticks += step
	}
	lo, hi := uint64(GrassFillTicks)*9/10, uint64(GrassFillTicks)*11/10
	if ticks < lo || ticks > hi {
		t.Fatalf("shoots to full took %d ticks, want %d +-10%%", ticks, GrassFillTicks)
	}
	// spans telescope
	if a, b := GrassGrowSpan(mild, 100, 600), GrassGrowSpan(mild, 100, 350)+GrassGrowSpan(mild, 350, 600); a != b {
		t.Fatalf("span %d vs split %d", a, b)
	}
	if GrassGrowSpan(0, 0, 1_000_000) != 0 || GrassGrowSpan(TempFactor(-10), 0, 1_000_000) != 0 {
		t.Fatal("frozen grass does not grow")
	}
	if GrassGrowSpan(mild, 500, 500) != 0 || GrassGrowSpan(mild, 600, 500) != 0 {
		t.Fatal("an empty span grows nothing")
	}
}

// TestGrazeFood: a meal scales with growth, never below GrassMinMeal.
func TestGrazeFood(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ growth, want int }{{100, 1500}, {60, 900}, {30, 450}, {15, 450}, {0, 450}} {
		if got := GrassMeal(1500, c.growth); got != c.want {
			t.Errorf("GrassMeal(1500, %d) = %d, want %d", c.growth, got, c.want)
		}
	}
}
