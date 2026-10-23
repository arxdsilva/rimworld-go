package geology

import (
	"math"
	"slices"

	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/noise"
	"rimworld-go/pkg/randx"
)

// regionCell is the value-noise cell size of the stone regions: large, so
// each stone forms broad bands rather than speckle.
const regionCell = 24

// unreached is the depth of rock with no open ground anywhere on the map.
const unreached = math.MaxInt32

// veinSpec is one ore's vein budget. Original numbers.
type veinSpec struct {
	ore Ore
	// per is how many deep (depth ≥ 2) rock cells buy one vein.
	per int
	// minDeep, when > 0, guarantees one vein once the map has that many
	// deep cells.
	minDeep int
	// A vein is minSize + IntN(sizeSpan) cells.
	minSize, sizeSpan int
}

var veinSpecs = [...]veinSpec{
	{ore: Steel, per: 350, minDeep: 60, minSize: 6, sizeSpan: 9}, // 6–14
	{ore: Silver, per: 1400, minSize: 4, sizeSpan: 6},            // 4–9
	{ore: Gold, per: 2800, minSize: 3, sizeSpan: 4},              // 3–6
	// Buried machinery (NG-1 §4.8) only on site maps (GenerateWith with
	// stones): legacy maps keep their exact ore.
	{ore: Machinery, per: 2500, minSize: 1, sizeSpan: 3}, // 1–3
}

// legacyVeins is the vein table of Generate: the ores before machinery.
var legacyVeins = veinSpecs[:3]

// count is how many veins a map with deep depth-≥2 cells gets.
func (v veinSpec) count(deep int) int {
	n := deep / v.per
	if n == 0 && v.minDeep > 0 && deep >= v.minDeep {
		n = 1
	}
	return n
}

// Generate assigns stone types and ore veins to a generated terrain map.
// It is a pure function of (seed, t); its only randomness is
// randx.New(seed^Salt), never the simulation RNG.
func Generate(seed uint64, t terrain.Map) Map {
	m := NewMap(t.W, t.H)
	if t.W <= 0 || t.H <= 0 || len(t.Cells) != t.W*t.H {
		return m
	}
	r := randx.New(seed ^ Salt)
	assignStones(&m, t, r, pickStones(r))
	placeOre(&m, t, r, legacyVeins)
	return m
}

// GenerateWith is Generate for a landing site: the stones are the site's
// (2–3, from the world tile) instead of a random pick, and deep rock also
// holds buried machinery. Invalid or repeated stones are dropped; with
// none left it is exactly Generate.
func GenerateWith(seed uint64, t terrain.Map, stones []Stone) Map {
	var picked []Stone
	for _, s := range stones {
		if s != None && s < numStones && !slices.Contains(picked, s) {
			picked = append(picked, s)
		}
	}
	if len(picked) == 0 {
		return Generate(seed, t)
	}
	m := NewMap(t.W, t.H)
	if t.W <= 0 || t.H <= 0 || len(t.Cells) != t.W*t.H {
		return m
	}
	r := randx.New(seed ^ Salt)
	assignStones(&m, t, r, picked)
	placeOre(&m, t, r, veinSpecs[:])
	return m
}

// pickStones shuffles the five stones and keeps two or three of them.
func pickStones(r *randx.Rand) []Stone {
	s := Stones()
	for i := len(s) - 1; i > 0; i-- {
		j := r.IntN(i + 1)
		s[i], s[j] = s[j], s[i]
	}
	return s[:2+r.IntN(2)]
}

// natural reports ground that is made of stone: rock walls and stone
// floors, rough or smoothed.
func natural(t terrain.Type) bool {
	return terrain.IsRockWall(t) || t == terrain.Stone || t == terrain.StoneSmooth
}

// assignStones gives every natural cell the stone whose noise layer is
// highest there (ties go to the earlier pick).
func assignStones(m *Map, t terrain.Map, r *randx.Rand, stones []Stone) {
	layers := make([]noise.Value, len(stones))
	for i := range layers {
		layers[i] = noise.New(r, t.W, t.H, regionCell)
	}
	for y := 0; y < t.H; y++ {
		for x := 0; x < t.W; x++ {
			i := y*t.W + x
			if !natural(t.Cells[i]) {
				continue
			}
			best, bestV := 0, -1.0
			for k, l := range layers {
				if v := l.At(x, y); v > bestV {
					best, bestV = k, v
				}
			}
			m.Stone[i] = stones[best]
		}
	}
}

// depths is each cell's Chebyshev distance to the nearest non-wall cell
// (0 for open ground). The map edge counts as rock, not open ground; rock
// with no open ground at all reads as unreached.
func depths(t terrain.Map) []int {
	d := make([]int, t.W*t.H)
	queue := make([]int, 0, len(d))
	for i, c := range t.Cells {
		if terrain.IsRockWall(c) {
			d[i] = unreached
		} else {
			queue = append(queue, i)
		}
	}
	for head := 0; head < len(queue); head++ {
		i := queue[head]
		x, y := i%t.W, i/t.W
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				nx, ny := x+dx, y+dy
				if nx < 0 || ny < 0 || nx >= t.W || ny >= t.H {
					continue
				}
				if n := ny*t.W + nx; d[n] == unreached {
					d[n] = d[i] + 1
					queue = append(queue, n)
				}
			}
		}
	}
	return d
}

// deepCells lists, in index order, the rough rock cells at depth ≥ 2:
// the only places a vein may start.
func deepCells(t terrain.Map, d []int) []int {
	var out []int
	for i, c := range t.Cells {
		if c == terrain.Rock && d[i] >= 2 {
			out = append(out, i)
		}
	}
	return out
}

// placeOre seeds each ore's veins in deep rock and grows them.
func placeOre(m *Map, t terrain.Map, r *randx.Rand, specs []veinSpec) {
	deep := deepCells(t, depths(t))
	if len(deep) == 0 {
		return
	}
	var frontier []int
	for _, v := range specs {
		for n := v.count(len(deep)); n > 0; n-- {
			start := deep[r.IntN(len(deep))]
			size := v.minSize + r.IntN(v.sizeSpan)
			if m.Ore[start] != NoOre {
				continue // landed in an earlier vein: this one is lost
			}
			frontier = growVein(m, t, r, start, v.ore, size, frontier[:0])
		}
	}
}

// growVein fills up to size cells of ore o from start, each step taking a
// random cell of the frontier of 4-adjacent ore-free rough rock. The
// frontier buffer is returned for reuse.
func growVein(m *Map, t terrain.Map, r *randx.Rand, start int, o Ore, size int, frontier []int) []int {
	push := func(i int) []int {
		x, y := i%t.W, i/t.W
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+d[0], y+d[1]
			if nx < 0 || ny < 0 || nx >= t.W || ny >= t.H {
				continue
			}
			if n := ny*t.W + nx; t.Cells[n] == terrain.Rock && m.Ore[n] == NoOre {
				frontier = append(frontier, n)
			}
		}
		return frontier
	}
	m.Ore[start] = o
	frontier = push(start)
	for placed := 1; placed < size && len(frontier) > 0; {
		k := r.IntN(len(frontier))
		c := frontier[k]
		last := len(frontier) - 1
		frontier[k] = frontier[last]
		frontier = frontier[:last]
		if m.Ore[c] != NoOre {
			continue // queued twice
		}
		m.Ore[c] = o
		placed++
		frontier = push(c)
	}
	return frontier
}
