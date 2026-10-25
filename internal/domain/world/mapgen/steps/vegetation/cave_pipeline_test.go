package vegetation_test

import (
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/mapgen/pipeline"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/terrain"
)

// TestCavePlantsOnGeneratedMaps: on real mountainous cave maps, duskmoss
// grows only on dark cave floor (rough stone or gravel), never on a
// feature, and every cave has some.
func TestCavePlantsOnGeneratedMaps(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 3; seed++ {
		b := biome.BorealForest
		s := site.Site{Biome: b, Hills: site.Mountainous, Caves: true, Climate: biome.DefOf(b).Climate}
		r, err := pipeline.Generate(mapgen.Config{Seed: seed, W: 160, H: 160, Site: s, Biome: biome.DefOf(b), Debug: true})
		if err != nil {
			t.Fatal(err)
		}
		f := r.Fields
		dusk, floor := 0, 0
		notSky := make([]bool, len(f.Cave))
		for i := range notSky {
			notSky[i] = r.Roof[i] != 0 || f.Rock[i]
		}
		sky := mapgen.ChebDepth(r.W, r.H, notSky)
		for i, cave := range f.Cave {
			t := r.Terrain.Cells[i]
			if cave && r.Roof[i] != 0 && sky[i] > 2 && (t == terrain.Stone || t == terrain.Gravel) && !f.Blocked[i] {
				floor++ // dark cave floor
			}
		}
		for _, p := range r.Plants {
			i := p.Y*r.W + p.X
			if f.Blocked[i] {
				t.Fatalf("seed %d: %s on a blocked cell", seed, p.Species)
			}
			if p.Species != plant.Duskmoss {
				continue
			}
			dusk++
			tt := r.Terrain.Cells[i]
			if !f.Cave[i] || r.Roof[i] == 0 || tt != terrain.Stone && tt != terrain.Gravel {
				t.Fatalf("seed %d: duskmoss on %s (cave %v, roof %d)", seed, tt, f.Cave[i], r.Roof[i])
			}
		}
		t.Logf("seed %d: %d duskmoss on %d dark cave floor", seed, dusk, floor)
		if floor > 200 && (dusk*100 < floor*12 || dusk*100 > floor*28) {
			t.Fatalf("seed %d: %d duskmoss on %d dark cave floor", seed, dusk, floor)
		}
	}
}
