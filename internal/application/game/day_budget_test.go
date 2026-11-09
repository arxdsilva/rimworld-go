package game_test

import (
	"slices"
	"testing"
	"time"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/scenario"
	"rimworld-go/internal/domain/sim/clock"
)

// TestDayTickUnderBudget (GAR): with the periodic sites spread over
// their periods (game/cadence), the ticks around midnight on the grown
// 500² colony stay inside the 1.5 ms frame budget and no more than three
// times the median tick. Each of three midnights times twenty ticks
// across it; a midnight's worst is its slowest tick, and the test takes
// the best of the three (one stray stall is the machine, not the tick).
// Serial, and skipped under -short and -race (wall-clock timing).
func TestDayTickUnderBudget(t *testing.T) {
	if testing.Short() || game.RaceOn {
		t.Skip("wall-clock budget: not under -short or -race")
	}
	game.SetIndexSelfCheck(false)
	t.Cleanup(func() { game.SetIndexSelfCheck(true) })
	g, err := scenario.Load(scenario.GrownColony(benchSeed, 500, 500, false))
	if err != nil {
		t.Fatal(err)
	}
	day := int(g.Tick())/clock.TicksPerDay + 1
	var all []time.Duration
	worst := time.Duration(1 << 62)
	for i := range 3 {
		g.SkipTo(day+i, 23)
		g.Step(clock.TicksPerHour - 10)
		w := time.Duration(0)
		for range 20 {
			t0 := time.Now()
			g.Step(1)
			d := time.Since(t0)
			all = append(all, d)
			w = max(w, d)
		}
		worst = min(worst, w)
	}
	slices.Sort(all)
	median := all[len(all)/2]
	t.Logf("worst tick %v (best of 3 midnights), median %v", worst, median)
	if worst > 1500*time.Microsecond || worst > 3*median {
		t.Fatalf("midnight's worst tick %v: over 1.5 ms or 3x the median %v", worst, median)
	}
}
