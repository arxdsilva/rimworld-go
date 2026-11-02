package pipeline

import (
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

// WG-08: a colony on a huge river generates the whole map without error.
func TestHugeRiverColonyGenerates(t *testing.T) {
	s := site.Site{Biome: biome.TemperateForest, Hills: site.Flat, Climate: biome.DefOf(biome.TemperateForest).Climate,
		River: site.RiverPath{Size: site.HugeRiver, In: 0, Out: 4}}
	r, err := Generate(mapgen.Config{Seed: 77, W: 120, H: 120, Site: s, Biome: biome.DefOf(s.Biome)})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Terrain.Cells) != 120*120 {
		t.Fatalf("%d cells", len(r.Terrain.Cells))
	}
}
