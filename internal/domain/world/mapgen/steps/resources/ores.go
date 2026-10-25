package resources

import (
	"slices"

	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/randx"
)

// Ores is the "ores" pass (MAPGEN-2 §3.1): writes Geology.Ore as irregular
// lumps inside deep rock.
type Ores struct{}

// Name is the step's stream and label key.
func (Ores) Name() string { return "ores" }

// oreSpec is one ore's budget (the numbers of geology's veinSpecs, ours).
type oreSpec struct {
	ore geology.Ore
	// per deep cells buy one vein; minDeep > 0 guarantees one vein from
	// that many deep cells.
	per, minDeep int
	// minSize + IntN(sizeSpan) cells per lump.
	minSize, sizeSpan int
	// minDepth is the rock depth an origin needs.
	minDepth uint8
}

var oreSpecs = [...]oreSpec{
	{ore: geology.Steel, per: 350, minDeep: 60, minSize: 6, sizeSpan: 9, minDepth: 2},
	{ore: geology.Silver, per: 1400, minSize: 4, sizeSpan: 6, minDepth: 3},
	{ore: geology.Gold, per: 2800, minSize: 3, sizeSpan: 4, minDepth: 5},
	{ore: geology.Machinery, per: 2500, minSize: 1, sizeSpan: 3, minDepth: 3},
}

const (
	originSpacing = 6   // min Chebyshev distance between origins
	growFactor    = 1.6 // the lump grows to this many times its size, then is trimmed
)

var dirs4 = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// lump is one placed vein: its ore, origin cell and cells.
type lump struct {
	ore    geology.Ore
	origin int
	cells  []int
}

// Run is the pass.
func (Ores) Run(c *mapgen.Ctx) error {
	placeOres(c)
	return nil
}

// placeOres writes Geology.Ore and returns the lumps it placed.
func placeOres(c *mapgen.Ctx) []lump {
	r := c.Rand("ores")
	var placed []lump
	rockAt := func(i int) bool { return c.R.Terrain.Cells[i] == terrain.Rock }
	var deep []int32
	for i := range c.R.Terrain.Cells {
		if rockAt(i) && c.F.RockDepth[i] >= 2 {
			deep = append(deep, int32(i))
		}
	}
	if len(deep) == 0 {
		return nil
	}
	for i := len(deep) - 1; i > 0; i-- { // deterministic shuffle
		j := r.IntN(i + 1)
		deep[i], deep[j] = deep[j], deep[i]
	}
	var origins []mapgen.Point
	for _, sp := range oreSpecs {
		n := len(deep) / sp.per
		if n == 0 && sp.minDeep > 0 && len(deep) >= sp.minDeep {
			n = 1
		}
		next := 0 // rejected candidates stay rejected: origins only grow
		for ; n > 0; n-- {
			var o int
			found := false
			for next < len(deep) {
				i := int(deep[next])
				next++
				if c.F.RockDepth[i] < sp.minDepth || c.R.Geology.Ore[i] != geology.NoOre {
					continue
				}
				x, y := c.XY(i)
				if farFromAll(origins, x, y) {
					o, found = i, true
					break
				}
			}
			if !found {
				break
			}
			x, y := c.XY(o)
			origins = append(origins, mapgen.Point{X: x, Y: y})
			size := sp.minSize + r.IntN(sp.sizeSpan)
			cells := growLump(c, r, o, size)
			for _, i := range cells {
				c.R.Geology.Ore[i] = sp.ore
			}
			placed = append(placed, lump{sp.ore, o, cells})
		}
	}
	return placed
}

func farFromAll(origins []mapgen.Point, x, y int) bool {
	for _, p := range origins {
		if max(abs(p.X-x), abs(p.Y-y)) < originSpacing {
			return false
		}
	}
	return true
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// growLump grows a random 4-connected frontier from origin through free
// rock to ceil(1.6 × size) cells, then trims it back to size.
func growLump(c *mapgen.Ctx, r *randx.Rand, origin, size int) []int {
	target := int(float64(size)*growFactor + 0.999)
	cells := []int{origin}
	in := map[int]bool{origin: true} // membership only; never iterated
	var frontier []int
	addNeighbours := func(i int) {
		x, y := c.XY(i)
		for _, d := range dirs4 {
			nx, ny := x+d[0], y+d[1]
			if !c.In(nx, ny) {
				continue
			}
			j := c.Idx(nx, ny)
			if !in[j] && c.R.Terrain.Cells[j] == terrain.Rock && c.R.Geology.Ore[j] == geology.NoOre && !slices.Contains(frontier, j) {
				frontier = append(frontier, j)
			}
		}
	}
	addNeighbours(origin)
	for len(cells) < target && len(frontier) > 0 {
		k := r.IntN(len(frontier))
		j := frontier[k]
		frontier[k] = frontier[len(frontier)-1]
		frontier = frontier[:len(frontier)-1]
		in[j] = true
		cells = append(cells, j)
		addNeighbours(j)
	}
	for len(cells) > size {
		cells = trimOne(c, r, cells, in, size)
	}
	return cells
}

// trimOne removes the lump cell with the fewest same-lump 4-neighbours
// (ties by the stream) that keeps the lump connected. At the last cut it
// avoids leaving a full rectangle when it can.
func trimOne(c *mapgen.Ctx, r *randx.Rand, lump []int, in map[int]bool, size int) []int {
	neigh := func(i int) int {
		x, y := c.XY(i)
		n := 0
		for _, d := range dirs4 {
			if in[c.Idx(x+d[0], y+d[1])] && c.In(x+d[0], y+d[1]) {
				n++
			}
		}
		return n
	}
	type cand struct{ k, n int }
	var cands []cand
	for k, i := range lump {
		cands = append(cands, cand{k, neigh(i)})
	}
	slices.SortStableFunc(cands, func(a, b cand) int { return a.n - b.n })
	// Tie groups: shuffle the equal-count runs.
	for s := 0; s < len(cands); {
		e := s
		for e < len(cands) && cands[e].n == cands[s].n {
			e++
		}
		for i := e - 1; i > s; i-- {
			j := s + r.IntN(i-s+1)
			cands[i], cands[j] = cands[j], cands[i]
		}
		s = e
	}
	pick := -1
	for _, cd := range cands {
		rest := without(lump, cd.k)
		if !connected(c, rest, in, lump[cd.k]) {
			continue
		}
		if len(lump) == size+1 && size >= 6 && isRect(c, rest) {
			if pick < 0 {
				pick = cd.k // fallback if every option is a rectangle
			}
			continue
		}
		pick = cd.k
		break
	}
	if pick < 0 {
		pick = len(lump) - 1
	}
	delete(in, lump[pick])
	return without(lump, pick)
}

func without(s []int, k int) []int {
	out := make([]int, 0, len(s)-1)
	out = append(out, s[:k]...)
	return append(out, s[k+1:]...)
}

// connected reports the cells form one 4-connected body, treating removed
// as absent from in.
func connected(c *mapgen.Ctx, cells []int, in map[int]bool, removed int) bool {
	if len(cells) == 0 {
		return true
	}
	seen := map[int]bool{cells[0]: true}
	stack := []int{cells[0]}
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		x, y := c.XY(i)
		for _, d := range dirs4 {
			nx, ny := x+d[0], y+d[1]
			if !c.In(nx, ny) {
				continue
			}
			j := c.Idx(nx, ny)
			if j != removed && in[j] && !seen[j] {
				seen[j] = true
				stack = append(stack, j)
			}
		}
	}
	return len(seen) == len(cells)
}

func isRect(c *mapgen.Ctx, cells []int) bool {
	x0, y0, x1, y1 := c.W, c.H, -1, -1
	for _, i := range cells {
		x, y := c.XY(i)
		x0, x1, y0, y1 = min(x0, x), max(x1, x), min(y0, y), max(y1, y)
	}
	return (x1-x0+1)*(y1-y0+1) == len(cells)
}
