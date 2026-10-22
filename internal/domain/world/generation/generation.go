package generation

// Generate is a pure function of (seed, width, height): terrain (soil,
// rich soil, gravel, stone ground, rock outcrops) and wild plant spawns.

import (
	"math"
	"sort"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/randx"
)

// PlantSpawn is one wild plant to place at generation time.
type PlantSpawn struct {
	X, Y    int
	Species plant.Species
	Growth  int
}

// Result is a generated map.
type Result struct {
	Terrain terrain.Map
	Plants  []PlantSpawn
	// Clear is the plant- and rock-free start area (inclusive bounds).
	ClearX0, ClearY0, ClearX1, ClearY1 int
}

// StartRadius is the half-size of the guaranteed open start area.
const StartRadius = 5

// valueNoise is smooth lattice noise in [0, 1] with cell size `cell`.
type valueNoise struct {
	cell    float64
	gw, gh  int
	lattice []float64
}

func newNoise(r *randx.Rand, w, h int, cell float64) valueNoise {
	gw := int(float64(w)/cell) + 2
	gh := int(float64(h)/cell) + 2
	l := make([]float64, gw*gh)
	for i := range l {
		l[i] = float64(r.IntN(1<<20)) / float64(1<<20)
	}
	return valueNoise{cell: cell, gw: gw, gh: gh, lattice: l}
}

func (n valueNoise) at(x, y int) float64 {
	fx, fy := (float64(x)+0.5)/n.cell, (float64(y)+0.5)/n.cell
	ix, iy := int(math.Floor(fx)), int(math.Floor(fy))
	tx, ty := fx-float64(ix), fy-float64(iy)
	tx = tx * tx * (3 - 2*tx)
	ty = ty * ty * (3 - 2*ty)
	g := func(a, b int) float64 { return n.lattice[b*n.gw+a] }
	top := g(ix, iy)*(1-tx) + g(ix+1, iy)*tx
	bot := g(ix, iy+1)*(1-tx) + g(ix+1, iy+1)*tx
	return top*(1-ty) + bot*ty
}

// Generate builds a map. The centre StartRadius area is always open soil.
func Generate(seed uint64, w, h int) Result {
	r := randx.New(seed ^ 0x6d61706765) // independent of the sim RNG stream
	rock := newNoise(r, w, h, 11)
	rock2 := newNoise(r, w, h, 4)
	fert := newNoise(r, w, h, 7)
	m := terrain.NewMap(w, h)
	cx, cy := w/2, h/2
	res := Result{
		Terrain: m,
		ClearX0: cx - StartRadius, ClearY0: cy - StartRadius,
		ClearX1: cx + StartRadius, ClearY1: cy + StartRadius,
	}
	inClear := func(x, y int) bool {
		return x >= res.ClearX0 && x <= res.ClearX1 && y >= res.ClearY0 && y <= res.ClearY1
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// rock prefers the map edges so the centre stays buildable
			dx := math.Abs(float64(x-cx)) / float64(w/2+1)
			dy := math.Abs(float64(y-cy)) / float64(h/2+1)
			edge := math.Max(dx, dy)
			v := 0.75*rock.at(x, y) + 0.25*rock2.at(x, y) + 0.28*edge
			f := fert.at(x, y)
			t := terrain.Soil
			switch {
			case v > 0.80 && !inClear(x, y):
				t = terrain.Rock
			case v > 0.71:
				t = terrain.Stone
			case f > 0.66:
				t = terrain.RichSoil
			case f < 0.22:
				t = terrain.Gravel
			}
			m.Set(x, y, t)
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if inClear(x, y) {
				continue
			}
			t := m.At(x, y)
			if t.Fertility() == 0 {
				continue
			}
			roll := r.IntN(1000)
			switch {
			case roll < 55:
				res.Plants = append(res.Plants, PlantSpawn{X: x, Y: y, Species: plant.Tree,
					Growth: plant.DefOf(plant.Tree).GrowTicks * (60 + r.IntN(41)) / 100})
			case roll < 80:
				res.Plants = append(res.Plants, PlantSpawn{X: x, Y: y, Species: plant.BerryBush,
					Growth: plant.DefOf(plant.BerryBush).GrowTicks * (40 + r.IntN(61)) / 100})
			}
		}
	}
	return res
}

// GenerateBiome builds a map for biome b (SIM-023). A legacy biome
// (temperate forest) is exactly Generate. Every other biome keeps the
// rock and stone noise and the open start; the soft ground is the fert
// noise ranked through b's Ground mix (ice and sand in the low band, rich
// soil in the high one), so each map holds the biome's shares. Wild plants
// roll per fertile tile against b's Plants permille. A biome of nothing but
// ice (sea ice) has no rock either.
func GenerateBiome(seed uint64, w, h int, b biome.Def) Result {
	if b.Legacy {
		return Generate(seed, w, h)
	}
	r := randx.New(seed ^ 0x6d61706765)
	rock := newNoise(r, w, h, 11)
	rock2 := newNoise(r, w, h, 4)
	fert := newNoise(r, w, h, 7)
	m := terrain.NewMap(w, h)
	cx, cy := w/2, h/2
	res := Result{
		Terrain: m,
		ClearX0: cx - StartRadius, ClearY0: cy - StartRadius,
		ClearX1: cx + StartRadius, ClearY1: cy + StartRadius,
	}
	inClear := func(x, y int) bool {
		return x >= res.ClearX0 && x <= res.ClearX1 && y >= res.ClearY0 && y <= res.ClearY1
	}
	noRock := b.Ground.Ice >= 1000
	soft := make([]int32, 0, w*h)
	fv := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := math.Abs(float64(x-cx)) / float64(w/2+1)
			dy := math.Abs(float64(y-cy)) / float64(h/2+1)
			v := 0.75*rock.at(x, y) + 0.25*rock2.at(x, y) + 0.28*math.Max(dx, dy)
			i := y*w + x
			switch {
			case noRock:
			case v > 0.80 && !inClear(x, y):
				m.Cells[i] = terrain.Rock
				continue
			case v > 0.71:
				m.Cells[i] = terrain.Stone
				continue
			}
			fv[i] = fert.at(x, y)
			soft = append(soft, int32(i))
		}
	}
	// rank the soft tiles by fertility (ties by index) and cut the ranks
	// at the mix's cumulative weights, low band first
	sort.Slice(soft, func(a, c int) bool {
		fa, fc := fv[soft[a]], fv[soft[c]]
		return fa < fc || fa == fc && soft[a] < soft[c]
	})
	g := b.Ground
	bands := [...]struct {
		t terrain.Type
		w int
	}{
		{terrain.Ice, g.Ice}, {terrain.Sand, g.Sand}, {terrain.Gravel, g.Gravel},
		{terrain.Marsh, g.Marsh}, {terrain.Soil, g.Soil}, {terrain.RichSoil, g.Rich},
	}
	total := 0
	for _, bd := range bands {
		total += bd.w
	}
	if total <= 0 {
		total = 1
		bands[4].w = 1 // no mix: plain soil
	}
	band, upTo := 0, 0
	for rank, i := range soft {
		for band < len(bands)-1 && rank*total >= (upTo+bands[band].w)*len(soft) {
			upTo += bands[band].w
			band++
		}
		m.Cells[i] = bands[band].t
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if inClear(x, y) || m.At(x, y).Fertility() == 0 {
				continue
			}
			roll, acc := r.IntN(1000), 0
			for _, pd := range b.Plants {
				if acc += pd.Permil; roll < acc {
					res.Plants = append(res.Plants, PlantSpawn{X: x, Y: y, Species: pd.Species,
						Growth: startGrowth(r, pd.Species)})
					break
				}
			}
		}
	}
	return res
}

// startGrowth is a wild plant's growth at generation: trees 60-100% grown,
// bushes and reeds 40-100%.
func startGrowth(r *randx.Rand, s plant.Species) int {
	d := plant.DefOf(s)
	if d.Trunk && !d.Regrows {
		return d.GrowTicks * (60 + r.IntN(41)) / 100
	}
	return d.GrowTicks * (40 + r.IntN(61)) / 100
}
