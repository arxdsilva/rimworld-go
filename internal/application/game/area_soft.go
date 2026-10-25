package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/grid"
	"rimworld-go/internal/domain/world/region"
)

// Soft area traversal (PLAY-015): a restricted mover's path search prices
// every cell outside its area areaOutsidePct of a plain step extra (a
// step outside costs four), so routes keep inside where they can and
// cross outside where they must. The same cost carries the fire danger
// (fireAvoidCost, fire_contract.go). Walkability never changes: regions
// and reachability are the plain map's.

// areaOutsidePct is the extra cost of entering a cell outside the
// mover's area, percent of a plain step.
const areaOutsidePct = 300

// softCost is one mover's extra path costs, fixed for one search: its
// area mask (nil home and bits: no area to keep to) and whether fire
// costs apply.
type softCost struct {
	g    *Game
	bits []uint64 // the area's member bits
	home []uint8  // the Home mask (Home ref)
	w    int
	area bool
	fire bool
}

// on reports any extra cost at all.
func (s softCost) on() bool { return s.area || s.fire }

// extra is the cost added to entering (x, y), path-step units.
func (s softCost) extra(x, y int) int {
	pct := 0
	if s.area && !s.inside(x, y) {
		pct += areaOutsidePct
	}
	if s.fire {
		pct += s.g.fireAvoidCost(x, y)
	}
	return pct * pathing.StraightCost / 100
}

func (s softCost) inside(x, y int) bool {
	if x < 0 || y < 0 || x >= s.w {
		return false
	}
	n := y*s.w + x
	if s.home != nil {
		return n < len(s.home) && s.home[n] != 0
	}
	return n>>6 < len(s.bits) && s.bits[n>>6]&(1<<(n&63)) != 0
}

// fireActive reports burning cells on the map (fire costs apply).
func (g *Game) fireActive() bool { return len(g.fire.fires) > 0 }

// softCostFor is mover id's extra path costs right now: its area unless
// it is overridden or on a forced or area-exempt job, and fire.
func (g *Game) softCostFor(id entity.EntityID) softCost {
	s := softCost{g: g, w: g.grid.Width(), fire: g.fireActive()}
	r := g.restrictedRef(id)
	if r == 0 || g.areaPathFree(id) {
		return s
	}
	if r == command.AreaHome {
		s.home, s.area = g.homeArea.cells, true
		if s.home == nil {
			s.home = []uint8{} // no Home yet: every cell is outside
		}
		return s
	}
	if i := g.areas.Index(uint16(r)); i >= 0 {
		s.bits, s.area = g.areas.Areas[i].Bits, true
	}
	return s
}

// areaPathFree reports that id paths free of its area: the job startJob
// is starting, or the one it runs, is forced or exempt.
func (g *Game) areaPathFree(id entity.EntityID) bool {
	if g.worldSt != nil && g.worldSt.pathFree == id {
		return true
	}
	if g.animals.Ref(id) != nil {
		return false
	}
	j, _ := g.jobs.Get(id)
	return j.Kind != job.None && (j.Forced || g.areaExemptJob(id, j))
}

// softQuery adds a mover's soft costs to a walkability query, forwarding
// the base's region and bitset extensions (walkability is the base's).
type softQuery struct {
	pathing.Query
	s softCost
}

func (q softQuery) ExtraCost(x, y int) int {
	e := q.s.extra(x, y)
	if c, ok := q.Query.(pathing.Coster); ok {
		e += max(c.ExtraCost(x, y), 0)
	}
	return e
}

func (q softQuery) Reach(sx, sy, tx, ty int) bool {
	switch b := q.Query.(type) {
	case pathing.Reacher:
		return b.Reach(sx, sy, tx, ty)
	case *grid.Grid:
		if g := q.s.g; b == g.grid && !g.noRegions() {
			return checkReach(b, sx, sy, tx, ty, g.regionsNow().Reach(region.Colonist, sx, sy, tx, ty))
		}
	}
	return true
}

func (q softQuery) BlockedBits() []uint64 {
	if b, ok := q.Query.(pathing.Bitset); ok {
		return b.BlockedBits()
	}
	return nil
}

// softPathQuery is base with mover id's soft costs (base itself when it
// has none: legacy routes stay as they were).
func (g *Game) softPathQuery(id entity.EntityID, base pathing.Query) pathing.Query {
	if s := g.softCostFor(id); s.on() {
		return softQuery{Query: base, s: s}
	}
	return base
}

// plainSoft hides q's region and bitset extensions for a reference
// search (plainQuery) but keeps its soft costs, so the search finds the
// same route.
func plainSoft(q pathing.Query) pathing.Query {
	if sq, ok := q.(softQuery); ok {
		return softQuery{Query: plainQuery{sq.Query}, s: sq.s}
	}
	return plainQuery{q}
}

// softAnimalQuery is softPathQuery for an animal mover (a colonist's
// query, from pathQuery, already carries its costs).
func (g *Game) softAnimalQuery(id entity.EntityID, q pathing.Query) pathing.Query {
	if g.animals.Ref(id) == nil {
		return q
	}
	return g.softPathQuery(id, q)
}

// softExtra is q's soft cost of entering (x, y) (0 for a plain query).
func softExtra(q pathing.Query, x, y int) int {
	if s, ok := q.(softQuery); ok {
		return s.s.extra(x, y)
	}
	return 0
}
