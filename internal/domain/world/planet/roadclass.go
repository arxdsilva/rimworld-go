package planet

import (
	"math"
	"slices"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/pkg/randx"
)

// Ancient road network (WG-07, v1): an extra minimum spanning tree over a
// few seeded "ruin" tiles, routed like the settlement roads. Counts and
// the highway share are original tuning.
const (
	ancientMin       = 6
	ancientSpan      = 5 // 6..10 ruins
	ancientTries     = 64
	ancientHighwayDn = 3 // the longest third of the tree becomes highway
)

// baseRoadClass is the class of settlement edge e (k-th edge), built to
// the lower tech of its two ends: a stone road between industrial (or
// better) bases, otherwise a path or a dirt road, half and half by a hash
// of the edge.
func baseRoadClass(p *Planet, e [2]int32, k int) RoadClass {
	industrial := true
	for _, t := range e {
		if s := p.Settlement[t]; s >= 0 {
			f := p.settlements[s].faction
			industrial = industrial && f < len(p.Factions) && faction.DefOf(p.Factions[f].Type).Tech >= faction.Industrial
		}
	}
	if industrial {
		return StoneRoad
	}
	if unit(randx.Mix64(p.Seed^saltAncient), int(e[0])*7919+int(e[1])+k*104729) < 0.5 {
		return Path
	}
	return DirtRoad
}

// ancientEdges appends the ancient network's edges (and classes) to the
// road edge list: ruins on passable, non-ice land, joined per landmass
// (comp) by a Kruskal tree on great-circle length.
func ancientEdges(b *Builder, comp []int32) {
	p, g := b.w, &b.g
	r := b.phaseRand(saltAncient)
	want := ancientMin + r.IntN(ancientSpan)
	n := p.TileCount()
	var ruins []int32
	for try := 0; try < want*ancientTries && len(ruins) < want && n > 0; try++ {
		t := int32(r.IntN(n))
		if !p.passable(int(t)) || p.Biome[t] == biome.IceSheet || slices.Contains(ruins, t) {
			continue
		}
		ruins = append(ruins, t)
	}
	type cand struct {
		a, b int32
		d    int64
	}
	var cands []cand
	for x := range ruins {
		for y := x + 1; y < len(ruins); y++ {
			a, c := ruins[x], ruins[y]
			if comp[a] < 0 || comp[a] != comp[c] {
				continue
			}
			ang := math.Acos(clampF(dot64(f64(p.Center[a]), f64(p.Center[c])), -1, 1))
			cands = append(cands, cand{min(a, c), max(a, c), int64(math.Round(ang * 1e9))})
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
	parent := make([]int, len(ruins))
	for i := range parent {
		parent[i] = i
	}
	idx := func(t int32) int { return slices.Index(ruins, t) }
	find := func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	var tree []cand
	for _, e := range cands {
		ra, rb := find(idx(e.a)), find(idx(e.b))
		if ra != rb {
			parent[ra] = rb
			tree = append(tree, e)
		}
	}
	// tree is in ascending length: the last third is highway.
	cut := len(tree) - (len(tree)+ancientHighwayDn-1)/ancientHighwayDn
	for k, e := range tree {
		cl := AncientAsphaltRoad
		if k >= cut {
			cl = AncientHighway
		}
		g.edges = append(g.edges, [2]int32{e.a, e.b})
		g.edgeClass = append(g.edgeClass, cl)
	}
}

// endRoadsV1 sorts and deduplicates the routed segments like endRoads,
// keeping the grandest class where routes share a segment.
func endRoadsV1(b *Builder) {
	g := &b.g
	type seg struct {
		r  [2]int32
		cl RoadClass
	}
	segs := make([]seg, len(g.roads))
	for k, r := range g.roads {
		segs[k] = seg{r, g.segClass[k]}
	}
	slices.SortFunc(segs, func(u, v seg) int {
		switch {
		case u.r[0] != v.r[0]:
			return int(u.r[0] - v.r[0])
		case u.r[1] != v.r[1]:
			return int(u.r[1] - v.r[1])
		default:
			return int(v.cl) - int(u.cl)
		}
	})
	segs = slices.CompactFunc(segs, func(u, v seg) bool { return u.r == v.r })
	roads := make([][2]int32, len(segs))
	classes := make([]RoadClass, len(segs))
	for k, s := range segs {
		roads[k], classes[k] = s.r, s.cl
	}
	b.w.Roads, b.w.ext.roadClass = roads, classes
}
