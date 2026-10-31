package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/world/temperature"
)

// TestBenchSpeedFollowsRoomTemperature (TEMP-0): through the real craft
// host (tenths of a degree), a bench in a 21 C room works at full speed and
// the same bench in a 5 C room at the wiki's 70 %. The bench once compared
// tenths against whole degrees, so every bench in a warm room ran at 70 %.
func TestBenchSpeedFollowsRoomTemperature(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	hut(t, g, 1, 1, 7, 7)
	if !g.BuildNow("stove", "wood", "south", 4, 3) {
		t.Fatal("build stove")
	}
	r := g.roomAt(4, 4)
	if r < 0 {
		t.Fatal("no room around the stove")
	}
	id := g.layerBldgAt(4, 3, construction.LayerObject)
	b, _ := g.bldgs.Get(id)
	speed := func(air temperature.Temp) int {
		g.thermo.Temps[r] = air
		g.structVer++ // drop the bench's kept speed
		_, v, _ := g.craft.Stations.View(id, b, "")
		return v
	}
	warm, cold := speed(temperature.C(21)), speed(temperature.C(5))
	if warm == 0 || warm*700 != cold*1000 {
		t.Fatalf("bench speed %d at 21 C and %d at 5 C, want the cold one at 70 %% of the warm one", warm, cold)
	}
}
