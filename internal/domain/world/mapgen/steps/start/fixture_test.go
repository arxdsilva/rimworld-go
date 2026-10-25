package start

import (
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/noise2"
)

// fixture is an all-soil w×h map of granite with mid fertility.
func fixture(seed uint64, w, h int) *mapgen.Ctx {
	s := site.Site{Biome: biome.TemperateForest, Hills: site.LargeHills, Stones: []geology.Stone{geology.Granite}}
	c := mapgen.NewCtx(mapgen.Config{Seed: seed, W: w, H: h, Site: s, Biome: biome.DefOf(s.Biome)})
	for i := range c.F.Fertility {
		c.F.Fertility[i] = 0.5
		c.F.Stone[i] = geology.Granite
	}
	return c
}

// setRock makes (x, y) a consistent rock-wall cell.
func setRock(c *mapgen.Ctx, x, y int) {
	i := c.Idx(x, y)
	c.R.Terrain.Cells[i] = terrain.Rock
	c.F.Rock[i] = true
	c.R.Geology.Stone[i] = geology.Granite
	c.R.Roof[i] = roof.Natural
}

// setWater makes (x, y) a consistent water cell.
func setWater(c *mapgen.Ctx, x, y int, w mapgen.Water) {
	i := c.Idx(x, y)
	c.R.Terrain.Cells[i] = terrain.ShallowWater
	if w == mapgen.Deep {
		c.R.Terrain.Cells[i] = terrain.DeepWater
	}
	c.F.Water[i] = w
}

// noiseFixture is a plausible finished map: rock ridges (~20%), a few
// lakes, a road, chunks, plants and a river line, all consistent.
func noiseFixture(seed uint64, w, h int) *mapgen.Ctx {
	c := fixture(seed, w, h)
	rs, ws := noise2.Seed(seed, "fx/rock"), noise2.Seed(seed, "fx/water")
	for y := range h {
		for x := range w {
			i := c.Idx(x, y)
			fx, fy := float64(x), float64(y)
			c.F.Fertility[i] = float32(noise2.FBm(ws^1, fx, fy, 2, 30, 0.5))
			switch e, wt := noise2.FBm(rs, fx, fy, 3, 40, 0.5), noise2.FBm(ws, fx, fy, 2, 50, 0.5); {
			case e > 0.62:
				setRock(c, x, y)
			case wt < 0.24:
				setWater(c, x, y, mapgen.Deep)
				c.F.Lake[i] = true
			case wt < 0.28:
				setWater(c, x, y, mapgen.Shallow)
				c.F.Lake[i] = true
			}
		}
	}
	c.F.RockDepth = mapgen.ChebDepth(w, h, c.F.Rock)
	// a road along one row, cutting through whatever is there
	ry := h / 3
	for x := range w {
		i := c.Idx(x, ry)
		c.R.Terrain.Cells[i] = terrain.Track
		c.F.Rock[i], c.F.Water[i], c.F.Lake[i] = false, mapgen.Dry, false
		c.R.Geology.Stone[i], c.R.Roof[i] = geology.None, 0
		c.F.Road[i], c.F.Blocked[i] = true, true
	}
	r := c.Rand("fx")
	for i, t := range c.R.Terrain.Cells {
		x, y := c.XY(i)
		if t != terrain.Soil {
			continue
		}
		switch v := r.IntN(1000); {
		case v < 4:
			c.R.Chunks = append(c.R.Chunks, mapgen.ChunkSpawn{X: x, Y: y, Stone: geology.Granite})
			c.F.Blocked[i] = true
		case v < 80:
			c.R.Plants = append(c.R.Plants, mapgen.PlantSpawn{X: x, Y: y, Species: plant.Tree, Growth: 100})
		}
	}
	return c
}
