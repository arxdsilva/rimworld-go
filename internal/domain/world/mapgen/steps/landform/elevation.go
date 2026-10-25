package landform

import (
	"math"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/noise2"
)

// Elevation is the "elevation" pass (MAPGEN-2 §2.1): writes Elevation,
// MountainBias.
//
// The base field is three warped fBm scales: macro (grows with the map, so
// big maps keep the same number of mountain masses), medium (grows with
// the square root of the scale) and fine (fixed, so a big map is more
// detailed, not zoomed). Flat maps weight the smaller scales, so their top
// share is many small peaks (outcrops) instead of one blob. Hilly maps add
// a mountain bias toward the uphill edge with a noise-perturbed front;
// mountainous ones may wrap a second lobe along an adjacent edge. The sum
// is rank-normalised, so a quantile of Elevation is a share of the map.
type Elevation struct{}

// Name is the step's stream and label key.
func (Elevation) Name() string { return "elevation" }

const (
	baseWeight   = 0.85 // E0's weight against the bias
	warpAmp      = 18.0 // domain-warp amplitude at 250², cells
	frontLo      = 0.55 // bias starts 45% of the map from its edge…
	frontHi      = 0.88 // …and is full this close to it
	frontJitter  = 0.6  // front perturbation (× fBm deviation)
	lobeChance   = 0.5  // mountainous: chance of a second lobe
	lobeWeight   = 0.75 // the lobe's share of the main bias
	lobeLo       = 0.70 // the lobe is a narrower band along its edge
	lobeHi       = 0.92
	lobeFadeLo   = 0.25 // the lobe fades out away from the main edge
	lobeFadeHi   = 0.60
	macroCell250 = 120.0
	mediumCell   = 40.0
	fineCell     = 12.0
)

// Run is the pass.
func (Elevation) Run(c *mapgen.Ctx) error {
	r := c.Rand("elevation")
	// Draw in a fixed order whether or not each value is used.
	drawn := site.Dir8(r.IntN(8))
	lobeRoll := r.Float64()
	lobeSide := r.IntN(2)

	s := c.Cfg.Site
	h := hillsKind(s.Hills)
	w := biasWeight(h)
	dir := drawn
	if s.HasUphill {
		dir = s.Uphill & 7
	}
	lobe, lobeDir := false, dir
	if h == site.Mountainous && lobeRoll < lobeChance {
		lobe = true
		step := site.Dir8(2) // a cardinal's adjacent edges are 90° away
		if dir%2 == 1 {
			step = 1 // a corner's are the two edges meeting there
		}
		if lobeSide == 0 {
			lobeDir = (dir + step) & 7
		} else {
			lobeDir = (dir + 8 - step) & 7
		}
	}

	sc := c.Scale()
	rs := math.Sqrt(sc)
	macro := max(16, macroCell250*sc)
	medium := max(8, mediumCell*rs)
	amp := max(3, warpAmp*rs)
	wMacro, wMed, wFine := 0.55, 0.30, 0.15
	if h == site.Flat {
		wMacro, wMed, wFine = 0.15, 0.45, 0.40
	}
	seedWarp := c.NoiseSeed("elevation/warp")
	seedMacro := c.NoiseSeed("elevation/macro")
	seedMed := c.NoiseSeed("elevation/medium")
	seedFine := c.NoiseSeed("elevation/fine")
	seedFront := c.NoiseSeed("elevation/front")
	frontCell := max(10, 50*rs)

	W, H := c.W, c.H
	f := c.F
	for y := 0; y < H; y++ {
		v := (float64(y) + 0.5) / float64(H)
		for x := 0; x < W; x++ {
			u := (float64(x) + 0.5) / float64(W)
			fx, fy := float64(x), float64(y)
			wx, wy := noise2.Warp(seedWarp, fx, fy, amp, macro*0.6)
			e0 := wMacro*noise2.FBm(seedMacro, wx, wy, 3, macro, 0.5) +
				wMed*noise2.FBm(seedMed, wx, wy, 3, medium, 0.5) +
				wFine*noise2.FBm(seedFine, wx, wy, 2, fineCell, 0.5)
			b := 0.0
			if w > 0 {
				t := edgeT(dir, u, v)
				if t+frontJitter/2 > frontLo || lobe { // p is within ±frontJitter/2
					p := (noise2.FBm(seedFront, wx, wy, 3, frontCell, 0.5) - 0.5) * frontJitter
					b = mapgen.Smoothstep(frontLo, frontHi, t+p)
					if lobe {
						l := mapgen.Smoothstep(lobeLo, lobeHi, edgeT(lobeDir, u, v)+p) *
							mapgen.Smoothstep(lobeFadeLo, lobeFadeHi, t)
						b = max(b, lobeWeight*l)
					}
				}
			}
			i := y*W + x
			f.MountainBias[i] = float32(b)
			f.Elevation[i] = float32(baseWeight*e0 + w*b)
		}
	}
	mapgen.RankNormalise(f.Elevation)
	return nil
}

// edgeT is how far (u, v) in [0, 1]² lies toward edge d: 1 at that edge
// (a corner direction: at the corner), 0 at the opposite one.
func edgeT(d site.Dir8, u, v float64) float64 {
	switch d & 7 {
	case 0:
		return 1 - v
	case 1:
		return (1 - v + u) / 2
	case 2:
		return u
	case 3:
		return (v + u) / 2
	case 4:
		return v
	case 5:
		return (v + 1 - u) / 2
	case 6:
		return 1 - u
	}
	return (2 - u - v) / 2
}
