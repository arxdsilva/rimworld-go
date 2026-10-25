package resources

import (
	"slices"

	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/noise2"
)

// Chunks is the "chunks" pass (MAPGEN-2 §3.2): writes ChunkDensity, Chunks,
// Blocked. The start-area clearing belongs to the "start" step.
type Chunks struct{}

// Name is the step's stream and label key.
func (Chunks) Name() string { return "chunks" }

const (
	chunkRate     = 0.004 // per eligible cell, times the density
	chunkNoise    = 60.0  // noise cell size at 250×250
	nearRockCells = 6     // within this many cells of rock the density peaks
	nearRockMul   = 1.6
	openMul       = 0.6
	roughMul      = 1.3
	falloffCells  = 8 // the near-rock boost fades over this many more cells
)

// Run is the pass.
func (Chunks) Run(c *mapgen.Ctx) error {
	r := c.Rand("chunks")
	seed := c.NoiseSeed("chunks/density")
	cell := chunkNoise * c.Scale()
	tm := c.R.Terrain.Cells
	dist := mapgen.BFSDist(c.W, c.H, c.F.Rock, nil, true)
	for i := range c.F.ChunkDensity {
		x, y := c.XY(i)
		n := noise2.FBm(seed, float64(x), float64(y), 3, cell, 0.5)
		edge := openMul
		if d := dist[i]; d >= 0 {
			t := mapgen.Smoothstep(nearRockCells, nearRockCells+falloffCells, float64(d))
			edge = nearRockMul + (openMul-nearRockMul)*t
		}
		v := (0.4 + 1.2*n) * edge // noise centred near 1
		if tm[i] == terrain.Gravel || tm[i] == terrain.Stone {
			v *= roughMul
		}
		c.F.ChunkDensity[i] = float32(v)
	}
	eligible := func(i int) bool {
		t := tm[i]
		if !t.Walkable() || terrain.IsWater(t) || t == terrain.Mud || t == terrain.Marsh || t == terrain.Track {
			return false
		}
		if c.F.Water[i].Wet() || c.F.Blocked[i] || c.F.Cave[i] || c.F.Rock[i] || c.F.Road[i] {
			return false
		}
		return c.F.Elevation[i] < c.F.RoughTau || t == terrain.Gravel || t == terrain.Stone
	}
	for i := range tm {
		if !eligible(i) || r.Float64() >= chunkRate*float64(c.F.ChunkDensity[i]) {
			continue
		}
		want := 1 + r.IntN(4)
		cur := i
		var formation []int
		for placed, tries := 0, 0; placed < want && tries < 12; tries++ {
			if eligible(cur) {
				x, y := c.XY(cur)
				c.R.Chunks = append(c.R.Chunks, mapgen.ChunkSpawn{X: x, Y: y, Stone: chunkStone(c, cur)})
				c.F.Blocked[cur] = true
				formation = append(formation, cur)
				placed++
			}
			x, y := c.XY(cur)
			d := dirs4[r.IntN(4)]
			if nx, ny := x+d[0], y+d[1]; c.In(nx, ny) && (eligible(c.Idx(nx, ny)) || slices.Contains(formation, c.Idx(nx, ny))) {
				cur = c.Idx(nx, ny)
			}
		}
	}
	return nil
}

// chunkStone is the cell's geology stone, else the material field's, else
// the site's first stone.
func chunkStone(c *mapgen.Ctx, i int) geology.Stone {
	if s := c.R.Geology.Stone[i]; s != geology.None {
		return s
	}
	if s := c.F.Stone[i]; s != geology.None {
		return s
	}
	if len(c.Cfg.Site.Stones) > 0 {
		return c.Cfg.Site.Stones[0]
	}
	return geology.None
}
