package planet

import (
	"container/heap"
	"math"
	"slices"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/site"
)

// popPct is the settlement density factor per population setting
// (sparse, normal, crowded), in percent.
var popPct = [3]int{50, 100, 160}

// Road numbers (NG-1 §4.1). Costs are doubled so they stay integers.
const (
	roadExtraPct  = 15
	roadStepCost  = 2
	roadRiverCost = 4
)

var roadHillCost = [...]int{site.HillsDefault: 0, site.Flat: 0, site.SmallHills: 1, site.LargeHills: 3, site.Mountainous: 8}

// itemFactions generates the factions and places their settlements.
func itemFactions(b *Builder, _ int) {
	p := b.w
	r := b.phaseRand(saltFactions)
	p.Factions = b.genFactions(b.p.Factions, r)
	b.place(p.Globe(), p.Factions, popPct[clampInt(b.p.Pop, 0, 2)], r)
	p.settlements = p.settlements[:0]
	for f := range p.Factions {
		for k, t := range p.Factions[f].Settlements {
			if t < 0 || t >= p.TileCount() || p.Water[t] != Land || p.Settlement[t] >= 0 {
				continue
			}
			p.Settlement[t] = int32(len(p.settlements))
			p.settlements = append(p.settlements, settlementRef{faction: f, k: k, tile: int32(t)})
		}
	}
}

// passable is where roads may go: land that is not sea ice.
func (p *Planet) passable(t int) bool { return p.Water[t] == Land && p.Biome[t] != biome.SeaIce }

// beginRoads picks the road network: per landmass, a minimum spanning
// tree over its settlements (great-circle length) plus the shortest
// roadExtraPct% of the remaining pairs as loops.
func beginRoads(b *Builder) int {
	p, g := b.w, &b.g
	n := p.TileCount()
	comp := make([]int32, n)
	for i := range comp {
		comp[i] = -1
	}
	var queue []int32
	nc := int32(0)
	for i := 0; i < n; i++ {
		if comp[i] >= 0 || !p.passable(i) {
			continue
		}
		comp[i] = nc
		queue = append(queue[:0], int32(i))
		for h := 0; h < len(queue); h++ {
			for _, nb := range p.Neighbours(int(queue[h])) {
				if comp[nb] < 0 && p.passable(int(nb)) {
					comp[nb] = nc
					queue = append(queue, nb)
				}
			}
		}
		nc++
	}
	type cand struct {
		a, b int32
		d    int64
	}
	byComp := map[int32][]int32{}
	var comps []int32
	for _, s := range p.settlements {
		c := comp[s.tile]
		if c < 0 {
			continue
		}
		if _, ok := byComp[c]; !ok {
			comps = append(comps, c)
		}
		byComp[c] = append(byComp[c], s.tile)
	}
	slices.Sort(comps)
	g.edges = g.edges[:0]
	for _, c := range comps {
		tiles := byComp[c]
		var cands []cand
		for x := 0; x < len(tiles); x++ {
			for y := x + 1; y < len(tiles); y++ {
				a, bb := tiles[x], tiles[y]
				ang := math.Acos(clampF(dot64(f64(p.Center[a]), f64(p.Center[bb])), -1, 1))
				cands = append(cands, cand{min(a, bb), max(a, bb), int64(math.Round(ang * 1e9))})
			}
		}
		slices.SortFunc(cands, func(u, v cand) int {
			switch {
			case u.d != v.d:
				return cmpInt64(u.d, v.d)
			case u.a != v.a:
				return int(u.a - v.a)
			default:
				return int(u.b - v.b)
			}
		})
		parent := map[int32]int32{}
		var find func(x int32) int32
		find = func(x int32) int32 {
			px, ok := parent[x]
			if !ok || px == x {
				return x
			}
			r := find(px)
			parent[x] = r
			return r
		}
		var extra []cand
		mst := 0
		for _, e := range cands {
			ra, rb := find(e.a), find(e.b)
			if ra != rb {
				parent[ra] = rb
				g.edges = append(g.edges, [2]int32{e.a, e.b})
				mst++
			} else {
				extra = append(extra, e)
			}
		}
		for _, e := range extra[:min(len(extra), (mst*roadExtraPct+50)/100)] {
			g.edges = append(g.edges, [2]int32{e.a, e.b})
		}
	}
	g.roads = g.roads[:0]
	g.astar = newAstar(n)
	return len(g.edges)
}

func itemRoads(b *Builder, k int) {
	e := b.g.edges[k]
	path := b.g.astar.route(b.w, int(e[0]), int(e[1]))
	for j := 1; j < len(path); j++ {
		a, c := path[j-1], path[j]
		b.g.roads = append(b.g.roads, [2]int32{min(a, c), max(a, c)})
	}
}

func endRoads(b *Builder) {
	r := b.g.roads
	slices.SortFunc(r, func(u, v [2]int32) int {
		if u[0] != v[0] {
			return int(u[0] - v[0])
		}
		return int(u[1] - v[1])
	})
	b.w.Roads = slices.Compact(r)
}

func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// roadCost is the doubled cost of entering tile t.
func (p *Planet) roadCost(t int) int {
	c := roadStepCost + roadHillCost[p.Hill[t]]
	switch p.Biome[t] {
	case biome.TemperateSwamp, biome.TropicalSwamp, biome.ColdBog:
		c += 3
	case biome.IceSheet:
		c += 6
	}
	if p.River[t] != site.NoRiver {
		c += roadRiverCost
	}
	return c
}

// astar is reusable A* scratch over the tile graph.
type astar struct {
	g     []int32
	from  []int32
	stamp []uint32
	gen   uint32
	open  openSet
}

func newAstar(n int) *astar {
	return &astar{g: make([]int32, n), from: make([]int32, n), stamp: make([]uint32, n)}
}

type openItem struct {
	f    float64
	tile int32
}

type openSet []openItem

func (o openSet) Len() int { return len(o) }
func (o openSet) Less(i, j int) bool {
	if o[i].f != o[j].f {
		return o[i].f < o[j].f
	}
	return o[i].tile < o[j].tile
}
func (o openSet) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o *openSet) Push(x any)   { *o = append(*o, x.(openItem)) }
func (o *openSet) Pop() any {
	old := *o
	it := old[len(old)-1]
	*o = old[:len(old)-1]
	return it
}

// route is the cheapest passable path from a to b (inclusive), or nil.
// The heuristic is the great-circle angle over the largest neighbour step
// times the minimum step cost, which never overestimates.
func (s *astar) route(p *Planet, a, b int) []int32 {
	s.gen++
	s.open = s.open[:0]
	goal := f64(p.Center[b])
	stepAng := math.Acos(clampF(p.stepCos, -1, 1))
	h := func(t int) float64 {
		ang := math.Acos(clampF(dot64(f64(p.Center[t]), goal), -1, 1))
		return ang / stepAng * roadStepCost
	}
	s.stamp[a], s.g[a], s.from[a] = s.gen, 0, -1
	heap.Push(&s.open, openItem{h(a), int32(a)})
	for s.open.Len() > 0 {
		it := heap.Pop(&s.open).(openItem)
		t := int(it.tile)
		if t == b {
			var path []int32
			for x := int32(b); x >= 0; x = s.from[x] {
				path = append(path, x)
			}
			slices.Reverse(path)
			return path
		}
		if it.f > float64(s.g[t])+h(t)+1e-9 {
			continue // stale entry
		}
		for _, nb := range p.Neighbours(t) {
			if !p.passable(int(nb)) {
				continue
			}
			ng := s.g[t] + int32(p.roadCost(int(nb)))
			if s.stamp[nb] != s.gen || ng < s.g[nb] {
				s.stamp[nb], s.g[nb], s.from[nb] = s.gen, ng, int32(t)
				heap.Push(&s.open, openItem{float64(ng) + h(int(nb)), nb})
			}
		}
	}
	return nil
}

// Globe is the planet as settlement placement sees it (faction.Globe).
func (p *Planet) Globe() faction.Globe { return globe{p} }

type globe struct{ p *Planet }

func (g globe) TileCount() int           { return g.p.TileCount() }
func (g globe) Land(i int) bool          { return g.p.Land(i) }
func (g globe) Biome(i int) biome.ID     { return g.p.Biome[i] }
func (g globe) Hills(i int) uint8        { return uint8(g.p.Hill[i]) }
func (g globe) Hops(a, b, limit int) int { return g.p.Hops(a, b, limit) }

// Land reports a land tile (sea ice counts as land).
func (p *Planet) Land(i int) bool { return i >= 0 && i < p.TileCount() && p.Water[i] == Land }
