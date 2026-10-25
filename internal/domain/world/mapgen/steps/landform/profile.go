// Package landform holds the MAPGEN-2 landform passes (.cursor/PLAY013_PLAN.md
// §2.1, §2.2, §2.5): elevation and mountain bias, the provisional rock
// mask, cave tunnels, and final rock with cleanup and natural roofs.
package landform

import (
	"math"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

// hillsKind folds the legacy default into small hills.
func hillsKind(h site.Hills) site.Hills {
	if h == site.HillsDefault || h > site.Mountainous {
		return site.SmallHills
	}
	return h
}

// biasWeight is the mountain bias weight per hilliness.
func biasWeight(h site.Hills) float64 {
	switch hillsKind(h) {
	case site.Flat:
		return 0
	case site.LargeHills:
		return 0.18
	case site.Mountainous:
		return 0.38
	}
	return 0.08
}

// rockShare is the nominal rock share of the map per hilliness.
func rockShareOf(h site.Hills) float64 {
	switch hillsKind(h) {
	case site.Flat:
		return 0.04
	case site.LargeHills:
		return 0.22
	case site.Mountainous:
		return 0.42
	}
	return 0.10
}

// seaIce reports a biome with no rock at all.
func seaIce(c *mapgen.Ctx) bool {
	return c.Cfg.Biome.ID == biome.SeaIce || c.Cfg.Site.Biome == biome.SeaIce
}

// dirVec is the unit vector from the map centre toward edge d (0 N is
// y = 0, clockwise).
func dirVec(d site.Dir8) (float64, float64) {
	v := [8][2]float64{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}}[d&7]
	l := math.Hypot(v[0], v[1])
	return v[0] / l, v[1] / l
}

