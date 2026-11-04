package craft

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/entity"
)

// A pack gathering (job.PackIngredients, CSE-029…031, 040) walks a bill's stacks
// into the pawn's pack, nearest next, and unpacks them at the bench, so one trip
// does what a hand load needs several for. Its plan lives in the job record and
// saves with it, as a hand gathering's does (haul.go):
//
//   - Route[:Count] holds a pair per pick. From RouteI on: the stack, the count
//     still to take. Before RouteI: the pack entry and count stowed, or 0, 0 once
//     laid on the bench.
//   - Route[Count:] lists the loads laid on the bench.
//   - SpareX is the recipe: bills can be reordered or removed during the walk.
//   - Target is where it heads: the next stack, or the bench (Dest).
//
// What is stowed stays in the pack if the job ends early: marked to go to storage
// (Packs.Stow), and so never lost with the job.

// packable reports pawn id takes picks in its pack: a colonist under the rule,
// with room for one of each, and no corpse among them (no pack takes one). Else
// the hand gathering does the bill.
func (s *Shop) packable(id entity.EntityID, picks []pick) bool {
	h := s.h
	if !h.Rule(gameplay.HaulAllIngredients) || h.Name(id) == "" {
		return false
	}
	room := h.PackRoom(id)
	for _, p := range picks {
		if p.kind.IsCorpse() || p.kind.MassGrams() > room {
			return false
		}
	}
	return true
}

// orderPicks orders picks by greedy nearest neighbour from (x, y), ties by id
// (CSE-031).
func orderPicks(picks []pick, x, y int) {
	for i := range picks {
		best := i
		bestD := pathing.Dist(x, y, picks[i].x, picks[i].y)
		for k := i + 1; k < len(picks); k++ {
			d := pathing.Dist(x, y, picks[k].x, picks[k].y)
			if c := cmp.Compare(d, bestD); c < 0 || c == 0 && picks[k].id < picks[best].id {
				best, bestD = k, d
			}
		}
		picks[i], picks[best] = picks[best], picks[i]
		x, y = picks[i].x, picks[i].y
	}
}

// billOf is the index in bills of the recipe job j gathers for (len(bills): it
// has gone): where j says if it is still there, else the first bill of it.
func billOf(j job.Record, bills []recipe.Bill) int {
	if j.BillIndex < len(bills) && int(bills[j.BillIndex].Recipe) == j.SpareX {
		return j.BillIndex
	}
	if i := slices.IndexFunc(bills, func(b recipe.Bill) bool { return int(b.Recipe) == j.SpareX }); i >= 0 {
		return i
	}
	return len(bills)
}

// packGather runs one tick of pack gathering j at bench b, on arriving: at a
// stack it stows what fits of the pick, at the bench it unpacks. Then it walks on.
func (s *Shop) packGather(id entity.EntityID, j job.Record, b construction.Building) {
	atBench := j.Target == j.Dest
	if atBench && !s.unpack(id, &j, b) || !atBench && !s.stowPick(id, &j) {
		s.fail(id, j, job.TargetGone)
		return
	}
	h := s.h
	tx, ty := j.DestX, j.DestY
	j.Target = j.Dest
	if j.RouteI < j.Count {
		st, ok := h.Stack(j.Route[j.RouteI])
		switch {
		case !ok:
			s.fail(id, j, job.TargetGone)
			return
		case s.fits(id, st.Kind) > 0:
			j.Target, tx, ty = j.Route[j.RouteI], st.X, st.Y
		case atBench: // the pack is as empty as it gets (hurt hands, say): by hand next time (packable)
			s.fail(id, j, job.Blocked)
			return
		}
	} else if atBench {
		toBill(&j)
		j.SpareX = 0
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

// fits is how many of kind k pawn id's pack still takes (CSE-030).
func (s *Shop) fits(id entity.EntityID, k item.Kind) int {
	if g := k.MassGrams(); g > 0 {
		return s.h.PackRoom(id) / g
	}
	return item.CarryLimit
}

// stowPick moves what fits of the pick at RouteI into pawn id's pack; false when
// the stack is gone, short or taken. What doesn't fit stays a pick, still claimed,
// for the next trip; nothing fitting at all sends the pawn back with what it has.
func (s *Shop) stowPick(id entity.EntityID, j *job.Record) bool {
	h := s.h
	sid, take := j.Route[j.RouteI], int(j.Route[j.RouteI+1])
	st, ok := h.Stack(sid)
	if !ok || st.Carrier != 0 || st.Qty < take {
		return false
	}
	n := min(take, s.fits(id, st.Kind))
	if n <= 0 {
		return true
	}
	entry, ok := h.Stow(id, sid, n)
	if !ok {
		return false
	}
	route := slices.Clone(j.Route)
	if n < take {
		route = slices.Insert(route, j.RouteI, 0, 0)
		route[j.RouteI+3] = entity.EntityID(take - n)
		j.Count += 2
		h.Claims().TryCount(id, sid, take-n, st.Qty-n)
	} else {
		h.Claims().ReleaseEntity(id, sid) // RES-1: stowed, the rest is free for other crafters
	}
	route[j.RouteI], route[j.RouteI+1] = entity.EntityID(entry), entity.EntityID(n)
	j.Route = route
	j.RouteI += 2
	return true
}

// unpack takes everything stowed out of pawn id's pack at bench b, the planned
// count of each entry, one load per kind in a row (as a hand gathering's), and
// lays each on the bench; the last stays in hand when no pick is left. False: an
// entry rotted away or came up short, with what it did take out in hand, so the
// job's end drops it.
func (s *Shop) unpack(id entity.EntityID, j *job.Record, b construction.Building) bool {
	h := s.h
	defer h.Settled(id)
	j.Route = slices.Clone(j.Route)
	for i := 0; i < j.RouteI; i += 2 {
		n := int(j.Route[i+1])
		if n == 0 {
			continue
		}
		c, ok := h.Unstow(id, uint32(j.Route[i]), n)
		if !ok {
			return false
		}
		j.Route[i], j.Route[i+1] = 0, 0
		st, _ := h.Stack(c)
		switch {
		case j.Carry == 0:
			j.Carry = c
		case s.joins(j.Carry, st, st.Qty):
			merge(h, j.Carry, c)
		default:
			s.lay(id, j, b)
			j.Carry = c
		}
		if st.Qty < n {
			return false
		}
	}
	if j.RouteI < j.Count && j.Carry != 0 {
		s.lay(id, j, b)
	}
	return true
}
