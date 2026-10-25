package water

import (
	"math"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/pkg/noise2"
)

// Wetness is the "wetness" pass (MAPGEN-2 §2.3): ground moisture in
// [0, 1], a climate base (the tile's rain, held near its biome) varied by
// smooth noise, raised near the river, the sea and the lakes. Open water
// is 1. Writes Wetness.
type Wetness struct{}

// Name is the step's stream and label key.
func (Wetness) Name() string { return "wetness" }

// Wetness tuning (our own numbers).
const (
	wetCell      = 48.0 // noise lattice at Scale 1
	wetVary      = 1.2  // noise contrast around the base
	wetRainPull  = 0.07 // how far the tile's rain may move the biome's base
	wetRiverGain = 0.60 // added wetness at the river, fading out over the reach
	wetRiverFar  = 20.0 // ×√Scale
	wetCoastGain = 0.45
	wetCoastFar  = 14.0
	wetLakeGain  = 0.50
	wetLakeFar   = 12.0
)

// Base wetness by yearly rain (mm) and by biome wetness (percent of
// temperate).
var (
	rainMM   = []float64{0, 100, 250, 600, 1000, 1500, 2500, 4000}
	rainWet  = []float64{0.03, 0.08, 0.15, 0.30, 0.42, 0.55, 0.70, 0.82}
	biomePct = []float64{0, 20, 50, 100, 130, 160}
	biomeWet = []float64{0.03, 0.12, 0.26, 0.45, 0.64, 0.74}
)

// wetBase is the map's mean dry-land wetness before water nearby.
func wetBase(c *mapgen.Ctx) float64 {
	b := table(biomePct, biomeWet, float64(c.Cfg.Biome.Wetness))
	if rain := c.Cfg.Site.Rain; rain > 0 {
		b += math.Max(-wetRainPull, math.Min(wetRainPull, table(rainMM, rainWet, float64(rain))-b))
	}
	return b
}

// Run is the pass.
func (Wetness) Run(c *mapgen.Ctx) error {
	f := c.F
	n := c.W * c.H
	base := wetBase(c)
	sc, sc5 := c.Scale(), math.Sqrt(c.Scale())
	seed := c.NoiseSeed("wetness/base")
	sum := 0.0
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			v := noise2.FBm(seed, float64(x), float64(y), 4, wetCell*sc, 0.5)
			f.Wetness[c.Idx(x, y)] = float32(v)
			sum += v
		}
	}
	mean := sum / float64(n) // the base is the map's mean, whatever the noise

	lakeSrc := make([]bool, n)
	for i := range lakeSrc {
		lakeSrc[i] = f.Lake[i] && f.Water[i].Wet()
	}
	lakeDist := distFrom(c.W, c.H, lakeSrc)
	riverFar, coastFar, lakeFar := wetRiverFar*sc5, wetCoastFar*sc5, wetLakeFar*sc5
	near := func(gain, reach float64, d float32) float64 {
		if d == mapgen.Far {
			return 0
		}
		return gain * (1 - mapgen.Smoothstep(0, reach, float64(d)))
	}
	for i := range f.Wetness {
		if f.Water[i].Wet() {
			f.Wetness[i] = 1
			continue
		}
		w := base * (1 + wetVary*(float64(f.Wetness[i])-mean))
		p := max(near(wetRiverGain, riverFar, f.RiverDist[i]), near(wetCoastGain, coastFar, f.CoastDist[i]),
			near(wetLakeGain, lakeFar, lakeDist[i]))
		f.Wetness[i] = float32(mapgen.Clamp01(w + (1-w)*p))
	}
	return nil
}
