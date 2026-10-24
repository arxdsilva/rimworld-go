package planet

import (
	"container/heap"
	"math"
	"slices"

	"rimworld-go/internal/domain/world/biome"
)

// routeSeaIceCost is the extra (doubled) cost of crossing sea ice, which
// roads never cross but caravans may (a colony can land on it).
const routeSeaIceCost = 6

// Route is the cheapest overland path from tile a to tile b, both ends
// included, by the road builder's costs (hills, swamps, ice and river
// crossings cost more; sea ice is crossable at a price), or nil when either
// end is not land or no path joins them. Caravans travel it (ENG-003,
// M9). It allocates its own search scratch, so it is safe beside the
// planet's other read-only queries (not Hops, which reuses a buffer).
func (p *Planet) Route(a, b int) []int {
	n := p.TileCount()
	if a < 0 || b < 0 || a >= n || b >= n || !p.Land(a) || !p.Land(b) {
		return nil
	}
	if a == b {
		return []int{a}
	}
	s := newAstar(n)
	s.gen++
	goal := f64(p.Center[b])
	stepAng := math.Acos(clampF(p.stepCos, -1, 1))
	h := func(t int) float64 {
		return math.Acos(clampF(dot64(f64(p.Center[t]), goal), -1, 1)) / stepAng * roadStepCost
	}
	s.stamp[a], s.g[a], s.from[a] = s.gen, 0, -1
	heap.Push(&s.open, openItem{h(a), int32(a)})
	for s.open.Len() > 0 {
		it := heap.Pop(&s.open).(openItem)
		t := int(it.tile)
		if t == b {
			var out []int
			for x := int32(b); x >= 0; x = s.from[x] {
				out = append(out, int(x))
			}
			slices.Reverse(out)
			return out
		}
		if it.f > float64(s.g[t])+h(t)+1e-9 {
			continue // stale entry
		}
		for _, nb := range p.Neighbours(t) {
			if !p.Land(int(nb)) {
				continue
			}
			ng := s.g[t] + int32(p.routeCost(int(nb)))
			if s.stamp[nb] != s.gen || ng < s.g[nb] {
				s.stamp[nb], s.g[nb], s.from[nb] = s.gen, ng, int32(t)
				heap.Push(&s.open, openItem{float64(ng) + h(int(nb)), nb})
			}
		}
	}
	return nil
}

// routeCost is the doubled cost of a caravan entering land tile t.
func (p *Planet) routeCost(t int) int {
	if p.Biome[t] == biome.SeaIce {
		return roadStepCost + routeSeaIceCost
	}
	return p.roadCost(t)
}
