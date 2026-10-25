package water

import (
	"math"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/pkg/noise2"
	"rimworld-go/pkg/randx"
)

// Lakes is the "lakes" pass (MAPGEN-2 §2.3): up to three noisy blobs, deep
// in the middle with a shallow rim and a shore ring, kept away from the
// river and the coast, off rock and off the map's centre third when there
// is room. Writes Lake, Water; cuts Rock under the lake and its shore.
type Lakes struct{}

// Name is the step's stream and label key.
func (Lakes) Name() string { return "lakes" }

// Lake tuning (our own numbers).
const (
	lakesMax       = 3
	lakeRadiusMin  = 6.0 // ×√Scale
	lakeRadiusSpan = 8.0
	lakeWobble     = 0.22 // the outline varies ±this share of the radius
	lakeRimMin     = 2    // shallow rim width range
	lakeRimSpan    = 2
	lakeShoreMin   = 1 // shore ring width range
	lakeShoreSpan  = 3
	lakeKeepOff    = 12.0 // cells from river and coast
	lakeGap        = 8.0  // cells between two lakes' reaches
	lakeTries      = 60
	lakeShrinks    = 3
	lakeShrink     = 0.75
	lakeOutline    = 128 // outline samples around the centre
	lakeRound      = 1.3 // radius of the noise circle: lower is rounder
)

type lakeSpot struct {
	x, y  float64
	reach float64
}

// Run is the pass.
func (Lakes) Run(c *mapgen.Ctx) error {
	n := min(int(c.Cfg.Site.Lakes), lakesMax)
	if n == 0 {
		return nil
	}
	r := c.Rand("lakes")
	f := c.F
	taken := make([]bool, c.W*c.H)
	for i := range taken {
		taken[i] = f.River[i] || f.Coast[i]
	}
	keepOff := distFrom(c.W, c.H, taken)
	shape := c.NoiseSeed("lakes/shape")
	sc5 := math.Sqrt(c.Scale())
	var placed []lakeSpot
	for range n {
		rad := (lakeRadiusMin + lakeRadiusSpan*r.Float64()) * sc5
		rim := float64(lakeRimMin + r.IntN(lakeRimSpan))
		shore := lakeShoreMin + r.IntN(lakeShoreSpan)
		ox, oy := float64(r.IntN(4096)), float64(r.IntN(4096))
		var spot lakeSpot
		ok := false
		for range lakeShrinks {
			reach := rad*(1+lakeWobble) + float64(shore) + 1
			spot, ok = pickLake(c, r, keepOff, placed, reach)
			if ok {
				break
			}
			rad *= lakeShrink
		}
		if !ok {
			continue
		}
		placed = append(placed, spot)
		drawLake(c, spot, rad, rim, shore, shape, ox, oy)
	}
	return nil
}

// pickLake is the best of lakeTries random centres: far enough from the
// river, the coast and the other lakes, then least rock and off the
// centre third.
func pickLake(c *mapgen.Ctx, r *randx.Rand, keepOff []float32, placed []lakeSpot, reach float64) (lakeSpot, bool) {
	W, H := float64(c.W), float64(c.H)
	mx, my := min(int(reach), c.W/2), min(int(reach), c.H/2)
	need := float32((reach+lakeKeepOff)*1.09 + 1) // chamfer runs ≤ 8% long
	cx0, cx1, cy0, cy1 := W/3, 2*W/3, H/3, 2*H/3
	best, bestScore := lakeSpot{}, math.Inf(1)
	for range lakeTries {
		x := mx + r.IntN(max(1, c.W-2*mx))
		y := my + r.IntN(max(1, c.H-2*my))
		if keepOff[c.Idx(x, y)] < need {
			continue
		}
		px, py := float64(x)+0.5, float64(y)+0.5
		clash := false
		for _, o := range placed {
			if math.Hypot(px-o.x, py-o.y) < reach+o.reach+lakeGap {
				clash = true
				break
			}
		}
		if clash {
			continue
		}
		score := rockShare(c, x, y, int(reach))
		dx := math.Max(0, math.Max(cx0-px, px-cx1))
		dy := math.Max(0, math.Max(cy0-py, py-cy1))
		if math.Hypot(dx, dy) <= reach+1 {
			score++
		}
		if score < bestScore {
			best, bestScore = lakeSpot{x: px, y: py, reach: reach}, score
		}
	}
	return best, !math.IsInf(bestScore, 1)
}

// rockShare is the share of rock among a coarse sample of the disc.
func rockShare(c *mapgen.Ctx, x0, y0, rad int) float64 {
	n, rock := 0, 0
	for y := y0 - rad; y <= y0+rad; y += 2 {
		for x := x0 - rad; x <= x0+rad; x += 2 {
			if c.In(x, y) && (x-x0)*(x-x0)+(y-y0)*(y-y0) <= rad*rad {
				n++
				if c.F.Rock[c.Idx(x, y)] {
					rock++
				}
			}
		}
	}
	if n == 0 {
		return 0
	}
	return float64(rock) / float64(n)
}

// drawLake fills a star-shaped blob (its outline radius is smooth noise
// around a circle, so the lake is one piece), deep inside the rim, and
// rings it with shore.
func drawLake(c *mapgen.Ctx, s lakeSpot, rad, rim float64, shore int, seed uint64, ox, oy float64) {
	f := c.F
	ext := int(math.Ceil(s.reach)) + 1
	x0, x1 := max(0, int(s.x)-ext), min(c.W-1, int(s.x)+ext)
	y0, y1 := max(0, int(s.y)-ext), min(c.H-1, int(s.y)+ext)
	// the outline: smooth noise sampled around a circle of the lattice
	var outline [lakeOutline]float64
	for k := range outline {
		a := float64(k)/lakeOutline*2*math.Pi - math.Pi
		v := 2*noise2.FBm(seed, ox+lakeRound*math.Cos(a), oy+lakeRound*math.Sin(a), 2, 1, 0.5) - 1
		outline[k] = rad * (1 + lakeWobble*math.Max(-1, math.Min(1, 2*v)))
	}
	var water []int32
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			i := c.Idx(x, y)
			if f.River[i] || f.Coast[i] || f.Lake[i] {
				continue
			}
			dx, dy := float64(x)+0.5-s.x, float64(y)+0.5-s.y
			d := math.Hypot(dx, dy)
			a := math.Atan2(dy, dx)
			u := (a + math.Pi) / (2 * math.Pi) * lakeOutline
			k := int(u) % lakeOutline
			t := u - math.Floor(u)
			edge := outline[k] + (outline[(k+1)%lakeOutline]-outline[k])*t
			switch {
			case d < edge-rim:
				f.Water[i] = mapgen.Deep
			case d < edge:
				f.Water[i] = mapgen.Shallow
			default:
				continue
			}
			f.Lake[i], f.Rock[i] = true, false
			water = append(water, int32(i))
		}
	}
	// the shore: shore steps out from the water (8-connected)
	ring := water
	for range shore {
		var next []int32
		for _, i := range ring {
			x, y := c.XY(int(i))
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if !c.In(nx, ny) {
						continue
					}
					j := c.Idx(nx, ny)
					if f.Lake[j] || f.River[j] || f.Coast[j] {
						continue
					}
					f.Water[j], f.Lake[j], f.Rock[j] = mapgen.Shore, true, false
					next = append(next, int32(j))
				}
			}
		}
		ring = next
	}
}
