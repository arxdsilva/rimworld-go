// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package pathstage

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/region"
)

// Guard is the job and area policy that must still match when a plan is used.
type Guard struct {
	Kind     job.Kind
	Forced   bool
	Area     int
	AreaFree bool
}

// PawnStage adds the map versions to the generic request queue.
type PawnStage struct {
	Stage
	gridVersion, structVersion uint64
}

// CloseBatch keeps only a batch large enough to benefit from parallel
// search. Discarded requests must never be visible to Lookup.
func (s *PawnStage) CloseBatch() bool {
	if len(s.Requests()) < 8 {
		s.Open()
	}
	s.Close()
	return len(s.Requests()) != 0
}

// ComputeAt searches the prepared requests against the recorded versions.
// The owner collects only due obstructed moves and warms their queries.
func (s *PawnStage) ComputeAt(p Pool, grid, structure uint64, sr *pathing.Searcher) {
	s.gridVersion, s.structVersion = grid, structure
	s.SetPool(p)
	s.Compute(sr)
}

// Lookup returns a staged result only while its job, walk, guard and map are
// unchanged. A false ready result asks the owner to use its serial fallback.
func (s *PawnStage) Lookup(id entity.EntityID, guard Guard, from, to, goal pathing.Point, token Token, grid, structure uint64, fire bool) (Result, bool) {
	if fire || s.gridVersion != grid || s.structVersion != structure {
		return Result{}, false
	}
	rs := s.Requests()
	i, found := slices.BinarySearchFunc(rs, id, func(r Request, id entity.EntityID) int {
		return cmp.Compare(r.ID, id)
	})
	if !found {
		return Result{}, false
	}
	r := rs[i]
	if r.Guard != guard || r.From != from || r.To != to || r.Goal != goal || r.Token != token ||
		r.Kind == Failed && !r.Query.Walkable(goal.X, goal.Y) {
		return Result{}, false
	}
	return s.Result(i), true
}

// ReachCheck is a value-based region pre-check. The application owner
// performs any sampled exact-path assertion on a failed result.
type ReachCheck struct {
	Map   *region.Map
	Layer region.Layer
	From  pathing.Point
}

func (r ReachCheck) OK(x, y int) bool {
	return r.Map == nil || r.Map.Reach(r.Layer, r.From.X, r.From.Y, x, y)
}

// PlanSearch chooses the exact or region-corridor search for a query.
func PlanSearch(q pathing.Query, sx, sy, tx, ty, longPath int, noRegions, exactLong, startInBounds bool, layer region.Layer, regions func() *region.Map, checkNoCorridor func()) (Kind, []uint64) {
	switch {
	case pathing.Octile(sx, sy, tx, ty) <= longPath || noRegions || exactLong:
		return Exact, nil
	case !q.Walkable(tx, ty) || !startInBounds:
		return Failed, nil
	case sx == tx && sy == ty:
		return Here, nil
	}
	allow, ok := regions().Corridor(layer, sx, sy, tx, ty)
	if !ok {
		checkNoCorridor()
		return Failed, nil
	}
	return Corridor, allow
}
