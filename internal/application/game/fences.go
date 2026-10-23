package game

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/world/region"
)

// Fences (SIM-028). A fence keeps every animal out (or in); a fence gate
// lets the colony's tame animals through but no wild ones. Colonists and
// raiders climb over a fence at fenceClimb times the step cost and walk
// through a gate. Fence cells are a derived per-map index, rebuilt after
// any building change (markRoomsDirty), so saves need nothing extra.

// fenceClimb multiplies the step cost of a pawn crossing a fence.
const fenceClimb = 3

// Fence cell values.
const (
	fenceNone uint8 = iota
	fencePost
	fenceGate
)

// fenceIndex marks the cells holding a built fence (fencePost) or gate
// (fenceGate). built is false until the next query after a building
// change; gen counts rebuilds. Which side of a fence a cell is on comes
// from the region labels (regions.go: the tame and wild layers).
type fenceIndex struct {
	built bool
	cells []uint8
	n     int
	gen   uint64
}

// fences is the current fence index, rebuilt from the buildings when
// stale.
func (g *Game) fences() *fenceIndex {
	f := &g.fence
	if f.built {
		return f
	}
	f.built, f.n = true, 0
	f.gen++
	w, h := g.grid.Width(), g.grid.Height()
	if len(f.cells) != w*h {
		f.cells = make([]uint8, w*h)
	} else {
		clear(f.cells)
	}
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if !ok || !construction.DefOf(b.Kind).AnimalBlock {
			continue
		}
		v := fencePost
		if construction.DefOf(b.Kind).Gate {
			v = fenceGate
		}
		cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		for _, c := range cells {
			if g.grid.InBounds(c[0], c[1]) {
				f.cells[c[1]*w+c[0]] = v
				f.n++
			}
		}
	}
	return f
}

// fenceDirty drops the fence index after a building change.
func (g *Game) fenceDirty() { g.fence.built = false }

// fenceAt is what stands on (x, y): fenceNone, fencePost or fenceGate.
func (g *Game) fenceAt(x, y int) uint8 {
	f := g.fences()
	if f.n == 0 || !g.grid.InBounds(x, y) {
		return fenceNone
	}
	return f.cells[y*g.grid.Width()+x]
}

// fenceBars reports that an animal (tame or not) may not enter (x, y):
// a fence, or a gate for a wild one.
func (g *Game) fenceBars(tame bool, x, y int) bool {
	switch g.fenceAt(x, y) {
	case fencePost:
		return true
	case fenceGate:
		return !tame
	}
	return false
}

// fenceSlow is the step cost base after climbing a fence on (x, y).
func (g *Game) fenceSlow(x, y, base int) int {
	if g.fenceAt(x, y) == fencePost {
		return base * fenceClimb
	}
	return base
}

// fenceQuery is an animal's walkability with fences shut (gates open for
// tame ones). g and gen tie it to the fence index it copied (regions.go:
// its region extensions hold only while that index stands).
type fenceQuery struct {
	pathing.Query
	cells  []uint8
	w      int
	gateOK bool
	g      *Game
	gen    uint64
}

func (q fenceQuery) Walkable(x, y int) bool {
	if !q.Query.Walkable(x, y) {
		return false
	}
	switch q.cells[y*q.w+x] {
	case fencePost:
		return false
	case fenceGate:
		return q.gateOK
	}
	return true
}

// animalQuery wraps an animal's base walkability (the grid for tame
// ones, the hostile map for wild ones) with the fences; with no fences
// on the map it is base itself.
func (g *Game) animalQuery(tame bool, base pathing.Query) pathing.Query {
	f := g.fences()
	if f.n == 0 {
		return base
	}
	return fenceQuery{Query: base, cells: f.cells, w: g.grid.Width(), gateOK: tame, g: g, gen: f.gen}
}

// animalBase is the walkability an animal plans with before fences.
func (g *Game) animalBase(tame bool) pathing.Query {
	if tame {
		return colonyQuery{g} // the grid: the colony's own animals use its doors
	}
	return g.hostileMap()
}

// animalQCache is one boxed animal path query and the versions it was
// built at (animalPathQuery).
type animalQCache struct {
	q          pathing.Query
	fgen, bver uint64
	ok         bool
}

// animalPathQuery is animalQuery over animalBase(tame), or for a wild
// animal with phase set over the movement phase's hostile map: the same
// query, boxed once per fence generation and base version (structVer for
// the live hostile map, the phase map's rebuild count) so an animal's
// path search allocates nothing. Under the test self-check it is rebuilt
// so the cached door set is checked as before.
func (g *Game) animalPathQuery(tame, phase bool) pathing.Query {
	f := g.fences()
	slot, ver := 0, uint64(0)
	var phaseBase pathing.Query
	switch {
	case tame:
	case phase:
		phaseBase = g.phaseHostileMap()
		slot, ver = 2, g.occ.hostileGen
	default:
		slot, ver = 1, g.structVer
	}
	c := &g.animalQ[slot]
	if c.ok && c.fgen == f.gen && c.bver == ver && !selfCheck() {
		return c.q
	}
	base := phaseBase
	if !phase || tame {
		base = g.animalBase(tame)
	}
	*c = animalQCache{q: g.animalQuery(tame, base), fgen: f.gen, bver: ver, ok: true}
	return c.q
}

// fenceReachable returns, while fences stand, a filter of the cells an
// animal standing at pos could walk to (or stand beside, for cells it
// cannot enter); nil when there are no fences (everything is a
// candidate, as before fences existed).
func (g *Game) fenceReachable(a *animal.State, pos Position) func(x, y int) bool {
	f := g.fences()
	if f.n == 0 {
		return nil
	}
	if !g.grid.InBounds(pos.X, pos.Y) {
		return nil
	}
	l := region.Wild
	if a.Tame {
		l = region.Tame
	}
	m := g.regionsNow()
	w, h := g.grid.Width(), g.grid.Height()
	own := m.Label(l, pos.X, pos.Y)
	if own < 0 {
		// standing on a fence cell (built around it): any neighbour's area
		for _, d := range stepDirs {
			if lb := m.Label(l, pos.X+d[0], pos.Y+d[1]); lb >= 0 {
				own = lb
				break
			}
		}
	}
	if own < 0 {
		return func(int, int) bool { return false } // boxed in: nothing in reach
	}
	return func(x, y int) bool {
		if x < 0 || y < 0 || x >= w || y >= h {
			return false
		}
		if lb := m.Label(l, x, y); lb >= 0 {
			return lb == own
		}
		if f.cells[y*w+x] != fenceNone {
			return false // food on a fence cell is behind it
		}
		for _, d := range stepDirs { // a tree or trough: eaten from beside
			if m.Label(l, x+d[0], y+d[1]) == own {
				return true
			}
		}
		return false
	}
}
