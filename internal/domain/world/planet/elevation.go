package planet

import (
	"math"
	"slices"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/noise3"
)

type (
	siteHills = site.Hills
	siteRiver = site.RiverSize
	biomeID   = biome.ID
	stone     = geology.Stone
)

// Elevation numbers (NG-1 §4.1).
const (
	elevOctaves     = 6
	elevFreq        = 1.6
	elevPersistence = 0.5
	landPctMin      = 32 // 38% ± 6%
	landPctSpan     = 13
	maxLandM        = 6000
	maxDepthM       = 4000
	landElevExp     = 2.4 // most land is lowland
)

// blob is a continent seed: a spherical cap that lifts the land.
type blob struct {
	c      [3]float64
	cosRad float64
	rad    float64
}

type elevState struct {
	field noise3.Field
	blobs []blob
}

func beginElev(b *Builder) int {
	r := b.phaseRand(saltElev)
	p, g := b.w, &b.g
	es := &elevState{field: noise3.New(seed64(r))}
	pole := f64(p.Pole)
	nb := 3 + r.IntN(3)
	for len(es.blobs) < nb {
		// A uniform direction, kept when it lies inside the coverage cap so
		// continents land on the visible part of the world.
		z := float64(r.IntN(200001))/100000 - 1
		a := float64(r.IntN(360000)) / 360000 * 2 * math.Pi
		s := math.Sqrt(1 - z*z)
		c := [3]float64{s * math.Cos(a), s * math.Sin(a), z}
		if dot64(c, pole) < float64(p.CapCos) {
			continue
		}
		rad := float64(25+r.IntN(31)) * math.Pi / 180
		es.blobs = append(es.blobs, blob{c: c, cosRad: math.Cos(rad), rad: rad})
	}
	g.land = landPctMin + r.IntN(landPctSpan)
	g.raw = make([]float64, p.TileCount())
	b.g.elev = es
	return p.TileCount()
}

// itemElev is the raw height of tile i: fBm plus the continent mask.
func itemElev(b *Builder, i int) {
	es, p := b.g.elev, b.w
	c := f64(p.Center[i])
	n := es.field.FBm(c[0], c[1], c[2], elevOctaves, elevFreq, elevPersistence)
	mask := 0.0
	for _, bl := range es.blobs {
		d := dot64(c, bl.c)
		if d <= bl.cosRad {
			continue
		}
		t := 1 - math.Acos(clampF(d, -1, 1))/bl.rad
		mask = math.Max(mask, t*t*(3-2*t))
	}
	b.g.raw[i] = 2.2*(n-0.5) + 0.55*mask
}

// endElev puts sea level at the land quantile and converts to metres.
func endElev(b *Builder) {
	p, g := b.w, &b.g
	n := p.TileCount()
	q := make([]int64, n)
	order := make([]int32, n)
	for i := range q {
		q[i] = int64(math.Round(g.raw[i] * 1e9))
		order[i] = int32(i)
	}
	slices.SortFunc(order, func(a, c int32) int {
		if q[a] != q[c] {
			if q[a] < q[c] {
				return -1
			}
			return 1
		}
		return int(a - c)
	})
	landN := (n*g.land + 50) / 100
	seaN := n - landN
	lo, hi := q[order[0]], q[order[n-1]]
	sea := hi
	if seaN < n {
		sea = q[order[seaN]]
	}
	for rank, t := range order {
		v := q[t]
		if rank >= seaN {
			e := 0.0
			if hi > sea {
				e = float64(v-sea) / float64(hi-sea)
			}
			p.Elev[t] = int16(math.Round(maxLandM * math.Pow(e, landElevExp)))
			p.Water[t] = Land
		} else {
			e := 1.0
			if sea > lo {
				e = float64(sea-v) / float64(sea-lo)
			}
			p.Elev[t] = int16(-math.Round(10 + (maxDepthM-10)*math.Pow(e, 0.6)))
			p.Water[t] = Ocean
		}
	}
	g.raw = nil
	g.elev = nil
}
