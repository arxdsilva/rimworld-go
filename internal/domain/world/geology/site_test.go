package geology_test

import (
	"fmt"
	"hash/fnv"
	"slices"
	"testing"

	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/terrain"
)

// goldenGeology pins Generate before landing sites (and machinery)
// existed. Recompute only for a deliberate change to legacy geology.
var goldenGeology = map[string]uint64{
	"generated 1/1":  0x8d9455807ca6cd17,
	"generated 77/9": 0xe2a3ccc8220d9dde,
	"hand-made/1":    0x687e431f252dcbfc,
	"hand-made/9":    0xaf22fd43530e5c34,
}

func geoHash(g geology.Map) uint64 {
	h := fnv.New64a()
	for i := range g.Stone {
		h.Write([]byte{byte(g.Stone[i]), byte(g.Ore[i])})
	}
	return h.Sum64()
}

func TestLegacyGeologyUnchanged(t *testing.T) {
	t.Parallel()
	for name, tm := range maps() {
		for _, seed := range []uint64{1, 9} {
			key := fmt.Sprintf("%s/%d", name, seed)
			if want, ok := goldenGeology[key]; ok {
				if got := geoHash(geology.Generate(seed, tm)); got != want {
					t.Errorf("%s: geology hash %#x, want %#x", key, got, want)
				}
			}
		}
	}
}

func TestGenerateWithoutStonesIsGenerate(t *testing.T) {
	t.Parallel()
	for name, tm := range maps() {
		for _, stones := range [][]geology.Stone{nil, {}, {geology.None}, {geology.Stone(77)}} {
			a, b := geology.GenerateWith(4, tm, stones), geology.Generate(4, tm)
			if !slices.Equal(a.Stone, b.Stone) || !slices.Equal(a.Ore, b.Ore) {
				t.Fatalf("%s %v: no valid stones must be Generate", name, stones)
			}
		}
	}
	if g := geology.GenerateWith(1, terrain.NewMap(0, 0), []geology.Stone{geology.Slate}); g.W != 0 || len(g.Stone) != 0 {
		t.Fatalf("empty map: %+v", g)
	}
}

// hugeRock is a 120×120 rock mass with one open tile: ~14 000 deep cells.
func hugeRock() terrain.Map {
	m := terrain.NewMap(120, 120)
	for i := range m.Cells {
		m.Cells[i] = terrain.Rock
	}
	m.Set(60, 60, terrain.Soil)
	return m
}

func TestGenerateWithUsesTheSiteStones(t *testing.T) {
	t.Parallel()
	tm := hugeRock()
	for _, want := range [][]geology.Stone{{geology.Marble, geology.Slate}, {geology.Granite, geology.Limestone, geology.Sandstone}} {
		for seed := uint64(0); seed < 6; seed++ {
			seen := map[geology.Stone]bool{}
			stones := append(slices.Clone(want), want[0], geology.None) // repeats and None are dropped
			for _, s := range geology.GenerateWith(seed, tm, stones).Stone {
				if s == geology.None {
					continue // the open tile
				}
				if !slices.Contains(want, s) {
					t.Fatalf("seed %d: stone %v not among the site's %v", seed, s, want)
				}
				seen[s] = true
			}
			if len(seen) < 2 {
				t.Fatalf("seed %d: only %v of the site's %v on a huge rock mass", seed, seen, want)
			}
		}
	}
}

func TestMachineryOnlyOnSiteMaps(t *testing.T) {
	t.Parallel()
	tm := hugeRock()
	deep := len(geology.DeepCells(tm, geology.Depths(tm)))
	for seed := uint64(0); seed < 10; seed++ {
		if slices.Contains(geology.Generate(seed, tm).Ore, geology.Machinery) {
			t.Fatalf("seed %d: legacy geology grew machinery", seed)
		}
		g := geology.GenerateWith(seed, tm, []geology.Stone{geology.Granite, geology.Slate})
		n := 0
		for i, o := range g.Ore {
			if o == geology.Machinery {
				n++
				if tm.Cells[i] != terrain.Rock {
					t.Fatalf("seed %d: machinery outside rock", seed)
				}
			}
		}
		veins := geology.VeinCount(deep, geology.Machinery)
		if veins == 0 || n == 0 || n > veins*geology.VeinMax(geology.Machinery) {
			t.Fatalf("seed %d: %d machinery tiles from %d veins", seed, n, veins)
		}
		for _, o := range []geology.Ore{geology.Steel, geology.Silver, geology.Gold} {
			if !slices.Contains(g.Ore, o) {
				t.Fatalf("seed %d: site geology lost %v", seed, o)
			}
		}
	}
	for _, c := range []struct{ deep, want int }{{2499, 0}, {2500, 1}, {7600, 3}} {
		if got := geology.VeinCount(c.deep, geology.Machinery); got != c.want {
			t.Errorf("machinery veins at %d deep: %d, want %d", c.deep, got, c.want)
		}
	}
	if geology.VeinMax(geology.Machinery) != 3 {
		t.Fatalf("machinery vein max %d", geology.VeinMax(geology.Machinery))
	}
	if geology.Machinery.String() != "machinery" || geology.Machinery.Label() != "Buried machinery" {
		t.Fatal("machinery names")
	}
}
