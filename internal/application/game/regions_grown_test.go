package game_test

import (
	"os"
	"testing"

	"rimworld-go/internal/application/scenario"
)

// TestRegionsGrownSameState: the grown 120² colony (hundreds of buildings,
// doors, a fenced pen) under a raid runs bit-identically with region
// answers and with plain searches.
func TestRegionsGrownSameState(t *testing.T) {
	if testing.Short() || os.Getenv("RIMWORLD_FIXTURE_DIR") == "" {
		t.Skip("needs the cached grown fixture (RIMWORLD_FIXTURE_DIR): building it takes minutes")
	}
	data := scenario.GrownColony(benchSeed, 120, 120, false)
	on, err := scenario.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	off, err := scenario.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	on.SetExactLongPaths(true) // corridors change long paths on purpose
	off.SetRegionsOff(true)
	for done := 0; done < 4000; done += 1000 {
		if done == 1000 {
			on.Raid(8)
			off.Raid(8)
		}
		on.Step(1000)
		off.Step(1000)
		if a, b := on.StateHash(), off.StateHash(); a != b {
			t.Fatalf("diverged by tick %d: %#x vs %#x", done+1000, a, b)
		}
	}
}
