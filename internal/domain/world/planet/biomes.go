package planet

import (
	"math"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/noise3"
)

// Hills, caves and stones numbers (NG-1 §4.1).
const (
	hillSmall      = 150 // g below: flat
	hillLarge      = 400
	hillMountain   = 900
	hillForceLarge = 3000 // elevation forcing at least large hills
	ridgeMetres    = 450  // ridge-noise term at full ridge on high land
	ridgeFreq      = 5.0
	caveMountain   = 0.30
	caveLarge      = 0.12
	stoneFreq      = 3.0
	stoneThird     = 0.08
)

type biomeState struct {
	ridge    noise3.Field
	stones   [5]noise3.Field
	caveSeed uint64
}

func beginBiomes(b *Builder) int {
	r := b.phaseRand(saltBiomes)
	bs := &biomeState{ridge: noise3.New(seed64(r)), caveSeed: seed64(r)}
	for k := range bs.stones {
		bs.stones[k] = noise3.New(seed64(r))
	}
	b.g.biomes = bs
	return b.w.TileCount()
}

func itemBiomes(b *Builder, i int) {
	p, bs := b.w, b.g.biomes
	c := f64(p.Center[i])
	e := int(p.Elev[i])

	// Stones: the top two of five smooth fields, plus a close third.
	var v [5]float64
	for k := range v {
		v[k] = bs.stones[k].FBm(c[0], c[1], c[2], 3, stoneFreq, 0.5)
	}
	p.Stones[i] = pickStones(v)

	if p.Water[i] != Land {
		p.Hill[i] = site.Flat
		if float64(p.Temp[i]) < seaIceTempX10 && p.Water[i] == Ocean {
			p.Water[i] = Land
			p.Biome[i] = biome.SeaIce
		}
		return
	}

	g := 0
	for _, nb := range p.Neighbours(i) {
		if d := absInt(e - int(max(p.Elev[nb], 0))); d > g {
			g = d
		}
	}
	height := clampF(float64(e)/2000, 0, 1)
	g += int(math.Round(bs.ridge.Ridged(c[0], c[1], c[2], 3, ridgeFreq, 0.5) * ridgeMetres * height))
	h := HillsFor(g, e)
	p.Hill[i] = h
	switch h {
	case site.Mountainous:
		p.Caves[i] = unit(bs.caveSeed, i) < caveMountain
	case site.LargeHills:
		p.Caves[i] = unit(bs.caveSeed, i) < caveLarge
	}
	p.Biome[i] = Classify(float64(p.Temp[i])/10, int(p.Rain[i]), h, e)
}

// HillsFor is the hilliness for a relief g (metres, max neighbour
// difference plus ridges) at elevation e.
func HillsFor(g, e int) site.Hills {
	h := site.Mountainous
	switch {
	case g < hillSmall:
		h = site.Flat
	case g < hillLarge:
		h = site.SmallHills
	case g < hillMountain:
		h = site.LargeHills
	}
	if e > hillForceLarge && h < site.LargeHills {
		h = site.LargeHills
	}
	return h
}

// Classify is the land biome for yearly mean t (°C), rain r (mm/yr),
// hills h and elevation e (m), checked in table order (NG-1 §4.1). The
// tropical rows start at 18 °C, where the temperate rows end, so no
// temperature falls between the two.
func Classify(t float64, r int, h site.Hills, e int) biome.ID {
	low := h <= site.SmallHills
	switch {
	case t < -17:
		return biome.IceSheet
	case t < -5:
		return biome.Tundra
	case t < 5 && r > 1200 && low:
		return biome.ColdBog
	case t < 5:
		return biome.BorealForest
	case t < 18 && r > 1500 && low && e < 400:
		return biome.TemperateSwamp
	case t < 18 && r > 550:
		return biome.TemperateForest
	case t >= 18 && r > 2500 && low && e < 300:
		return biome.TropicalSwamp
	case t >= 18 && r > 1900:
		return biome.TropicalRainforest
	case r >= 250:
		return biome.AridShrubland
	case r >= 100:
		if t >= 15 {
			return biome.Desert
		}
		return biome.Tundra
	case t >= 24:
		return biome.ExtremeDesert
	default:
		return biome.Desert
	}
}

// pickStones takes the two strongest fields, plus the third when it is
// within stoneThird of the second.
func pickStones(v [5]float64) [3]geology.Stone {
	all := geology.Stones()
	var top [3]int
	for k := range top {
		top[k] = -1
	}
	for s := range v {
		for k := 0; k < 3; k++ {
			if top[k] < 0 || v[s] > v[top[k]] {
				copy(top[k+1:], top[k:2])
				top[k] = s
				break
			}
		}
	}
	out := [3]geology.Stone{all[top[0]], all[top[1]], geology.None}
	if v[top[1]]-v[top[2]] <= stoneThird {
		out[2] = all[top[2]]
	}
	return out
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
