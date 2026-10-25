package water

import (
	"math"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/noise2"
)

// Coast is the "coast" pass (MAPGEN-2 §2.3): one sea band per coast edge
// (a diagonal edge is its corner), deep water, then a shallow shelf, then
// a beach on land. Writes Coast, CoastDist, Water; the sea cuts Rock.
type Coast struct{}

// Name is the step's stream and label key.
func (Coast) Name() string { return "coast" }

// Coast tuning (our own numbers).
const (
	coastWideCells   = 40.0 // sea width with the ocean next door, at Scale 1
	coastNarrowCells = 14.0 // sea width with the ocean far away
	coastFarHops     = 6    // CoastNear at or beyond which the sea is narrowest
	coastUnknownHops = 2    // CoastNear 0 (unknown) is treated as this
	coastMaxShare    = 0.3  // the sea never takes more of the map's width across
	coastShoreCell   = 70.0 // lattice of the shoreline wobble along the edge
	coastBandCell    = 26.0 // lattice of the shelf and beach widths
	coastShelfMin    = 4.0  // shallow shelf width range
	coastShelfMax    = 7.0
	coastBeachMin    = 2.0 // beach width range
	coastBeachMax    = 8.0
)

// coastFrame is (distance inland from edge d, position along it) of the
// point (x, y), in continuous map units.
func coastFrame(w, h int, d site.Dir8, x, y float64) (dist, along float64) {
	W, H := float64(w), float64(h)
	const r2 = math.Sqrt2
	switch d & 7 {
	case 0:
		return y, x
	case 2:
		return W - x, y
	case 4:
		return H - y, x
	case 6:
		return x, y
	case 1:
		return (W - x + y) / r2, (x + y) / r2
	case 3:
		return (W - x + H - y) / r2, (x-y)/r2 + H/r2
	case 5:
		return (x + H - y) / r2, (x + y) / r2
	default:
		return (x + y) / r2, (x-y)/r2 + H/r2
	}
}

// coastEdge is one coast edge's sea profile, sampled per unit along it.
type coastEdge struct {
	d            site.Dir8
	sea          []float64 // sea width (shoreline wobble included)
	shelf, beach []float64
}

// at samples a profile at along-edge position a.
func at(p []float64, a float64) float64 {
	k := int(a)
	k = max(0, min(len(p)-2, k))
	t := mapgen.Clamp01(a - float64(k))
	return p[k] + (p[k+1]-p[k])*t
}

// coastWidth is the base sea width in cells at Scale 1: the closer the
// ocean on the planet, the wider.
func coastWidth(near uint8) float64 {
	n := float64(near)
	if near == 0 {
		n = coastUnknownHops
	}
	t := mapgen.Clamp01((coastFarHops - math.Max(1, n)) / (coastFarHops - 1))
	return coastNarrowCells + (coastWideCells-coastNarrowCells)*t
}

// Run is the pass.
func (Coast) Run(c *mapgen.Ctx) error {
	s := c.Cfg.Site
	if s.Coast == 0 {
		return nil
	}
	r := c.Rand("coast")
	sc := c.Scale()
	base := coastWidth(s.CoastNear) * sc
	n := int(2*float64(c.W+c.H)) + 4 // along-edge samples cover every frame
	var edges []coastEdge
	for d := site.Dir8(0); d < 8; d++ {
		if !s.Coast.Has(d) {
			continue
		}
		across := float64(c.W)
		switch d {
		case 0, 4:
			across = float64(c.H)
		case 1, 3, 5, 7:
			across = float64(min(c.W, c.H)) / math.Sqrt2
		}
		width := math.Min(base*(0.9+0.2*r.Float64()), coastMaxShare*across)
		wobble := math.Min((14+10*r.Float64())*sc, 0.6*width)
		off := float64(r.IntN(1 << 16))
		name := "coast/" + string(rune('0'+d))
		shore, shelf, beach := c.NoiseSeed(name+"/shore"), c.NoiseSeed(name+"/shelf"), c.NoiseSeed(name+"/beach")
		e := coastEdge{d: d, sea: make([]float64, n), shelf: make([]float64, n), beach: make([]float64, n)}
		for k := range n {
			a := float64(k) + off
			wob := 2*noise2.FBm(shore, a, 0.5, 4, coastShoreCell*sc, 0.6) - 1
			e.sea[k] = width + wobble*math.Max(-1, math.Min(1, 2.2*wob))
			e.shelf[k] = coastShelfMin + (coastShelfMax-coastShelfMin)*spread(noise2.FBm(shelf, a, 0.5, 2, coastBandCell, 0.5))
			e.beach[k] = coastBeachMin + (coastBeachMax-coastBeachMin)*spread(noise2.FBm(beach, a, 0.5, 2, coastBandCell, 0.5))
		}
		edges = append(edges, e)
	}

	// the sea first; each cell keeps the beach width of the edge whose sea
	// is nearest
	f := c.F
	beachW := make([]float32, c.W*c.H)
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			best, bestDepth, bw := mapgen.Dry, math.Inf(-1), 0.0
			for _, e := range edges {
				dist, along := coastFrame(c.W, c.H, e.d, float64(x)+0.5, float64(y)+0.5)
				depth := at(e.sea, along) - dist // > 0: in the sea
				w := mapgen.Dry
				switch {
				case depth > at(e.shelf, along):
					w = mapgen.Deep
				case depth > 0:
					w = mapgen.Shallow
				}
				if rank(w) > rank(best) {
					best = w
				}
				if depth > bestDepth {
					bestDepth, bw = depth, at(e.beach, along)
				}
			}
			i := c.Idx(x, y)
			beachW[i] = float32(bw)
			if best.Wet() {
				f.Water[i], f.Coast[i], f.Rock[i] = best, true, false
			}
		}
	}
	dropStraySea(c)

	src := make([]bool, c.W*c.H)
	for i := range src {
		src[i] = f.Coast[i] && f.Water[i].Wet()
	}
	if !anyTrue(src) {
		return nil
	}
	copy(f.CoastDist, distFrom(c.W, c.H, src))
	// the beach hugs the sea that is actually there; cliffs stay rock
	for i, d := range f.CoastDist {
		if d > 0 && d <= beachW[i] && !f.Rock[i] {
			f.Water[i], f.Coast[i] = mapgen.Beach, true
		}
	}
	dropStrayBeach(c)
	return nil
}

// dropStrayBeach removes beach pockets that do not touch the sea.
func dropStrayBeach(c *mapgen.Ctx) {
	f := c.F
	label, sizes := mapgen.Components(c.W, c.H, f.Coast, false)
	sea := make([]bool, len(sizes))
	for i, l := range label {
		if l >= 0 && f.Water[i].Wet() {
			sea[l] = true
		}
	}
	for i, l := range label {
		if l >= 0 && !sea[l] {
			f.Water[i], f.Coast[i] = mapgen.Dry, false
		}
	}
}

// rank orders the coast classes from the sea inland.
func rank(w mapgen.Water) int {
	switch w {
	case mapgen.Deep:
		return 3
	case mapgen.Shallow:
		return 2
	case mapgen.Beach:
		return 1
	}
	return 0
}

// dropStraySea turns sea pieces cut off from the map border back into
// land, so the sea is always open to its edge.
func dropStraySea(c *mapgen.Ctx) {
	f := c.F
	wet := make([]bool, c.W*c.H)
	for i := range wet {
		wet[i] = f.Coast[i] && f.Water[i].Wet()
	}
	label, sizes := mapgen.Components(c.W, c.H, wet, false)
	keep := make([]bool, len(sizes))
	for i, l := range label {
		if x, y := c.XY(i); l >= 0 && (x == 0 || y == 0 || x == c.W-1 || y == c.H-1) {
			keep[l] = true
		}
	}
	for i, l := range label {
		if l >= 0 && !keep[l] {
			f.Water[i], f.Coast[i] = mapgen.Dry, false
		}
	}
}

func anyTrue(b []bool) bool {
	for _, v := range b {
		if v {
			return true
		}
	}
	return false
}
