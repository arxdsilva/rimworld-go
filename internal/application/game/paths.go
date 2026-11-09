package game

import (
	"fmt"

	"rimworld-go/internal/application/game/pathstage"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
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
	if !ok && checkIndex(r.g) {
		checkReach(r.g, r.g.pathQuery(r.id), r.pos.X, r.pos.Y, x, y, ok)
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

// findPath is the tile search behind pathTo: exact below longPath, through
// the region corridor above it (falling back to a full search should the
// corridor hold no path).
func (g *Game) findPath(id entity.EntityID, q pathing.Query, sx, sy, tx, ty int) ([]pathing.Point, bool) {
	k, allow := planSearch(g, id, q, sx, sy, tx, ty)
	return pathstage.Search(g.searcher(), k, q, allow, sx, sy, tx, ty)
}

// planSearch is how findPath searches; allow, a corridor's cells, is valid
// until the region map's next corridor.
func planSearch(g *Game, id entity.EntityID, q pathing.Query, sx, sy, tx, ty int) (pathstage.Kind, []uint64) {
	switch {
	case pathing.Octile(sx, sy, tx, ty) <= longPath || g.noRegions() || g.regs.exactLong:
		return pathstage.Exact, nil
	case !q.Walkable(tx, ty) || !g.grid.InBounds(sx, sy):
		return pathstage.Failed, nil
	case sx == tx && sy == ty:
		return pathstage.Here, nil
	}
	allow, ok := g.regionsNow().Corridor(g.moverLayer(id), sx, sy, tx, ty)
	if !ok {
		checkReach(g, q, sx, sy, tx, ty, false) // under the test self-check, proven with a full search
		return pathstage.Failed, nil
	}
	return pathstage.Corridor, allow
}

// findPathVia is findPath, or a plain exact search without regions.
func (g *Game) findPathVia(id entity.EntityID, q pathing.Query, regions bool, sx, sy, tx, ty int) ([]pathing.Point, bool) {
	if !regions {
		return g.searcher().Find(q, sx, sy, tx, ty)
	}
	return g.findPath(id, q, sx, sy, tx, ty)
}

// pathToJob is pathTo for startJob, under the long-search budget: a long
// search past the tick's budget is deferred (pathDeferred).
func (g *Game) pathToJob(id entity.EntityID, tx, ty int) ([]pathing.Point, bool) {
	b := &g.regs.budget
	b.Clear()
	pos, _ := g.positions.Get(id)
	if !g.noRegions() && !g.regs.exactLong && pathing.Octile(pos.X, pos.Y, tx, ty) > longPath && !b.Take(uint64(g.clock.Now()), longSearchBudget) {
		return nil, false
	}
	return g.pathTo(id, tx, ty)
}

// pathDeferred reports that the last pathToJob was put off by the budget.
func (g *Game) pathDeferred() bool { return g.regs.budget.Deferred() }

// checkThreatScan, under the test self-check, proves nearestThreatTo's
// indexed animal answer against the scan of every animal it replaced.
func (g *Game) checkThreatScan(id entity.EntityID, pos Position, animalR, raiderR int, got entity.EntityID, gotD int) {
	var want entity.EntityID
	wantD := 0
	for _, aid := range g.animalIDs {
		if _, _, d, ok := g.combat.Guard.ThreatTo(id, aid, pos.X, pos.Y, animalR, raiderR); ok && (want == 0 || d < wantD) {
			want, wantD = aid, d
		}
	}
	if want != got || wantD != gotD {
		panic(fmt.Sprintf("nearestThreatTo: index found %d at %d, scan %d at %d", got, gotD, want, wantD))
	}
}

// runRepaths is the movement phase's path stage (pathstage, GAR-035): the
// animal searches repathEvery queued are planned (every lazy cache their
// queries read is built here), searched against the occupancy the whole
// loop left, and applied in ID order. An answer is used only while the
// walk it was asked for stands (makeWay or a give-up may rewrite it) and
// its first step is free; a failed search doubles the wait, and a wild
// wanderer drops its path (it picks again at its next think).
func runRepaths(g *Game) {
	s := &g.regs.paths
	s.Close()
	if len(s.Requests()) == 0 {
		return
	}
	if g.plantIndex(); !g.noRegions() {
		g.regionsNow().Regions()
	}
	for i, r := range s.Requests() {
		q := g.costedQuery(r.ID)
		q.(pathing.Coster).ExtraCost(r.From.X, r.From.Y) // the ground layer and the region bitsets
		if b, ok := q.(pathing.Bitset); ok {
			b.BlockedBits()
		}
		k, allow := planSearch(g, r.ID, q, r.From.X, r.From.Y, r.Goal.X, r.Goal.Y)
		s.SetPlan(i, q, k, allow)
	}
	s.Compute(g.searcher())
	for i, r := range s.Requests() {
		res := s.Result(i)
		a, ok := g.animals.Get(r.ID)
		if !ok {
			continue
		}
		pos, _ := g.positions.Get(r.ID)
		same := pathstage.TokenOf(a.Path, a.PathI, pathing.Point(pos)) == r.Token
		if res.OK && len(res.Path) > 0 && res.Path[0] != r.To && g.holderFor(r.ID, g.partner(r.ID), Position(res.Path[0])) == 0 {
			a.Path, a.PathI = res.Path, 0
		} else {
			m := g.motionOf(r.ID)
			m.RepathWait = min(2*max(m.RepathWait, repathAfter), repathCap)
			g.motion.Set(r.ID, m)
			same = same && !a.Tame && a.Mode == animal.Wander && !a.Errand
			a.Path, a.PathI = nil, 0
		}
		if same {
			g.animals.Set(r.ID, a)
		}
	}
}
