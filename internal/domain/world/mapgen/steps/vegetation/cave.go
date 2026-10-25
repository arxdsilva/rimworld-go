package vegetation

import (
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/randx"
)

// The cave pass (MAPGEN-3, .cursor/PLAY014_PLAN.md §1): after the surface
// plants, dark cave floor gets duskmoss in small tight clusters, and lit
// cave mouths an occasional ordinary moss where the biome has it. A biome
// with no plants at all keeps its caves bare.
//
// Cave floor is a cave cell of rough stone or gravel that no feature,
// road or puddle took and no plant holds. It is lit within caveMouth
// (Chebyshev) of open sky (an unroofed cell that is not rock), else dark;
// a dark cell is always roofed.
//
// Duskmoss: about caveSeedShare of the dark floor seeds a cluster, which
// grows over edge-touching dark floor to a drawn size of 1–4 cells
// (mostly 2–3). No cluster cell touches another cluster, diagonals
// included, so the clusters stay apart and most of the floor stays bare
// (≈15–25% covered). Cave plants start 30–100% grown. The pass draws its
// own stream, so the surface plants are unchanged by caves.
const (
	caveMouth     = 2
	caveSeedShare = 0.07
	mouthMoss     = 0.06
	caveGrowLo    = 30
)

// clusterOdds are the chances (percent) of a cluster of 1, 2, 3, 4 cells.
var clusterOdds = [4]float64{10, 35, 40, 15}

// cave runs the cave pass.
func (st *state) cave() {
	c := st.c
	if len(st.b.Plants) == 0 {
		return
	}
	n := c.W * c.H
	notSky := make([]bool, n)
	any := false
	for i := range notSky {
		notSky[i] = c.R.Roof[i] != 0 || c.F.Rock[i]
		any = any || c.F.Cave[i]
	}
	if !any {
		return
	}
	sky := mapgen.ChebDepth(c.W, c.H, notSky)
	r := c.Rand("vegetation/cave")
	dark := make([]bool, n)
	var darkCells, lit []int32
	for i := range n {
		if !st.caveFloor(i) {
			continue
		}
		if sky[i] > caveMouth && c.R.Roof[i] != 0 {
			dark[i] = true
			darkCells = append(darkCells, int32(i))
		} else {
			lit = append(lit, int32(i))
		}
	}
	st.duskmoss(r, dark, darkCells)
	if hasMoss(st.b.Plants) {
		for _, i := range lit {
			if r.Float64() < mouthMoss {
				x, y := c.XY(int(i))
				st.caveSpawn(r, x, y, plant.Moss)
			}
		}
	}
}

// caveFloor reports open cave floor a cave plant may take.
func (st *state) caveFloor(i int) bool {
	c := st.c
	t := c.R.Terrain.Cells[i]
	return c.F.Cave[i] && (t == terrain.Stone || t == terrain.Gravel) && !c.F.Blocked[i] && !c.F.Road[i] &&
		!c.F.Water[i].Wet() && !c.F.Rock[i] && st.occ[i] == 0 && !st.nearStart(i)
}

// duskmoss seeds and grows the clusters over the dark cells.
func (st *state) duskmoss(r *randx.Rand, dark []bool, cells []int32) {
	c := st.c
	target := int(caveSeedShare*float64(len(cells)) + 0.5)
	if target == 0 {
		return
	}
	for k := len(cells) - 1; k > 0; k-- {
		j := r.IntN(k + 1)
		cells[k], cells[j] = cells[j], cells[k]
	}
	owner := make([]int32, c.W*c.H)
	seeds := 0
	var grown, cand []int32
	for _, s := range cells {
		if seeds >= target {
			break
		}
		if owner[s] != 0 || st.touchesOther(owner, int(s), -1) {
			continue
		}
		seeds++
		id := int32(seeds)
		size := 1 + pick(r, clusterOdds[:])
		owner[s] = id
		grown = append(grown[:0], s)
		for len(grown) < size {
			cand = cand[:0]
			for _, g := range grown {
				x, y := c.XY(int(g))
				for _, d := range [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
					nx, ny := x+d[0], y+d[1]
					if !c.In(nx, ny) {
						continue
					}
					j := int32(c.Idx(nx, ny))
					if dark[j] && owner[j] == 0 && !containsIdx(cand, j) && !st.touchesOther(owner, int(j), id) {
						cand = append(cand, j)
					}
				}
			}
			if len(cand) == 0 {
				break
			}
			j := cand[r.IntN(len(cand))]
			owner[j] = id
			grown = append(grown, j)
		}
		for _, g := range grown {
			x, y := c.XY(int(g))
			st.caveSpawn(r, x, y, plant.Duskmoss)
		}
	}
}

// touchesOther reports a cell of a cluster other than id among i's eight
// neighbours (id -1: any cluster).
func (st *state) touchesOther(owner []int32, i int, id int32) bool {
	c := st.c
	x, y := c.XY(i)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if (dx != 0 || dy != 0) && c.In(x+dx, y+dy) {
				if o := owner[c.Idx(x+dx, y+dy)]; o != 0 && o != id {
					return true
				}
			}
		}
	}
	return false
}

func containsIdx(s []int32, v int32) bool {
	for _, e := range s {
		if e == v {
			return true
		}
	}
	return false
}

// caveSpawn places a cave plant of species s at (x, y), 30–100% grown.
func (st *state) caveSpawn(r *randx.Rand, x, y int, s plant.Species) {
	pct := caveGrowLo + r.IntN(101-caveGrowLo)
	st.c.R.Plants = append(st.c.R.Plants, mapgen.PlantSpawn{X: x, Y: y, Species: s, Growth: plant.DefOf(s).GrowTicks * pct / 100})
	st.occ[st.c.Idx(x, y)] = uint8(groupOf(s)) + 1
}

// hasMoss reports ordinary moss in the biome's plant list.
func hasMoss(ps []biome.PlantDensity) bool {
	for _, p := range ps {
		if p.Species == plant.Moss && p.Permil > 0 {
			return true
		}
	}
	return false
}
