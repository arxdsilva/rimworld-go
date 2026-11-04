package sense

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/inventory"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// CSE-005: a colonist with time to spare keeps one meal in its pack, and eats
// it when hungry and nothing else is at hand. It takes one when its own food
// bar is above 30 % (the mod's gate), which is well above the hungry band, and
// waits spareGap after every look and every meal, so taking, eating and taking
// again can't follow each other at once.

const (
	// spareFoodMin is the food bar above which a colonist stocks a meal: 30 %.
	spareFoodMin = needs.Full * 30 / 100
	// spareGap is the wait after a failed search, a meal taken or eaten (ours).
	spareGap = 2500
)

// IsMeal reports a kind that is eaten as it is: what a spare is.
func IsMeal(k item.Kind) bool {
	return k == item.Meal || k == item.FineMeal || k == item.LavishMeal || k == item.SurvivalMeal
}

// eatFromPack sends a hungry colonist to eat a meal from its pack, in place.
func (g Gear) eatFromPack(id entity.EntityID, p inventory.Pack) bool {
	if !g.s.Rules.On(gameplay.SpareMeal) || !g.r.Hungry(id) {
		return false
	}
	for _, e := range p.Items {
		if !IsMeal(e.Stack.Kind) || !g.r.Allows(id, e.Stack.Kind) {
			continue
		}
		stack, ok := g.TakeOut(id, e.ID, 1)
		if !ok {
			return false
		}
		x, y := g.r.Pos(id)
		if g.r.StartJob(id, job.Record{Kind: job.Eat, Target: stack, TX: x, TY: y, WorkLeft: needs.EatWorkTicks, Count: 2}) {
			j := g.r.Job(id)
			j.Carry = stack // set down uneaten, it goes back (BackToPack)
			g.r.SetJob(id, j)
			g.s.SpareAt[id] = g.r.Now() + spareGap
			return true
		}
		g.BackToPack(id, stack)
		g.r.SetJob(id, job.Record{})
		return false
	}
	return false
}

// takeSpare sends a colonist in free time to fetch one meal for its pack.
func (g Gear) takeSpare(id entity.EntityID, p inventory.Pack) bool {
	now := g.r.Now()
	if now < g.s.SpareAt[id] || !g.s.Rules.On(gameplay.SpareMeal) || hasMeal(p) ||
		g.r.Hunger(id) < spareFoodMin || !g.r.Free(id) {
		return false
	}
	g.s.SpareAt[id] = now + spareGap
	stack, x, y, ok := g.r.NearestMeal(id)
	return ok && g.r.StartJob(id, job.Record{Kind: job.TakeSpare, Target: stack, TX: x, TY: y})
}

func hasMeal(p inventory.Pack) bool {
	for _, e := range p.Items {
		if IsMeal(e.Stack.Kind) {
			return true
		}
	}
	return false
}

// DoSpare runs one tick of job.TakeSpare: the colonist stands at the meal.
func (g Gear) DoSpare(id entity.EntityID, j job.Record) {
	st, ok := g.r.Stack(j.Target)
	if !ok || st.Qty <= 0 || st.Carrier != 0 || !IsMeal(st.Kind) || !g.Stow(id, j.Target, 1) {
		g.end(id, j, false)
		return
	}
	g.end(id, j, true)
}
