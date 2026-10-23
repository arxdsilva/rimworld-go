package game

import (
	"fmt"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/region"
)

// Path queries (NG-1 §6.5, E3): pathTo and pathAround ask the region map
// before searching, so an unreachable target costs a label lookup instead
// of a flood of the mover's whole component.

// searcher is the game's A* (pathing.Searcher): its search state is kept
// for the game's life instead of the shared pool the garbage collector
// empties, so a long search never reallocates 4 MB at 500².
func (g *Game) searcher() *pathing.Searcher {
	if g.pathSearch == nil {
		g.pathSearch = new(pathing.Searcher)
	}
	return g.pathSearch
}

// reachFrom is the region pre-check for a mover at pos: may a walk reach
// (x, y)? Without regions everything may.
func (g *Game) reachFrom(id entity.EntityID, pos Position, regions bool) reachCheck {
	if !regions {
		return reachCheck{}
	}
	return reachCheck{g: g, m: g.regionsNow(), l: g.moverLayer(id), id: id, pos: pos}
}

// reachCheck is reachFrom's answer (a value, not a closure: asking
// allocates nothing). With no map every cell may be reached.
type reachCheck struct {
	g   *Game
	m   *region.Map
	l   region.Layer
	id  entity.EntityID
	pos Position
}

func (r reachCheck) ok(x, y int) bool {
	if r.m == nil {
		return true
	}
	ok := r.m.Reach(r.l, r.pos.X, r.pos.Y, x, y)
	if !ok && selfCheck() {
		checkReach(r.g.pathQuery(r.id), r.pos.X, r.pos.Y, x, y, ok)
	}
	return ok
}

// Long searches (NG-1 §6.5): a path whose ends are more than longPath
// apart (octile) is searched inside a region corridor (the regions on a
// route over the region graph plus two rings of neighbours) instead of
// over the whole map; shorter ones stay exact tile A*. At most
// longSearchBudget long job searches run per tick: later ones that tick
// are deferred (startJob fails the job without postponing it; the AI
// tries again next tick, in its usual entity-id order).
const (
	longPath         = 96 * pathing.StraightCost
	longSearchBudget = 6
)

// searchBudget counts the tick's long job searches (not saved: it starts
// afresh every tick).
type searchBudget struct {
	tick     uint64
	used     int
	deferred bool // the last pathToJob was deferred
}

// findPath is the tile search behind pathTo: exact below longPath, through
// the region corridor above it (falling back to a full search should the
// corridor hold no path).
func (g *Game) findPath(id entity.EntityID, q pathing.Query, sx, sy, tx, ty int) ([]pathing.Point, bool) {
	if pathing.Octile(sx, sy, tx, ty) <= longPath || g.noRegions() || g.regs.exactLong {
		return g.searcher().Find(q, sx, sy, tx, ty)
	}
	if !q.Walkable(tx, ty) || !g.grid.InBounds(sx, sy) {
		return nil, false
	}
	if sx == tx && sy == ty {
		return nil, true
	}
	allow, ok := g.regionsNow().Corridor(g.moverLayer(id), sx, sy, tx, ty)
	if !ok {
		return checkReachPath(q, sx, sy, tx, ty)
	}
	if p, ok := g.searcher().FindIn(q, allow, sx, sy, tx, ty); ok {
		return p, true
	}
	return g.searcher().Find(q, sx, sy, tx, ty)
}

// findPathVia is findPath, or a plain exact search without regions.
func (g *Game) findPathVia(id entity.EntityID, q pathing.Query, regions bool, sx, sy, tx, ty int) ([]pathing.Point, bool) {
	if !regions {
		return g.searcher().Find(q, sx, sy, tx, ty)
	}
	return g.findPath(id, q, sx, sy, tx, ty)
}

// checkReachPath is a corridor's "no route": under the test self-check it
// is proven with a full search.
func checkReachPath(q pathing.Query, sx, sy, tx, ty int) ([]pathing.Point, bool) {
	checkReach(q, sx, sy, tx, ty, false)
	return nil, false
}

// pathToJob is pathTo for startJob, under the long-search budget: a long
// search past the tick's budget is deferred (g.regs.budget.deferred).
func (g *Game) pathToJob(id entity.EntityID, tx, ty int) ([]pathing.Point, bool) {
	b := &g.regs.budget
	b.deferred = false
	pos, _ := g.positions.Get(id)
	if !g.noRegions() && !g.regs.exactLong && pathing.Octile(pos.X, pos.Y, tx, ty) > longPath {
		if now := uint64(g.clock.Now()); b.tick != now {
			b.tick, b.used = now, 0
		}
		if b.used >= longSearchBudget {
			b.deferred = true
			return nil, false
		}
		b.used++
	}
	return g.pathTo(id, tx, ty)
}

// pathDeferred reports that the last pathToJob was put off by the budget.
func (g *Game) pathDeferred() bool { return g.regs.budget.deferred }

// checkThreatScan, under the test self-check, proves nearestThreatTo's
// indexed animal answer against the scan of every animal it replaced.
func (g *Game) checkThreatScan(id entity.EntityID, pos Position, animalR, raiderR int, got entity.EntityID, gotD int) {
	var want entity.EntityID
	wantD := 0
	for _, aid := range g.animalIDs {
		if _, d, ok := g.threatTo(id, aid, pos, animalR, raiderR); ok && (want == 0 || d < wantD) {
			want, wantD = aid, d
		}
	}
	if want != got || wantD != gotD {
		panic(fmt.Sprintf("nearestThreatTo: index found %d at %d, scan %d at %d", got, gotD, want, wantD))
	}
}
