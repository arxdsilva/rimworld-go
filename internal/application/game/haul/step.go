package haul

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Gather drives the pickups of a stockpile haul (HAUL-002): with its
// first pile in hand, the hauler walks to each pile left in the job's
// Pickups, adds it to the stack it carries (rot and poison averaged by
// quantity) and only then walks on to its storage cell. The planned count
// of every pile is claimed for the hauler when the job starts; each claim
// is released as the pile is taken or skipped (the game releases the
// first pile's once the load is split off it, RES-1). Interrupts need nothing here: the game
// drops the carried load and frees the hauler's claims.
type Gather struct{ h Host }

// NewGather is a Gather over host h.
func NewGather(h Host) Gather { return Gather{h} }

// Next walks hauler id on to the next pile left in j that still joins
// its load, taking at once any pile it stands on, and skips piles that
// are gone, taken, changed or out of reach. It reports whether the hauler
// walks to a pile (j then heads there, State Goto); false means every
// pile is done (PickupI at the end) and the load goes to storage.
func (g Gather) Next(id entity.EntityID, j *job.Record) bool {
	for j.PickupI+1 < len(j.Pickups) {
		sid := j.Pickups[j.PickupI]
		if st, ok := g.joins(j, sid); ok {
			path, reach := g.h.PathTo(id, st.X, st.Y)
			if reach && len(path) > 0 {
				j.TX, j.TY, j.Path, j.PathI, j.State = st.X, st.Y, path, 0, job.Goto
				return true
			}
			if reach {
				g.take(j, sid, st)
			}
		}
		g.h.Release(id, sid)
		j.PickupI += 2
	}
	j.PickupI = len(j.Pickups)
	return false
}

// Arrive takes the pile hauler id walked to into its load, then walks on
// (Next, whose result it returns).
func (g Gather) Arrive(id entity.EntityID, j *job.Record) bool {
	if j.PickupI+1 < len(j.Pickups) {
		sid := j.Pickups[j.PickupI]
		if st, ok := g.joins(j, sid); ok {
			g.take(j, sid, st)
		}
		g.h.Release(id, sid)
		j.PickupI += 2
	}
	return g.Next(id, j)
}

// joins is stack sid when it still lies free and joins j's load, which
// still has room (j.Count caps it; 0: no cap).
func (g Gather) joins(j *job.Record, sid entity.EntityID) (item.Stack, bool) {
	c, cok := g.h.Stack(j.Carry)
	st, ok := g.h.Stack(sid)
	ok = ok && cok && sid != j.Carry && st.Qty > 0 && st.Carrier == 0 && !st.Forbidden && item.SameLoad(c, st)
	return st, ok && (j.Count <= 0 || c.Qty < j.Count)
}

// take moves the planned count of pile sid (stack st) into j's load: the
// whole pile goes, or the rest stays behind.
func (g Gather) take(j *job.Record, sid entity.EntityID, st item.Stack) {
	c, _ := g.h.Stack(j.Carry)
	n := min(int(j.Pickups[j.PickupI+1]), st.Qty)
	if j.Count > 0 {
		n = min(n, j.Count-c.Qty)
	}
	c = item.MergeFood(c, st, n) // rot, poison, FOOD-4 ingredients
	c.Qty += n
	g.h.SetStack(j.Carry, c)
	if n >= st.Qty {
		g.h.DeleteItem(sid)
		return
	}
	st.Qty -= n
	g.h.SetStack(sid, st)
}
