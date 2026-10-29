package craft

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

// A gather-ingredients job (job.HaulIngredients, PRD-1) keeps its plan in
// the job record, so it saves with the job: Route holds two entries per
// stack to take from, the stack's id then the count to take, Count is the
// length of those entries and RouteI the next one; the loads laid on the
// bench are appended after them. Target is where the pawn is heading: the
// next stack, or the bench (Dest) to lay a load down. Every stack is
// reserved for the pawn while the job runs.

// gather runs one tick of gather-ingredients job j at bench b, on
// arriving: at a stack it picks the planned count up (into the load it
// carries), at the bench it lays the load down. Then it walks on to the
// next stack, back to the bench to lay a load of another kind down, or,
// with everything gathered, to the work spot as a plain bill job carrying
// the last load (the others lie on the bench, listed in Route).
func (s *Shop) gather(id entity.EntityID, j job.Record, b construction.Building) {
	h := s.h
	switch {
	case j.Target == j.Dest && j.Carry != 0:
		h.PutDown(j.Carry, b.X, b.Y)
		h.Claim(id, j.Carry)
		j.Route = append(j.Route[:len(j.Route):len(j.Route)], j.Carry) // a fresh array: copies never share it
		j.Carry = 0
	case j.Target != j.Dest && !s.pickUp(id, &j):
		s.fail(id, j, job.TargetGone)
		return
	}
	tx, ty := j.DestX, j.DestY
	j.Target = j.Dest
	if j.RouteI < j.Count {
		next := j.Route[j.RouteI]
		st, ok := h.Stack(next)
		if !ok {
			s.fail(id, j, job.TargetGone)
			return
		}
		if j.Carry == 0 || s.joins(j.Carry, st, int(j.Route[j.RouteI+1])) {
			j.Target, tx, ty = next, st.X, st.Y
		}
	} else {
		j.Kind = job.Bill
		j.Route = slices.Clone(j.Route[j.Count:])
		j.Count, j.RouteI = 0, 0
	}
	path, ok := h.PathTo(id, tx, ty)
	if !ok {
		h.Postpone(id, j.Dest) // SIM-027: put the bench off a while
		s.fail(id, j, job.Unreachable)
		return
	}
	j.TX, j.TY = tx, ty
	j.Path, j.PathI = path, 0
	if len(path) > 0 {
		j.State = job.Goto
	}
	h.SetJob(id, j)
}

// pickUp takes the planned count of the next stack on j's route into the
// load pawn id carries; false when the stack is gone, short, taken, or
// won't join the load.
func (s *Shop) pickUp(id entity.EntityID, j *job.Record) bool {
	h := s.h
	if j.RouteI+1 >= j.Count {
		return false
	}
	sid, take := j.Route[j.RouteI], int(j.Route[j.RouteI+1])
	st, ok := h.Stack(sid)
	if !ok || st.Carrier != 0 || st.Qty < take || j.Carry != 0 && !s.joins(j.Carry, st, take) {
		return false
	}
	c := h.Carry(id, sid, take)
	if j.Carry == 0 {
		j.Carry = c
	} else {
		h.Merge(j.Carry, c)
	}
	j.RouteI += 2
	return true
}

// joins reports take items of stack st can join carried load carry: the
// same kind in the same state, within what one pawn carries.
func (s *Shop) joins(carry entity.EntityID, st item.Stack, take int) bool {
	c, ok := s.h.Stack(carry)
	return ok && c.Kind == st.Kind && c.Rotten() == st.Rotten() && c.Dried == st.Dried && c.Of == st.Of &&
		c.Animal == st.Animal && c.Qty+take <= item.CarryLimit
}

// laidOut reports every ingredient bill job j laid on the bench is still
// lying there.
func (s *Shop) laidOut(j job.Record) bool {
	for _, rid := range j.Route {
		if st, ok := s.h.Stack(rid); !ok || st.Carrier != 0 {
			return false
		}
	}
	return true
}

// store sends the product of bill bl, just dropped on the work spot,
// where the bill's store mode says: pawn id carries it to the best
// stockpile or to the bill's stockpile; dropped on the floor it stays
// for the haulers (as does a product no stockpile has room for).
func (s *Shop) store(id entity.EntityID, j job.Record, bl recipe.Bill, d recipe.Def) {
	switch {
	case bl.Store == recipe.BestStockpile:
		s.h.Store(id, d.Output, j.DestX, j.DestY, 0)
	case bl.Store == recipe.StockpileZone && bl.Zone != 0:
		s.h.Store(id, d.Output, j.DestX, j.DestY, bl.Zone)
	}
}
