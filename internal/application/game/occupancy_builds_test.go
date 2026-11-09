package game

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// TestOccupancyRebuildOncePerTick: however many movers, hooks and
// repaths ask for the occupancy index in a tick, each open map rebuilds
// it at most once (on first use, or the periodic stack sweep), and a
// busy colony does rebuild it.
func TestOccupancyRebuildOncePerTick(t *testing.T) {
	t.Parallel()
	raid := NewColony(Config{Seed: 7, Width: 48, Height: 40})
	raid.Raid(4)
	two := worldGame(t)
	tile, ada := plainNear(t, two), two.pawnIDs[0]
	two.Enqueue(command.Command{Kind: command.EnterMap, Target: uint64(arriveAt(t, two, tile, ada))})
	two.Step(1)
	if len(two.maps) != 2 {
		t.Fatal("no second map")
	}
	for name, g := range map[string]*Game{"raid": raid, "two maps": two} {
		total := uint64(0)
		for range clock.TicksPerHour {
			before := make([]uint64, len(g.maps))
			for i, m := range g.maps {
				before[i] = m.occ.builds
			}
			g.Step(1)
			for i, m := range g.maps[:min(len(g.maps), len(before))] {
				if d := m.occ.builds - before[i]; d > 1 {
					t.Fatalf("%s: map %d rebuilt its occupancy %d times on tick %d", name, m.id, d, g.clock.Now())
				} else {
					total += d
				}
			}
		}
		if total == 0 {
			t.Fatalf("%s: never rebuilt: the test proves nothing", name)
		}
	}
}
