package start

import (
	"math"

	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// Metric parameters (our numbers).
const (
	tinyRock    = 5 // rock bodies of this many cells or fewer are tiny
	groveJoin   = 2 // trees this close (Chebyshev) share a grove
	groveMinLen = 6 // trees in a grove
)

// metrics measures the finished map (Repairs left empty).
func metrics(c *mapgen.Ctx) mapgen.Metrics {
	var m mapgen.Metrics
	n := float64(c.W * c.H)
	rock := make([]bool, c.W*c.H)
	landN := 0
	var stones [8]int
	stoneN := 0
	for i, t := range c.R.Terrain.Cells {
		rock[i] = terrain.IsRockWall(t)
		if land(t) {
			landN++
		}
		if s := c.R.Geology.Stone[i]; s != geology.None && int(s) < len(stones) {
			stones[s]++
			stoneN++
		}
	}
	_, sizes := mapgen.Components(c.W, c.H, rock, true)
	total, largest := 0, 0
	for _, s := range sizes {
		total += s
		largest = max(largest, s)
		if s <= tinyRock {
			m.TinyRockBodies++
		}
	}
	m.RockShare = float64(total) / n
	if total > 0 {
		m.LargestRockShare = float64(largest) / float64(total)
	}
	m.MeanPatchSize = n / float64(patches(c))
	m.OpenShare = float64(landN) / n
	if stoneN > 0 {
		for k, v := range stones {
			m.StoneShare[k] = float64(v) / float64(stoneN)
		}
	}
	m.VegQuadrants = vegQuadrants(c)
	m.TreeClusters = groves(c)
	m.RiverSinuosity = sinuosity(c.F.RiverLine)
	m.StartReach = startReach(c)
	return m
}

// patches counts the 4-connected same-terrain patches.
func patches(c *mapgen.Ctx) int {
	cells := c.R.Terrain.Cells
	seen := make([]bool, len(cells))
	queue := make([]int32, 0, 64)
	count := 0
	for s := range cells {
		if seen[s] {
			continue
		}
		count++
		seen[s] = true
		queue = append(queue[:0], int32(s))
		for h := 0; h < len(queue); h++ {
			i := int(queue[h])
			x, y := c.XY(i)
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if !c.In(nx, ny) {
					continue
				}
				if j := c.Idx(nx, ny); !seen[j] && cells[j] == cells[s] {
					seen[j] = true
					queue = append(queue, int32(j))
				}
			}
		}
	}
	return count
}

// vegQuadrants is plants per cell in the NW, NE, SW and SE quadrants.
func vegQuadrants(c *mapgen.Ctx) [4]float64 {
	hw, hh := c.W/2, c.H/2
	area := [4]int{hw * hh, (c.W - hw) * hh, hw * (c.H - hh), (c.W - hw) * (c.H - hh)}
	var cnt [4]int
	for _, p := range c.R.Plants {
		if !c.In(p.X, p.Y) {
			continue
		}
		q := 0
		if p.X >= hw {
			q++
		}
		if p.Y >= hh {
			q += 2
		}
		cnt[q]++
	}
	var out [4]float64
	for q := range out {
		if area[q] > 0 {
			out[q] = float64(cnt[q]) / float64(area[q])
		}
	}
	return out
}

// groves counts the groups of groveMinLen or more woody plants, trees
// within groveJoin cells of each other joined.
func groves(c *mapgen.Ctx) int {
	tree := make([]bool, c.W*c.H)
	var cells []int32
	for _, p := range c.R.Plants {
		if c.In(p.X, p.Y) && plant.DefOf(p.Species).Trunk && !tree[c.Idx(p.X, p.Y)] {
			tree[c.Idx(p.X, p.Y)] = true
			cells = append(cells, int32(c.Idx(p.X, p.Y)))
		}
	}
	seen := make([]bool, c.W*c.H)
	count := 0
	var queue []int32
	for _, s := range cells {
		if seen[s] {
			continue
		}
		seen[s] = true
		queue = append(queue[:0], s)
		for h := 0; h < len(queue); h++ {
			x, y := c.XY(int(queue[h]))
			for ny := max(0, y-groveJoin); ny <= min(c.H-1, y+groveJoin); ny++ {
				for nx := max(0, x-groveJoin); nx <= min(c.W-1, x+groveJoin); nx++ {
					if j := c.Idx(nx, ny); tree[j] && !seen[j] {
						seen[j] = true
						queue = append(queue, int32(j))
					}
				}
			}
		}
		if len(queue) >= groveMinLen {
			count++
		}
	}
	return count
}

// sinuosity is the centreline length over its end-to-end distance (0
// without a river or for a closed loop).
func sinuosity(line []mapgen.PointF) float64 {
	if len(line) < 2 {
		return 0
	}
	length := 0.0
	for k := 1; k < len(line); k++ {
		length += math.Hypot(line[k].X-line[k-1].X, line[k].Y-line[k-1].Y)
	}
	a, b := line[0], line[len(line)-1]
	d := math.Hypot(b.X-a.X, b.Y-a.Y)
	if d < 1e-9 {
		return 0
	}
	return length / d
}

// startReach is the share of dry walkable land (vaults aside) reachable
// from the start, bridges walked.
func startReach(c *mapgen.Ctx) float64 {
	reached, _, _ := reach(c)
	o := newOverlay(c)
	got, total := 0, 0
	for i := range c.R.Terrain.Cells {
		if o.land(c, i) {
			total++
			if reached[i] {
				got++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(got) / float64(total)
}
