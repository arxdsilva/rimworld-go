package landform

import (
	"slices"

	"rimworld-go/internal/domain/world/mapgen"
)

// RockMask is the "rockmask" pass (MAPGEN-2 §2.1): writes
// RockTau/RoughTau/GravelTau and the provisional Rock mask.
//
// The rock threshold is the elevation quantile that leaves the
// hilliness's rock share above it (±25% per seed), so every seed hits its
// profile; the rough-stone and gravel bands are further shares below it.
// Sea ice has no rock: every threshold sits above 1.
type RockMask struct{}

// Name is the step's stream and label key.
func (RockMask) Name() string { return "rockmask" }

const (
	shareJitter = 0.25 // ± seeded variation of the rock share
	roughShare  = 0.06 // rough stone ground just below rock
	gravelShare = 0.08 // gravel below the rough band
	tauGap      = 1e-6 // smallest gap between two thresholds
)

// Run is the pass.
func (RockMask) Run(c *mapgen.Ctx) error {
	r := c.Rand("rockmask")
	share := rockShareOf(c.Cfg.Site.Hills) * (1 + (2*r.Float64()-1)*shareJitter)
	f := c.F
	if seaIce(c) {
		f.RockTau, f.RoughTau, f.GravelTau = 1.3, 1.2, 1.1
		clear(f.Rock)
		return nil
	}
	q := quantiles(f.Elevation, 1-share, 1-share-roughShare, 1-share-roughShare-gravelShare)
	// Keep the order strict even where a tiny map's quantiles coincide.
	f.RockTau = q[0]
	f.RoughTau = min(q[1], f.RockTau-tauGap)
	f.GravelTau = min(q[2], f.RoughTau-tauGap)
	for i, e := range f.Elevation {
		f.Rock[i] = e >= f.RockTau
	}
	return nil
}

// quantiles is mapgen.Quantile for several shares with one sort.
func quantiles(vals []float32, qs ...float64) []float32 {
	out := make([]float32, len(qs))
	if len(vals) == 0 {
		return out
	}
	s := slices.Clone(vals)
	slices.Sort(s)
	for k, q := range qs {
		out[k] = s[int(mapgen.Clamp01(q)*float64(len(s)-1))]
	}
	return out
}
