package vegetation

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/noise2"
)

// caveFixture is biome b's fixture with a mountain on its right two
// thirds: rock with a noise-carved cave system (rough stone, gravel
// patches, a few puddles, a blocked vault-like square), its mouths open
// to the soil on the left. Roofs follow the rocks step: Natural from
// depth 2 over rock and cave together.
func caveFixture(b biome.ID, seed uint64) *mapgen.Ctx {
	const w, h = 150, 110
	c := fixture(b, seed, w, h)
	c.R.Start = mapgen.Point{X: 10, Y: 10}
	cs, gs := noise2.Seed(seed, "fixture/cave"), noise2.Seed(seed, "fixture/gravel")
	solid := make([]bool, w*h)
	for i := range c.R.Terrain.Cells {
		x, y := c.XY(i)
		if x < 50 {
			continue
		}
		solid[i] = true
		v := noise2.FBm(cs, float64(x), float64(y), 3, 14, 0.5)
		switch {
		case v > 0.55:
			c.F.Cave[i] = true
			c.R.Terrain.Cells[i] = terrain.Stone
			if noise2.FBm(gs, float64(x), float64(y), 2, 9, 0.5) > 0.6 {
				c.R.Terrain.Cells[i] = terrain.Gravel
			}
			if (x+y)%53 == 0 {
				c.R.Terrain.Cells[i] = terrain.ShallowWater // a cave puddle
			}
		default:
			c.F.Rock[i] = true
			c.R.Terrain.Cells[i] = terrain.Rock
		}
		if x >= 100 && x < 110 && y >= 40 && y < 50 { // a vault footprint
			c.F.Blocked[i] = true
		}
		if x < 54 && c.F.Cave[i] { // the cave runs out to the open ground
			c.F.Rock[i] = false
		}
	}
	// open the mountain's left face into the caves that reach it
	for y := range h {
		for x := 50; x < 53; x++ {
			i := c.Idx(x, y)
			if y%9 < 4 {
				c.F.Rock[i], c.F.Cave[i] = false, true
				c.R.Terrain.Cells[i] = terrain.Stone
			}
		}
	}
	for i := range solid {
		solid[i] = c.F.Rock[i] || c.F.Cave[i]
	}
	d := mapgen.ChebDepth(w, h, solid)
	for i, s := range solid {
		if s && d[i] >= 2 {
			c.R.Roof[i] = roof.Natural
		}
	}
	return c
}

// skyDist is the test's own distance (Chebyshev) to the nearest open cell
// under the sky.
func skyDist(c *mapgen.Ctx) []uint8 {
	notSky := make([]bool, c.W*c.H)
	for i := range notSky {
		notSky[i] = c.R.Roof[i] != 0 || c.F.Rock[i]
	}
	return mapgen.ChebDepth(c.W, c.H, notSky)
}

func caveFloor(c *mapgen.Ctx, i int) bool {
	t := c.R.Terrain.Cells[i]
	return c.F.Cave[i] && (t == terrain.Stone || t == terrain.Gravel) && !c.F.Blocked[i] && !c.F.Road[i] &&
		!c.F.Water[i].Wet()
}

// darkFloor is dark cave floor: roofed and more than 2 from open sky.
func darkFloor(c *mapgen.Ctx, sky []uint8, i int) bool {
	return caveFloor(c, i) && c.R.Roof[i] != 0 && sky[i] > caveMouth
}

func TestCaveDuskmossOnDarkFloorInClusters(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 4; seed++ {
		c := caveFixture(biome.BorealForest, seed)
		run(t, c)
		sky := skyDist(c)
		dm := make([]bool, c.W*c.H)
		seen := map[int]bool{}
		n, dark := 0, 0
		for _, p := range c.R.Plants {
			i := c.Idx(p.X, p.Y)
			if seen[i] {
				t.Fatalf("two plants on %d,%d", p.X, p.Y)
			}
			seen[i] = true
			if c.F.Blocked[i] {
				t.Fatalf("%s on a blocked cell", p.Species)
			}
			if p.Species != plant.Duskmoss {
				continue
			}
			if !darkFloor(c, sky, i) {
				t.Fatalf("seed %d: duskmoss on %d,%d (%s, roof %d, sky %d)", seed, p.X, p.Y,
					c.R.Terrain.Cells[i], c.R.Roof[i], sky[i])
			}
			gt := plant.DefOf(plant.Duskmoss).GrowTicks
			if p.Growth < gt*30/100 || p.Growth > gt {
				t.Fatalf("duskmoss growth %d of %d", p.Growth, gt)
			}
			dm[i] = true
			n++
		}
		for i := range dm {
			if darkFloor(c, sky, i) {
				dark++
			}
		}
		if dark < 500 {
			t.Fatalf("seed %d: fixture has only %d dark floor cells", seed, dark)
		}
		if cov := float64(n) / float64(dark); cov < 0.13 || cov > 0.27 {
			t.Fatalf("seed %d: duskmoss covers %.3f of %d dark floor cells", seed, cov, dark)
		}
		// clusters: 8-connected groups (so groups never touch) of 1–4
		// cells, each one 4-connected, mostly 2–3
		sizes := clusters(c, dm)
		mid := 0
		for _, cl := range sizes {
			if len(cl) < 1 || len(cl) > 4 {
				t.Fatalf("seed %d: a cluster of %d", seed, len(cl))
			}
			if !connected4(c, cl) {
				t.Fatalf("seed %d: cluster %v not edge-connected", seed, cl)
			}
			if len(cl) == 2 || len(cl) == 3 {
				mid++
			}
		}
		t.Logf("seed %d: %d dark floor, duskmoss %.3f in %d clusters (%d of 2–3)", seed, dark, float64(n)/float64(dark), len(sizes), mid)
		if mid*2 < len(sizes) {
			t.Fatalf("seed %d: only %d of %d clusters have 2–3 cells", seed, mid, len(sizes))
		}
	}
}

// clusters are the 8-connected components of the marked cells.
func clusters(c *mapgen.Ctx, on []bool) [][]int {
	seen := make([]bool, len(on))
	var out [][]int
	for s := range on {
		if !on[s] || seen[s] {
			continue
		}
		cl := []int{s}
		seen[s] = true
		for k := 0; k < len(cl); k++ {
			x, y := c.XY(cl[k])
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if c.In(x+dx, y+dy) {
						if j := c.Idx(x+dx, y+dy); on[j] && !seen[j] {
							seen[j] = true
							cl = append(cl, j)
						}
					}
				}
			}
		}
		out = append(out, cl)
	}
	return out
}

func connected4(c *mapgen.Ctx, cl []int) bool {
	reach := []int{cl[0]}
	for k := 0; k < len(reach); k++ {
		x, y := c.XY(reach[k])
		for _, j := range cl {
			jx, jy := c.XY(j)
			if abs(jx-x)+abs(jy-y) == 1 && !slices.Contains(reach, j) {
				reach = append(reach, j)
			}
		}
	}
	return len(reach) == len(cl)
}

// TestCaveMouthMoss: ordinary moss grows only on lit cave floor (open sky
// or within 2 of it), and only where the biome lists moss.
func TestCaveMouthMoss(t *testing.T) {
	t.Parallel()
	c := caveFixture(biome.BorealForest, 2)
	run(t, c)
	sky := skyDist(c)
	moss := 0
	for _, p := range c.R.Plants {
		i := c.Idx(p.X, p.Y)
		if !c.F.Cave[i] {
			continue
		}
		switch p.Species {
		case plant.Duskmoss:
		case plant.Moss:
			if !caveFloor(c, i) || sky[i] > caveMouth {
				t.Fatalf("moss deep in the cave at %d,%d (sky %d)", p.X, p.Y, sky[i])
			}
			moss++
		default:
			t.Fatalf("%s in a cave", p.Species)
		}
	}
	if moss == 0 {
		t.Fatal("no moss at the cave mouths")
	}
	c = caveFixture(biome.TemperateForest, 2) // no moss in its plant list
	run(t, c)
	for _, p := range c.R.Plants {
		if p.Species == plant.Moss {
			t.Fatal("moss in a biome without it")
		}
	}
}

// TestCaveDeterministicAndSurfaceUnchanged: same seed, same caves; the
// cave pass draws its own stream, so the surface plants are exactly the
// ones the map gets without caves.
func TestCaveDeterministicAndSurfaceUnchanged(t *testing.T) {
	t.Parallel()
	a, b := caveFixture(biome.BorealForest, 5), caveFixture(biome.BorealForest, 5)
	run(t, a)
	run(t, b)
	if !slices.Equal(a.R.Plants, b.R.Plants) {
		t.Fatal("same seed differs")
	}
	o := caveFixture(biome.BorealForest, 5)
	for i := range o.F.Cave {
		if o.F.Cave[i] { // the same map with its caves taken by features
			o.F.Blocked[i] = true
		}
	}
	run(t, o)
	var surface []mapgen.PlantSpawn
	for _, p := range a.R.Plants {
		if !a.F.Cave[a.Idx(p.X, p.Y)] {
			surface = append(surface, p)
		}
	}
	if !slices.Equal(surface, o.R.Plants) {
		t.Fatalf("caves moved the surface plants: %d vs %d", len(surface), len(o.R.Plants))
	}
}

// TestCaveBareBiome: a biome with no plants keeps its caves bare.
func TestCaveBareBiome(t *testing.T) {
	t.Parallel()
	c := caveFixture(biome.BorealForest, 3)
	c.Cfg.Biome = biome.Def{ID: biome.NumBiomes + 3}
	run(t, c)
	if len(c.R.Plants) != 0 {
		t.Fatalf("%d plants without a plant list", len(c.R.Plants))
	}
}
