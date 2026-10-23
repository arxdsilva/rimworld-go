package generation

import (
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/site"
)

// TestGroundPlantsSpawn: every biome grows its PLAY-010 ground plants
// (the temperate forest on landing sites only: its legacy map is frozen),
// on fertile ground, at about their permille of fertile tiles.
func TestGroundPlantsSpawn(t *testing.T) {
	t.Parallel()
	s := site.Site{Stones: []geology.Stone{geology.Granite}}
	for _, d := range biome.All() {
		res := GenerateSite(5, 120, 120, d, s)
		got := map[plant.Species]int{}
		for _, p := range res.Plants {
			got[p.Species]++
			if res.Terrain.At(p.X, p.Y).Fertility() == 0 {
				t.Fatalf("%s: %s on barren ground", d.Key, p.Species)
			}
		}
		fertile := 0
		for _, c := range res.Terrain.Cells {
			if c.Fertility() > 0 {
				fertile++
			}
		}
		for _, pd := range d.Plants {
			if !plant.DefOf(pd.Species).Ground {
				continue
			}
			pm := got[pd.Species] * 1000 / max(fertile, 1)
			if got[pd.Species] == 0 || pm < pd.Permil/2 || pm > pd.Permil*2 {
				t.Errorf("%s: %s %d permille (%d plants), want ~%d", d.Key, pd.Species, pm, got[pd.Species], pd.Permil)
			}
		}
	}
	// the legacy temperate map has none
	for _, p := range GenerateBiome(5, 120, 120, biome.DefOf(biome.TemperateForest)).Plants {
		if plant.DefOf(p.Species).Ground {
			t.Fatalf("legacy temperate map grew %s", p.Species)
		}
	}
}
