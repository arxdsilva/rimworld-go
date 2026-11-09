package game_test

import (
	"os"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/scenario"
)

// TestRaidIndexesMatchScansOnFixture (EAI-021, EAI-023): on a grown colony
// under a raid, the raiders' indexed look-ups pick what scans of the whole
// lists pick, from every raider's tile, as the raid goes on (as
// TestRegionsGrownSameState, with the cached fixture only).
func TestRaidIndexesMatchScansOnFixture(t *testing.T) {
	t.Parallel()
	if testing.Short() || os.Getenv("RIMWORLD_FIXTURE_DIR") == "" {
		t.Skip("needs the cached grown fixture (RIMWORLD_FIXTURE_DIR): building it takes minutes; raid_eai_test.go checks the same on built maps")
	}
	g, err := scenario.Load(scenario.GrownColony(benchSeed, 120, 120, true))
	if err != nil {
		t.Fatal(err)
	}
	if standing, _ := g.Raiders(); standing == 0 {
		t.Fatal("the fixture has no raid")
	}
	for i := 0; i < 10; i++ {
		if bad := game.RaidIndexMismatch(g); bad != "" {
			t.Fatalf("step %d: %s", i, bad)
		}
		g.Step(30)
	}
}
