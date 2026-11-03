package industry

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/randx"
)

// Deep ore (wiki: Ground-penetrating scanner, Deep drill): lumps of ore
// under the map that only a scanner finds. A find is one lump: its ore by
// weight, its size in cells and its yield (wiki: size 20-40 / 7-20 /
// 2-10 / 4-10 / 1-5 / 1-5 cells, average yield 9000 / 4200 / 2100 /
// 2100 / 900 / 900 units for steel, silver, plasteel, uranium, jade and
// gold), spread evenly over its cells. The weights and the yield's
// spread (75-125 % of the average) are original tuning.

// oreDef is one deep ore's lump.
type oreDef struct {
	kind           item.Kind
	weight         int // orig
	minSize, maxSz int // wiki
	avgYield       int // wiki
	portion        int // units one drill cycle brings up (orig)
}

var oreDefs = [...]oreDef{
	{item.Steel, 40, 20, 40, 9000, 35},
	{item.Silver, 20, 7, 20, 4200, 35},
	{item.Plasteel, 10, 2, 10, 2100, 10},
	{item.Uranium, 10, 4, 10, 2100, 10},
	{item.Jade, 10, 1, 5, 900, 10},
	{item.Gold, 10, 1, 5, 900, 10},
}

// oreFinds is how many candidate centres a find tries (orig).
const oreTries = 64

// stream is a find's numbers: each call the next hash.
type stream struct{ v uint64 }

func (r *stream) next(n int) int {
	r.v = randx.SplitMix64(r.v)
	return int(r.v % uint64(max(1, n)))
}

// oreAt is the index in s.ore of cell i, and whether it is there.
func (s *State) oreAt(i int32) (int, bool) {
	return slices.BinarySearchFunc(s.ore, i, func(c OreCell, i int32) int { return int(c.I - i) })
}

// OreAt is the deep ore found under (x, y) on a w-wide map and the units
// left (ok false: none found there).
func (s *State) OreAt(x, y, w int) (item.Kind, int, bool) {
	if k, ok := s.oreAt(int32(y*w + x)); ok {
		return item.Kind(s.ore[k].Ore), int(s.ore[k].Left), true
	}
	return 0, 0, false
}

// OreVer moves whenever the found deep ore changes.
func (s *State) OreVer() uint64 { return s.ver }

// Ore is the deep ore found so far, ascending by cell (read only).
func (s *State) Ore() []OreCell { return s.ore }

// oreGround reports a cell a lump may cover: dry, open ground.
func oreGround(t terrain.Type) bool { return !terrain.IsWater(t) && !terrain.IsRockWall(t) }

// Find lays the next lump of deep ore now, as a scanner's find does (dev
// tools and tests); false when no place was found.
func (s *State) Find(h Host) bool { return s.find(h) }

// find lays the next lump of deep ore and posts its letter; false when no
// place was found (the map is all rock and water).
func (s *State) find(h Host) bool {
	w, ht := h.Size()
	if w <= 0 || ht <= 0 {
		return false
	}
	r := stream{h.Seed() ^ uint64(s.finds+1)*0x2545f4914f6cdd1d}
	s.finds++
	total := 0
	for _, d := range oreDefs {
		total += d.weight
	}
	pick, d := r.next(total), oreDefs[0]
	for _, od := range oreDefs {
		if pick < od.weight {
			d = od
			break
		}
		pick -= od.weight
	}
	size := d.minSize + r.next(d.maxSz-d.minSize+1)
	yield := d.avgYield * (75 + r.next(51)) / 100
	free := func(x, y int) bool {
		_, taken := s.oreAt(int32(y*w + x))
		return x >= 0 && y >= 0 && x < w && y < ht && !taken && oreGround(h.Terrain(x, y))
	}
	cx, cy, ok := 0, 0, false
	for range oreTries {
		if cx, cy = r.next(w), r.next(ht); free(cx, cy) {
			ok = true
			break
		}
	}
	if !ok {
		return false
	}
	blob := [][2]int{{cx, cy}}
	in := map[[2]int]bool{{cx, cy}: true}
	for len(blob) < size {
		var edge [][2]int // in blob order, then N, E, S, W: deterministic
		for _, c := range blob {
			for _, dd := range [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
				n := [2]int{c[0] + dd[0], c[1] + dd[1]}
				if !in[n] && free(n[0], n[1]) && !slices.Contains(edge, n) {
					edge = append(edge, n)
				}
			}
		}
		if len(edge) == 0 {
			break
		}
		n := edge[r.next(len(edge))]
		blob, in[n] = append(blob, n), true
	}
	per := max(1, yield/len(blob))
	for _, c := range blob {
		i := int32(c[1]*w + c[0])
		k, _ := s.oreAt(i)
		s.ore = slices.Insert(s.ore, k, OreCell{I: i, Ore: uint8(d.kind), Left: int32(per)})
	}
	s.ver++
	h.Letter("Deep ore found", fmt.Sprintf("A ground scan found %s deep under the ground: about %d units over %d cells. A deep drill set within reach brings it up.",
		d.kind, per*len(blob), len(blob)), cx, cy)
	h.Changed()
	return true
}

// portionOf is how much of ore k one drill cycle brings up.
func portionOf(k item.Kind) int {
	for _, d := range oreDefs {
		if d.kind == k {
			return d.portion
		}
	}
	return 1
}

// drillReach2 is a deep drill's reach, squared (wiki: a 5-cell circle, 21
// cells).
const drillReach2 = 6

// nearestOre is the found ore cell within reach of (x, y) with units left
// nearest it (ties: the lower cell); ok false when none.
func (s *State) nearestOre(x, y, w int) (int, bool) {
	best, bestD := -1, 1<<30
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			d := dx*dx + dy*dy
			if d > drillReach2 || x+dx < 0 || x+dx >= w || y+dy < 0 {
				continue
			}
			if k, ok := s.oreAt(int32((y+dy)*w + x + dx)); ok && s.ore[k].Left > 0 && d < bestD {
				best, bestD = k, d
			}
		}
	}
	return best, best >= 0
}

// take brings up one cycle's portion of ore cell k: the kind and count,
// and whether the cell ran out (it is then dropped from the list).
func (s *State) take(k int) (item.Kind, int, bool) {
	c := &s.ore[k]
	kind := item.Kind(c.Ore)
	n := min(int(c.Left), portionOf(kind))
	c.Left -= int32(n)
	s.ver++
	if c.Left <= 0 {
		s.ore = slices.Delete(s.ore, k, k+1)
		return kind, n, true
	}
	return kind, n, false
}
