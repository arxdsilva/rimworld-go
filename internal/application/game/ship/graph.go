package ship

import (
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// IsPart reports one of the escape ship's six part kinds.
func IsPart(k construction.Kind) bool {
	switch k {
	case construction.ShipBeam, construction.ShipCasket, construction.ShipCore,
		construction.ShipReactor, construction.ShipEngine, construction.ShipSensor:
		return true
	}
	return false
}

// joins reports two touching parts of kinds a and b one ship: any two
// parts join, but a casket only through a beam (wiki: Ship).
func joins(a, b construction.Kind) bool {
	if a == construction.ShipCasket || b == construction.ShipCasket {
		return a == construction.ShipBeam || b == construction.ShipBeam
	}
	return true
}

// Ship is one connected ship: its id (its lowest part id), its parts
// ascending, and how many of each kind.
type Ship struct {
	ID       uint64
	Parts    []entity.EntityID
	Beams    int
	Caskets  []entity.EntityID
	Cores    int
	Reactors []entity.EntityID
	Engines  int
	Sensors  int
}

// graph is the ships at structure version ver.
type graph struct {
	ver   uint64
	ok    bool
	ships []Ship
	of    map[entity.EntityID]int // part -> index in ships
}

// Ships are the map's ships, ascending id (recomputed when the structure
// version moves; the slice is the cache's, read only).
func (s *State) Ships(h Sites) []Ship {
	if s.g.ok && s.g.ver == h.StructVer() {
		return s.g.ships
	}
	s.g = build(h)
	return s.g.ships
}

// ShipOf is the ship part id belongs to.
func (s *State) ShipOf(h Sites, id entity.EntityID) (Ship, bool) {
	s.Ships(h)
	i, ok := s.g.of[id]
	if !ok {
		return Ship{}, false
	}
	return s.g.ships[i], true
}

// Primary is the ship the launch report speaks of: the biggest with a
// reactor, else the biggest (ties: the lowest id).
func (s *State) Primary(h Sites) (Ship, bool) {
	best, bi := -1, -1
	for i, sh := range s.Ships(h) {
		score := len(sh.Parts)
		if len(sh.Reactors) > 0 {
			score += 1 << 20
		}
		if score > best {
			best, bi = score, i
		}
	}
	if bi < 0 {
		return Ship{}, false
	}
	return s.g.ships[bi], true
}

// build finds the ships: the parts' cells, then a union of every two
// parts that touch side to side and join.
func build(h Sites) graph {
	type part struct {
		id entity.EntityID
		k  construction.Kind
	}
	var parts []part
	cell := map[[2]int]int{}
	for _, id := range h.BuildingIDs() {
		b, ok := h.Building(id)
		if !ok || !IsPart(b.Kind) {
			continue
		}
		cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		for _, c := range cells {
			cell[c] = len(parts)
		}
		parts = append(parts, part{id, b.Kind})
	}
	root := make([]int, len(parts))
	for i := range root {
		root[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for root[i] != i {
			root[i] = root[root[i]]
			i = root[i]
		}
		return i
	}
	for c, i := range cell {
		for _, d := range [2][2]int{{1, 0}, {0, 1}} {
			j, ok := cell[[2]int{c[0] + d[0], c[1] + d[1]}]
			if !ok || j == i || !joins(parts[i].k, parts[j].k) {
				continue
			}
			if a, b := find(i), find(j); a != b {
				root[max(a, b)] = min(a, b)
			}
		}
	}
	g := graph{ver: h.StructVer(), ok: true, of: map[entity.EntityID]int{}}
	at := map[int]int{} // root -> ship index
	for i, p := range parts {
		r := find(i)
		si, ok := at[r]
		if !ok {
			si = len(g.ships)
			at[r] = si
			g.ships = append(g.ships, Ship{ID: uint64(p.id)})
		}
		sh := &g.ships[si]
		sh.Parts = append(sh.Parts, p.id)
		switch p.k {
		case construction.ShipBeam:
			sh.Beams++
		case construction.ShipCasket:
			sh.Caskets = append(sh.Caskets, p.id)
		case construction.ShipCore:
			sh.Cores++
		case construction.ShipReactor:
			sh.Reactors = append(sh.Reactors, p.id)
		case construction.ShipEngine:
			sh.Engines++
		case construction.ShipSensor:
			sh.Sensors++
		}
	}
	// parts came in ascending id order, so each ship's first part is its
	// lowest id; ships sort by it
	slices.SortFunc(g.ships, func(a, b Ship) int { return int(a.ID) - int(b.ID) })
	for i, sh := range g.ships {
		for _, id := range sh.Parts {
			g.of[id] = i
		}
	}
	return g
}
