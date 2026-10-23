package planet

import (
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/site"
)

// River numbers (NG-1 §4.1).
const (
	riverSourceRain = 500
	riverSourceElev = 200
	riverSourceProb = 0.06
	riverFlowRiver  = 4
	riverFlowLarge  = 12
	lakeMinTiles    = 2
	basinMaxTiles   = 24
	riverMaxSteps   = 4096
)

func beginRivers(b *Builder) int {
	p, g := b.w, &b.g
	seed := mix(p.Seed ^ saltRivers)
	g.sources = g.sources[:0]
	for i := 0; i < p.TileCount(); i++ {
		if p.Water[i] != Land || p.Biome[i] == biome.IceSheet || p.Biome[i] == biome.SeaIce {
			continue
		}
		if p.Rain[i] > riverSourceRain && p.Elev[i] > riverSourceElev && unit(seed, i) < riverSourceProb {
			g.sources = append(g.sources, int32(i))
		}
	}
	return len(g.sources)
}

// riverEnds reports a tile where a river stops: open water, sea ice or an
// ice sheet.
func (p *Planet) riverEnds(t int) bool {
	return p.Water[t] != Land || p.Biome[t] == biome.SeaIce || p.Biome[t] == biome.IceSheet
}

// itemRivers traces source k downhill along the steepest descent until it
// reaches water or ice, joins an existing river, or fills a pit: a basin
// of lakeMinTiles or more becomes a lake, a single-tile pit is filled to
// its lowest rim and the river spills over it.
func itemRivers(b *Builder, k int) {
	p := b.w
	cur := int(b.g.sources[k])
	if p.River[cur] != site.NoRiver {
		return // already on a river
	}
	for step := 0; step < riverMaxSteps; step++ {
		if p.riverEnds(cur) {
			return
		}
		if p.River[cur] != site.NoRiver && p.RiverTo[cur] >= 0 {
			return // joined an existing river
		}
		p.River[cur] = site.Creek
		next := p.lowestNeighbour(cur)
		if next >= 0 && p.Elev[next] < p.Elev[cur] {
			p.RiverTo[cur] = int32(next)
			cur = next
			continue
		}
		basin, outlet := p.basin(cur)
		if len(basin) >= lakeMinTiles || outlet < 0 || p.drainsTo(outlet, cur) {
			for _, t := range basin {
				p.Water[t] = Lake
				p.River[t] = site.NoRiver
				p.RiverTo[t] = -1
			}
			return
		}
		// Single-tile pit: fill it to the rim and spill.
		p.Elev[cur] = p.Elev[outlet]
		p.RiverTo[cur] = int32(outlet)
		cur = outlet
	}
}

// drainsTo reports that the river at from already flows into t (spilling
// there would close a loop).
func (p *Planet) drainsTo(from, t int) bool {
	for s, x := 0, from; x >= 0 && s < riverMaxSteps; s++ {
		if x == t {
			return true
		}
		if p.River[x] == site.NoRiver {
			return false
		}
		x = int(p.RiverTo[x])
	}
	return false
}

// lowestNeighbour is i's lowest neighbour (lowest index on ties), or -1.
func (p *Planet) lowestNeighbour(i int) int {
	best := -1
	for _, nb := range p.Neighbours(i) {
		if best < 0 || p.Elev[nb] < p.Elev[best] || (p.Elev[nb] == p.Elev[best] && int(nb) < best) {
			best = int(nb)
		}
	}
	return best
}

// basin floods outward from pit t in elevation order (priority flood) until
// it pops a tile that drains elsewhere: a tile with a strictly lower
// neighbour outside the basin, or water/ice. It returns the basin and that
// outlet (-1 when the flood hit basinMaxTiles first).
func (p *Planet) basin(t int) ([]int32, int) {
	in := map[int32]bool{int32(t): true}
	basin := []int32{int32(t)}
	var front []int32
	queued := map[int32]bool{}
	push := func(x int32) {
		for _, nb := range p.Neighbours(int(x)) {
			if !in[nb] && !queued[nb] {
				queued[nb] = true
				front = append(front, nb)
			}
		}
	}
	push(int32(t))
	for len(front) > 0 {
		bi := 0
		for j := 1; j < len(front); j++ {
			a, c := front[j], front[bi]
			if p.Elev[a] < p.Elev[c] || (p.Elev[a] == p.Elev[c] && a < c) {
				bi = j
			}
		}
		x := front[bi]
		front = append(front[:bi], front[bi+1:]...)
		if p.riverEnds(int(x)) {
			return basin, int(x)
		}
		for _, nb := range p.Neighbours(int(x)) {
			if !in[nb] && p.Elev[nb] < p.Elev[x] {
				return basin, int(x)
			}
		}
		if len(basin) >= basinMaxTiles {
			return basin, -1
		}
		in[x] = true
		basin = append(basin, x)
		push(x)
	}
	return basin, -1
}

// endRivers accumulates flow down every river (each river tile adds one
// unit of catchment) and sizes it: creek, river from riverFlowRiver,
// large river from riverFlowLarge.
func endRivers(b *Builder) {
	p := b.w
	n := p.TileCount()
	indeg := make([]uint8, n)
	for i := 0; i < n; i++ {
		if p.River[i] != site.NoRiver {
			if to := p.RiverTo[i]; to >= 0 && p.River[to] != site.NoRiver {
				indeg[to]++
			}
		}
	}
	flow := make([]uint16, n)
	queue := make([]int32, 0, 256)
	for i := 0; i < n; i++ {
		if p.River[i] != site.NoRiver && indeg[i] == 0 {
			queue = append(queue, int32(i))
		}
	}
	for h := 0; h < len(queue); h++ {
		t := queue[h]
		if flow[t] < 0xffff {
			flow[t]++
		}
		to := p.RiverTo[t]
		if to < 0 || p.River[to] == site.NoRiver {
			continue
		}
		flow[to] = uint16(min(0xffff, int(flow[to])+int(flow[t])))
		indeg[to]--
		if indeg[to] == 0 {
			queue = append(queue, to)
		}
	}
	for i := 0; i < n; i++ {
		switch {
		case p.River[i] == site.NoRiver:
		case flow[i] >= riverFlowLarge:
			p.River[i] = site.LargeRiver
		case flow[i] >= riverFlowRiver:
			p.River[i] = site.River
		default:
			p.River[i] = site.Creek
		}
	}
}
