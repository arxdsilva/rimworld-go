package craft

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

// Shop finds bill work for a pawn and does it: fetch the ingredient,
// carry it to the bench, work it off, and leave the product.
type Shop struct {
	h        Host
	bills    *Bills
	drying   *Drying
	cabinets *Cabinets
}

// Assign starts the first bill at the first bench, in building order,
// that pawn id may work with work types m and that wants doing.
func (s *Shop) Assign(id entity.EntityID, m work.Mask) bool {
	h := s.h
	for _, bid := range h.BuildingIDs() {
		b, ok := h.Bench(bid) // a bench: copied before anything below can change the store
		if !ok {
			continue
		}
		if owner, taken := h.Owner(bid); taken && owner != id {
			continue
		}
		if h.Postponed(id, bid) {
			continue // SIM-027: blocked or out of reach lately
		}
		var spot [2]int // the work spot, worked out once a bill wants doing
		spotOK, unreach := false, false
		for i, bl := range s.bills.list[bid] {
			d := recipe.DefOf(bl.Recipe)
			if !m.Enabled(d.Skill) || !h.MeetsMinSkill(id, d.Learn, d.MinSkill) {
				continue
			}
			stock := s.bills.stock(d)
			bl = bl.Update(stock)
			s.bills.list[bid][i] = bl
			if !bl.Wants(stock + s.pending(bl, d)) {
				continue
			}
			if d.Dry && s.drying.busy(bid) {
				continue // still drying the last batch
			}
			if b.Kind == construction.Campfire && h.Fuel(bid) <= 0 {
				continue // no fire to cook over
			}
			if ElectricBench(b.Kind) && !h.Powered(bid) {
				continue // PLAY-021: an electric bench works only on power
			}
			if d.ID == recipe.Cremate && h.FreeWood() < d.ExtraQty {
				continue // no wood to burn the body with
			}
			sid, st := h.Ingredient(id, d.Input)
			if sid == 0 {
				continue
			}
			if !spotOK {
				_, spot, _ = construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
				spotOK = true
			}
			if unreach || !h.Reachable(id, spot[0], spot[1]) {
				if !unreach { // SIM-027: searched once, put off
					h.Postpone(id, bid)
				}
				unreach = true
				continue
			}
			rec := job.Record{Kind: job.Bill, Target: sid, TX: st.X, TY: st.Y, Dest: bid,
				DestX: spot[0], DestY: spot[1], WorkLeft: d.Work, BillIndex: i}
			if !h.StartJob(id, rec) {
				continue
			}
			h.Claim(id, bid)
			return true
		}
	}
	return false
}

// pending is the output already being made for an "until you have" bill,
// so two cooks don't both start the last meal.
func (s *Shop) pending(bl recipe.Bill, d recipe.Def) int {
	if bl.Mode != recipe.UntilHave || d.Dry {
		return 0
	}
	n := 0
	for _, pid := range s.h.PawnIDs() {
		j, ok := s.h.Job(pid)
		if !ok || j.Kind != job.Bill || j.State == job.Complete || j.State == job.Fail {
			continue
		}
		if bills := s.bills.list[j.Dest]; j.BillIndex < len(bills) && bills[j.BillIndex].Recipe == bl.Recipe {
			n += d.OutQty
		}
	}
	return n
}

// fail ends pawn id's job j for reason why.
func (s *Shop) fail(id entity.EntityID, j job.Record, why job.FailReason) {
	j.State, j.Fail = job.Fail, why
	s.h.SetJob(id, j)
}

// Do runs one tick of pawn id's bill job j.
func (s *Shop) Do(id entity.EntityID, j job.Record) {
	h := s.h
	b, ok := h.Building(j.Dest)
	bills := s.bills.list[j.Dest]
	if !ok || j.BillIndex >= len(bills) {
		s.fail(id, j, job.TargetGone)
		return
	}
	bl := bills[j.BillIndex]
	d := recipe.DefOf(bl.Recipe)
	if j.Carry == 0 {
		s.fetch(id, j, d)
		return
	}
	if ElectricBench(b.Kind) && !h.Powered(j.Dest) {
		s.fail(id, j, job.Blocked) // PLAY-021: the power went (put off a while: postpone.go)
		return
	}
	u := h.BenchWork(id, d.Learn, 100+s.cabinets.Bonus(j.Dest)+ElectricPct(b.Kind)) // tool cabinets
	if u == 0 {
		h.SetJob(id, j)
		return
	}
	j.WorkLeft -= u
	if j.WorkLeft > 0 {
		h.SetJob(id, j)
		return
	}
	s.finish(id, j, b, d)
	j.Carry = 0
	bl.Done++
	bills[j.BillIndex] = bl
	s.bills.list[j.Dest] = bills
	j.State = job.Complete
	h.SetJob(id, j)
}

// fetch picks the ingredient up (splitting off what the recipe takes) and
// heads for the bench's work spot.
func (s *Shop) fetch(id entity.EntityID, j job.Record, d recipe.Def) {
	h := s.h
	st, ok := h.Stack(j.Target)
	if !ok || st.Carrier != 0 || st.Qty < d.Input.Qty {
		s.fail(id, j, job.TargetGone)
		return
	}
	j.Carry = h.Carry(id, j.Target, d.Input.Take(st.Qty))
	path, ok := h.PathTo(id, j.DestX, j.DestY)
	if !ok {
		s.fail(id, j, job.Unreachable)
		return
	}
	j.TX, j.TY = j.DestX, j.DestY
	j.Path, j.PathI = path, 0
	if len(path) > 0 {
		j.State = job.Goto
	}
	h.SetJob(id, j)
}

// finish turns the carried ingredient into the bill's product: a batch on
// the rack, a cremation, or the output (and extra) on the work spot.
func (s *Shop) finish(id entity.EntityID, j job.Record, b construction.Building, d recipe.Def) {
	h := s.h
	switch {
	case d.Dry:
		if st, ok := h.Stack(j.Carry); ok {
			s.drying.racks[j.Dest] = Batch{Kind: st.Kind, Qty: st.Qty, Rot: st.Rot, Left: st.Kind.DryTicks()}
		}
		h.DeleteItem(j.Carry)
		return
	case d.ID == recipe.Cremate:
		h.Cremate(j.Dest, j.Carry, d) // burns the body and its wood
		return
	}
	out, extra := d.OutQty, d.ExtraQty
	if d.BySpecies { // butchering: the yields come from the carcass's species
		if st, ok := h.Stack(j.Carry); ok {
			out, extra = animal.Yield(animal.Species(st.Animal))
			out, extra = h.ButcherYield(id, out), h.ButcherYield(id, extra)
		}
	}
	h.DeleteItem(j.Carry)
	switch {
	case out > 0 && d.Output == item.Meal: // the cook's poison chance
		h.DropMeal(id, b.Kind, d.Output, out, j.DestX, j.DestY)
	case out > 0:
		h.DropStack(d.Output, out, j.DestX, j.DestY)
	}
	if extra > 0 {
		h.DropStack(d.Extra, extra, j.DestX, j.DestY)
	}
}
