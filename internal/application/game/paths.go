// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"

	"rimworld-go/internal/application/game/pathstage"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
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

// reachFrom is a value, not a closure: only a failed region check needs
// the native owner's sampled exact-path verification.
func (g *Game) reachFrom(id entity.EntityID, pos Position, regions bool) reachCheck {
	if !regions {
		return reachCheck{}
	}
	return reachCheck{pathstage.ReachCheck{Map: g.regionsNow(), Layer: pathstage.MoverLayer(g.sapper(id), g.isRaider(id)), From: pathing.Point(pos)}, g, id}
}

type reachCheck struct {
	pathstage.ReachCheck
	g  *Game
	id entity.EntityID
}

func (r reachCheck) OK(x, y int) bool {
	ok := r.ReachCheck.OK(x, y)
	if !ok && checkIndex(r.g) {
		pathstage.CheckReach(r.g.pathQuery(r.id), r.From.X, r.From.Y, x, y, ok, true)
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
	return pathstage.PlanSearch(q, sx, sy, tx, ty, longPath, g.noRegions(), g.regs.exactLong,
		g.grid.InBounds(sx, sy), pathstage.MoverLayer(g.sapper(id), g.isRaider(id)), g.regionsNow,
		func() { pathstage.CheckReach(q, sx, sy, tx, ty, false, checkIndex(g)) })
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

func pathsFrozen(g *Game) bool {
	return g.regs.paths.Frozen() || g.regs.pawnPaths.Frozen()
}

// preplanPawnPaths batches only the static-obstruction routes that can be
// searched without changing the ordered movement loop.
func preplanPawnPaths(g *Game, movers []mover, now uint64) {
	p := g.maps[0].regs.paths.Pool()
	s := &g.regs.pawnPaths
	s.Open()
	if p == nil || p.Lanes() <= 1 || len(g.pawnIDs) < 8 || g.noRegions() || g.fireActive() {
		s.Close()
		return
	}
	for _, mv := range movers {
		id := mv.id
		if mv.animal || !g.colonist(id) || now < g.motionOf(id).NextMoveAt || !g.alive(id) {
			continue
		}
		j, _ := g.jobs.Get(id)
		if j.State != job.Goto || j.PathI >= len(j.Path) {
			continue
		}
		pos, _ := g.positions.Get(id)
		tx, ty := walkTarget(j)
		next, goal := j.Path[j.PathI], pathing.Point{X: tx, Y: ty}
		if g.grid.Walkable(next.X, next.Y) || !cellStandable(g, tx, ty) || pathing.Point(pos) == goal {
			continue
		}
		s.Queue(pathstage.Request{ID: id, From: pathing.Point(pos), To: next, Goal: goal,
			Token: pathstage.TokenOf(j.Path, j.PathI, pathing.Point(pos)),
			Guard: pathstage.Guard{Kind: j.Kind, Forced: j.Forced, Area: g.restrictedRef(id), AreaFree: g.areaPathFree(id)}})
	}
	if !s.CloseBatch() {
		return
	}
	g.plantIndex()
	g.regionsNow().Regions()
	for i, r := range s.Requests() {
		q := g.pathQuery(r.ID)
		k := pathstage.Failed
		var allow []uint64
		if q.Walkable(r.Goal.X, r.Goal.Y) && g.reachFrom(r.ID, Position(r.From), true).OK(r.Goal.X, r.Goal.Y) {
			pathstage.WarmQuery(q, r.From)
			k, allow = planSearch(g, r.ID, q, r.From.X, r.From.Y, r.Goal.X, r.Goal.Y)
		}
		s.SetPlan(i, q, k, allow)
	}
	s.ComputeAt(p, g.grid.Version(), g.structVer, g.searcher())
}

// preplannedPawnPath returns a staged route only while its job and map still match.
func preplannedPawnPath(g *Game, id entity.EntityID, j job.Record, tx, ty int) ([]pathing.Point, bool, bool) {
	if !cellStandable(g, tx, ty) {
		return nil, false, false
	}
	from, _ := g.positions.Get(id)
	result, ready := g.regs.pawnPaths.Lookup(id, pathstage.Guard{
		Kind: j.Kind, Forced: j.Forced, Area: g.restrictedRef(id), AreaFree: g.areaPathFree(id),
	}, pathing.Point(from), j.Path[j.PathI], pathing.Point{X: tx, Y: ty},
		pathstage.TokenOf(j.Path, j.PathI, pathing.Point(from)), g.grid.Version(), g.structVer, g.fireActive())
	return result.Path, result.OK, ready
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
		pathstage.WarmQuery(q, r.From)
		k, allow := planSearch(g, r.ID, q, r.From.X, r.From.Y, r.Goal.X, r.Goal.Y)
		s.SetPlan(i, q, k, allow)
	}
	s.SetPool(g.maps[0].regs.paths.Pool()) // an opened map runs on the colony's lanes
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
