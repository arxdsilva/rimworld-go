// Package ground holds the MAPGEN-2 ground passes: the fertility field,
// the stone material regions and the terrain classification (plan §2.4,
// §2.6). All numbers are our own.
package ground

import (
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/pkg/noise2"
)

// Fertility field shape at a 250×250 map (cells scale with the map): a
// medium patch size, four octaves, and a mild warp that bends the patch
// edges without tearing them.
const (
	fertCell     = 52.0
	fertOctaves  = 4
	fertGain     = 0.5
	fertWarpAmp  = 9.0
	fertWarpCell = 70.0
	fertMinCell  = 8.0 // tiny maps still get patches, not noise
)

// Fertility is the "fertility" pass (MAPGEN-2): writes Fertility, a smooth
// field rank-normalised to [0, 1] (a threshold t keeps a share 1-t).
type Fertility struct{}

// Name is the step's stream and label key.
func (Fertility) Name() string { return "fertility" }

// Run is the pass.
func (Fertility) Run(c *mapgen.Ctx) error {
	s := c.Scale()
	cell := max(fertMinCell, fertCell*s)
	amp, wcell := fertWarpAmp*s, max(fertMinCell, fertWarpCell*s)
	seed, wseed := c.NoiseSeed("fertility/field"), c.NoiseSeed("fertility/warp")
	f := c.F.Fertility
	for i := range f {
		x, y := c.XY(i)
		wx, wy := noise2.Warp(wseed, float64(x)+0.5, float64(y)+0.5, amp, wcell)
		f[i] = float32(noise2.FBm(seed, wx, wy, fertOctaves, cell, fertGain))
	}
	mapgen.RankNormalise(f)
	return nil
}
