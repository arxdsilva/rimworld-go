package landform

import (
	"math"
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

// newCtx is a fixture context for a w×h map of site s.
func newCtx(w, h int, seed uint64, s site.Site) *mapgen.Ctx {
	return mapgen.NewCtx(mapgen.Config{Seed: seed, W: w, H: h, Site: s, Biome: biome.DefOf(s.Biome)})
}

// runSteps runs steps in order on c.
func runSteps(t testing.TB, c *mapgen.Ctx, steps ...mapgen.Step) {
	t.Helper()
	for _, s := range steps {
		if err := s.Run(c); err != nil {
			t.Fatalf("%s: %v", s.Name(), err)
		}
	}
}

// all is the four landform steps in pipeline order.
func all() []mapgen.Step {
	return []mapgen.Step{Elevation{}, RockMask{}, Caves{}, Rocks{}}
}

func rockShare(c *mapgen.Ctx) float64 {
	n := 0
	for _, r := range c.F.Rock {
		if r {
			n++
		}
	}
	return float64(n) / float64(len(c.F.Rock))
}

func TestStepNames(t *testing.T) {
	want := []string{"elevation", "rockmask", "caves", "rocks"}
	for i, s := range all() {
		if s.Name() != want[i] {
			t.Fatalf("step %d named %q, want %q", i, s.Name(), want[i])
		}
	}
}

// The whole landform chain is a pure function of the seed.
func TestLandformDeterministic(t *testing.T) {
	s := site.Site{Hills: site.Mountainous, Caves: true}
	a, b, o := newCtx(120, 90, 7, s), newCtx(120, 90, 7, s), newCtx(120, 90, 8, s)
	runSteps(t, a, all()...)
	runSteps(t, b, all()...)
	runSteps(t, o, all()...)
	same, diff := true, false
	for i := range a.F.Rock {
		if a.F.Rock[i] != b.F.Rock[i] || a.F.Cave[i] != b.F.Cave[i] || a.R.Roof[i] != b.R.Roof[i] ||
			a.F.Elevation[i] != b.F.Elevation[i] || a.F.RockDepth[i] != b.F.RockDepth[i] {
			same = false
		}
		if a.F.Rock[i] != o.F.Rock[i] || a.F.Elevation[i] != o.F.Elevation[i] {
			diff = true
		}
	}
	if !same {
		t.Fatal("same seed gave different landforms")
	}
	if !diff {
		t.Fatal("another seed gave the same landforms")
	}
	if a.F.RockTau != b.F.RockTau || a.F.RoughTau != b.F.RoughTau || a.F.GravelTau != b.F.GravelTau {
		t.Fatal("same seed gave different thresholds")
	}
}

// Tiny, thin and single-cell maps survive every hilliness with caves on.
func TestLandformTinyMaps(t *testing.T) {
	sizes := [][2]int{{24, 24}, {1, 1}, {1, 7}, {50, 3}, {2, 2}}
	for _, sz := range sizes {
		for h := site.HillsDefault; h <= site.Mountainous; h++ {
			for _, b := range []biome.ID{biome.TemperateForest, biome.SeaIce} {
				c := newCtx(sz[0], sz[1], 3, site.Site{Hills: h, Caves: true, Biome: b})
				runSteps(t, c, all()...)
				for i, e := range c.F.Elevation {
					if e < 0 || e > 1 || math.IsNaN(float64(e)) {
						t.Fatalf("%v hills %d: elevation[%d] = %v", sz, h, i, e)
					}
				}
			}
		}
	}
}

// BenchmarkLandform runs elevation, rockmask, caves and rocks at 500×500
// (budget: 400 ms together).
func BenchmarkLandform(b *testing.B) {
	s := site.Site{Hills: site.Mountainous, Caves: true, Uphill: 2, HasUphill: true}
	for b.Loop() {
		c := newCtx(500, 500, 11, s)
		runSteps(b, c, all()...)
	}
}
