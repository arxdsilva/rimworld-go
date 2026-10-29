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
	p        planner
	buf      planBuf // the ingredient search's scratch (the tick path)
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
			if !m.Enabled(d.Skill) || !s.workerOK(id, bl, d) {
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
			if bl.SearchAt > h.Now() {
				continue // PRD-1: nothing to gather lately, retried after a cooldown
			}
			if !spotOK {
				_, spot, _ = construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
				spotOK = true
			}
			rec, ok := s.plan(id, bl, d, spot)
			if !ok {
				s.bills.list[bid][i].SearchAt = h.Now() + IngredientRetry
				continue
			}
			if unreach || !h.Reachable(id, spot[0], spot[1]) {
				if !unreach { // SIM-027: searched once, put off
					h.Postpone(id, bid)
				}
				unreach = true
				continue
			}
			rec.Dest, rec.DestX, rec.DestY, rec.WorkLeft, rec.BillIndex = bid, spot[0], spot[1], d.Work, i
			if s.start(id, rec) {
				return true
			}
		}
	}
	return false
}

// workerOK reports pawn id may take bill bl: the bill's own worker if it
// names one, with the recipe's minimum skill and inside the bill's range.
func (s *Shop) workerOK(id entity.EntityID, bl recipe.Bill, d recipe.Def) bool {
	if bl.Worker != 0 && bl.Worker != id {
		return false
	}
	if d.MinSkill == 0 && bl.SkillMin == 0 && bl.SkillMax == 0 {
		return true
	}
	lvl := s.h.SkillLevel(id, d.Learn)
	return lvl >= d.MinSkill && bl.SkillOK(lvl)
}

// plan finds bill bl's ingredients for pawn id: the nearest single stack
// as ever (a plain bill job) when it will do, else stacks gathered from
// around the work spot (a gather-ingredients job, PRD-1).
func (s *Shop) plan(id entity.EntityID, bl recipe.Bill, d recipe.Def, spot [2]int) (job.Record, bool) {
	if len(d.Inputs) == 0 {
		if sid, st := s.h.Ingredient(id, d.Input); sid != 0 && fits(bl, st, spot) {
			return job.Record{Kind: job.Bill, Target: sid, TX: st.X, TY: st.Y}, true
		}
	}
	picks, ok := s.p.plan(id, bl, d, spot, &s.buf)
	if !ok {
		return job.Record{}, false
	}
	if len(picks) == 1 && len(d.Inputs) == 0 {
		return job.Record{Kind: job.Bill, Target: picks[0].id, TX: picks[0].x, TY: picks[0].y}, true
	}
	route := make([]entity.EntityID, 0, 2*len(picks))
	for _, p := range picks {
		route = append(route, p.id, entity.EntityID(p.take))
	}
	return job.Record{Kind: job.HaulIngredients, Target: picks[0].id, TX: picks[0].x, TY: picks[0].y,
		Route: route, Count: len(route)}, true
}

// start starts bill job rec for pawn id and reserves the bench and every
// stack a gathering takes from. A gathering that can't reach its first
// stack puts the bench off (SIM-027), as a plain bill job's start does.
func (s *Shop) start(id entity.EntityID, rec job.Record) bool {
	h := s.h
	if !h.StartJob(id, rec) {
		if j, _ := h.Job(id); rec.Kind == job.HaulIngredients && j.Fail == job.Unreachable {
			h.Postpone(id, rec.Dest)
		}
		return false
	}
	h.Claim(id, rec.Dest)
	for i := 2; i < rec.Count; i += 2 {
		h.Claim(id, rec.Route[i])
	}
	return true
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
		if !ok || j.Kind != job.Bill && j.Kind != job.HaulIngredients || j.State == job.Complete || j.State == job.Fail {
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

// Do runs one tick of pawn id's bill job j: gathering ingredients
// (job.HaulIngredients, haul.go), fetching the one ingredient, working.
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
	if j.Kind == job.HaulIngredients {
		s.gather(id, j, b)
		return
	}
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
	if !s.laidOut(j) {
		s.fail(id, j, job.TargetGone) // an ingredient left the bench
		return
	}
	stored := s.finish(id, j, b, d)
	j.Carry, j.Route = 0, nil
	bl.Done++
	bills[j.BillIndex] = bl
	s.bills.list[j.Dest] = bills
	j.State = job.Complete
	h.SetJob(id, j)
	if stored {
		s.store(id, j, bl, d)
	}
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

// finish turns the carried ingredient (and those laid on the bench) into
// the bill's product: a batch on the rack, a cremation, or the output
// (and extra) on the work spot. stored: an output was dropped there for
// the bill's store mode to deal with.
func (s *Shop) finish(id entity.EntityID, j job.Record, b construction.Building, d recipe.Def) (stored bool) {
	h := s.h
	switch {
	case d.Dry:
		if st, ok := h.Stack(j.Carry); ok {
			s.drying.racks[j.Dest] = Batch{Kind: st.Kind, Qty: st.Qty, Rot: st.Rot, Left: st.Kind.DryTicks()}
		}
		h.DeleteItem(j.Carry)
		return false
	case d.ID == recipe.Cremate:
		h.Cremate(j.Dest, j.Carry, d) // burns the body and its wood
		return false
	}
	for _, rid := range j.Route { // PRD-1: the other ingredients, laid on the bench
		h.DeleteItem(rid)
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
	return out > 0
}
