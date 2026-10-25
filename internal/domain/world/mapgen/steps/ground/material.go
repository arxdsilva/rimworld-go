package ground

import (
	"slices"

	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/pkg/noise2"
)

// Material field shape at a 250×250 map: one broad, low-frequency field per
// stone (two octaves), with warped boundaries; each cell takes the stone
// whose field is highest there.
const (
	matCell     = 180.0
	matMinCell  = 40.0
	matOctaves  = 2
	matGain     = 0.45
	matWarpAmp  = 0.22 // × the cell
	matWarpCell = 0.55 // × the cell
	matBias     = 0.03 // ± per-stone offset, so one stone may dominate
	// regions under this share of the map (percent; at least
	// matMinRegion cells) merge into their neighbour
	matRegionPct   = 2
	matMinRegion   = 16
	matMergePasses = 4
)

// Material is the "material" pass (MAPGEN-2): writes Stone, the stone of
// every cell, in one to three broad regions.
type Material struct{}

// Name is the step's stream and label key.
func (Material) Name() string { return "material" }

// Run is the pass.
func (Material) Run(c *mapgen.Ctx) error {
	r := c.Rand("material")
	stones := siteStones(c)
	if len(stones) == 0 {
		pool := geology.Stones()
		n := 2 + r.IntN(2)
		for k := 0; k < n && len(pool) > 0; k++ {
			j := r.IntN(len(pool))
			stones = append(stones, pool[j])
			pool = slices.Delete(pool, j, j+1)
		}
	}
	out := c.F.Stone
	if len(stones) == 1 {
		for i := range out {
			out[i] = stones[0]
		}
		return nil
	}
	bias := make([]float64, len(stones))
	seeds := make([]uint64, len(stones))
	for k, s := range stones {
		bias[k] = (2*r.Float64() - 1) * matBias
		seeds[k] = c.NoiseSeed("material/" + s.String())
	}
	cell := max(matMinCell, matCell*c.Scale())
	wseed := c.NoiseSeed("material/warp")
	for i := range out {
		x, y := c.XY(i)
		wx, wy := noise2.Warp(wseed, float64(x)+0.5, float64(y)+0.5, cell*matWarpAmp, cell*matWarpCell)
		best, bv := 0, -1.0
		for k := range stones {
			if v := noise2.FBm(seeds[k], wx, wy, matOctaves, cell, matGain) + bias[k]; v > bv {
				best, bv = k, v
			}
		}
		out[i] = stones[best]
	}
	// pockets where two fields nearly tie join the region around them
	minRegion := max(matMinRegion, c.W*c.H*matRegionPct/100)
	mergeSmall(c.W, c.H, out, func(_ geology.Stone, size int) bool { return size < minRegion },
		func(geology.Stone) bool { return true }, matMergePasses)
	return nil
}

// siteStones is the site's stone list without None, unknown stones or
// repeats, in the site's order.
func siteStones(c *mapgen.Ctx) []geology.Stone {
	all := geology.Stones()
	var out []geology.Stone
	for _, s := range c.Cfg.Site.Stones {
		if slices.Contains(all, s) && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}
