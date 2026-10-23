package planet

import (
	"hash/fnv"

	"rimworld-go/pkg/noise3"

	"rimworld-go/pkg/randx"
)

// Phase salts: each phase draws from its own randx stream, so changing one
// phase never shifts another's numbers.
const (
	saltShape    = 0x5348415045000001
	saltElev     = 0x454c455600000002
	saltClimate  = 0x434c494d00000003
	saltBiomes   = 0x42494f4d00000004
	saltRivers   = 0x5249564500000005
	saltFactions = 0x4641435400000006
	saltLanding  = 0x4c414e4400000007
)

// stage is one resumable generation phase: begin once, item for each of
// count work units (in index order), end once. Each item does a fixed
// piece of work independent of how Step splits the run, so the planet is
// identical for every step size.
type stage struct {
	phase  Phase
	weight float32
	begin  func(b *Builder) int // returns the item count
	item   func(b *Builder, i int)
	end    func(b *Builder)
}

var stages = []stage{
	{PhaseShape, 0.10, beginShape, itemShape, endShape},
	{PhaseElevation, 0.20, beginElev, itemElev, endElev},
	{PhaseClimate, 0.20, beginClimate, itemClimate, nil},
	{PhaseBiomes, 0.25, beginBiomes, itemBiomes, nil},
	{PhaseRivers, 0.10, beginRivers, itemRivers, endRivers},
	{PhaseFactions, 0.05, nil, itemFactions, nil},
	{PhaseRoads, 0.10, beginRoads, itemRoads, endRoads},
}

// Step does up to units of work (tile visits; an indivisible begin or end
// counts as one unit and may overrun) and reports the phase now running,
// overall progress in [0, 1] and whether the planet is finished.
func (b *Builder) Step(units int) (Phase, float32, bool) {
	if units < 1 {
		units = 1
	}
	for units > 0 && b.stage < len(stages) {
		st := &stages[b.stage]
		if !b.began {
			if b.stage == 0 {
				b.w = &Planet{Params: b.p, Seed: Seed(b.p)}
			}
			b.count = 1
			if st.begin != nil {
				b.count = st.begin(b)
			}
			b.began, b.cursor = true, 0
			units--
		}
		for b.cursor < b.count && units > 0 {
			st.item(b, b.cursor)
			b.cursor++
			units--
		}
		if b.cursor >= b.count {
			if st.end != nil {
				st.end(b)
			}
			b.stage++
			b.began = false
		}
	}
	if b.stage >= len(stages) {
		if b.out == nil {
			finish(b.w)
			b.out, b.w, b.g = b.w, nil, gen{}
		}
		b.phase = PhaseDone
		return PhaseDone, 1, true
	}
	b.phase = stages[b.stage].phase
	var frac float32
	for i := 0; i < b.stage; i++ {
		frac += stages[i].weight
	}
	if b.began && b.count > 0 {
		frac += stages[b.stage].weight * float32(b.cursor) / float32(b.count)
	}
	return b.phase, frac, false
}

// Build generates the whole planet in one call.
func Build(p Params) *Planet {
	b := NewBuilder(p)
	for {
		if _, _, done := b.Step(1 << 30); done {
			return b.Planet()
		}
	}
}

// Seed is the planet seed for p: fnv64a of the seed text xor the version.
func Seed(p Params) uint64 {
	h := fnv.New64a()
	h.Write([]byte(p.SeedText))
	return h.Sum64() ^ uint64(p.Version)
}

// phaseRand is the phase's deterministic stream.
func (b *Builder) phaseRand(salt uint64) *randx.Rand { return randx.New(mix(b.w.Seed ^ salt)) }

// seed64 draws a 64-bit seed from r.
func seed64(r *randx.Rand) uint64 {
	return uint64(r.IntN(1<<31))<<33 ^ uint64(r.IntN(1<<31))<<2 ^ uint64(r.IntN(4))
}

// mix is the splitmix64 finaliser.
func mix(z uint64) uint64 {
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// unit is a per-tile hash in [0, 1).
func unit(seed uint64, i int) float64 {
	return float64(mix(seed^uint64(i)*0x9e3779b97f4a7c15)>>11) / (1 << 53)
}

// gen is scratch state shared between phases while building.
type gen struct {
	keep    []int32 // full-mesh index of each kept tile
	fullIdx []int32 // full-mesh index -> kept index or -1
	raw     []float64
	land    int // land tile target
	sources []int32
	edges   [][2]int32 // road edges (settlement tiles) to route
	roads   [][2]int32
	astar   *astar
	elev    *elevState
	biomes  *biomeState
	rain    noise3.Field
}
