package herd

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Taming (FOOD-3, wiki: Animal husbandry, Wildness, Minimum handling
// skill, Tame animal chance). A handler with at least the animal's
// minimum handling skill fetches food it eats (raw food or kibble, never
// hay or meals: animal.TameFeed), TameFood's worth, walks to the marked
// animal, which then holds still, coaxes it for TameWork and feeds it the
// food; the attempt succeeds at animal.TameChance (doubled for a downed
// animal, too weak to resist: original tuning). A failed animal waits out
// its retry (and may go manhunter at its species' revenge chance).
type Taming struct{ h Host }

// downedTameMul multiplies the chance of taming a downed animal (original
// tuning).
const downedTameMul = 2

// Chance is the permille chance handler level l tames animal a: the
// wiki's chance, doubled when it is downed (at most certain).
func Chance(l int, a animal.State) int {
	c := animal.TameChance(l, animal.Wildness(a.Species))
	if a.Downed {
		c *= downedTameMul
	}
	return min(c, 1000)
}

// Tameable reports an animal handler level l may try to tame now: marked,
// wild, calm, past its retry, of a species that is ever tamed, and not
// beyond the handler's skill.
func Tameable(a *animal.State, l int, now uint64) bool {
	return a.TameMark && !a.Tame && !a.Enraged(now) && now >= a.TameRetry && animal.DefOf(a.Species).TameChance > 0 &&
		l >= animal.MinHandling(animal.Wildness(a.Species))
}

// foodQty is how many of stack kind k make TameFood for species s.
func foodQty(s animal.Species, k item.Kind) int {
	n := max(k.Nutrition(), 1)
	return max((animal.TameFood(s)+n-1)/n, 1)
}

// Assign sends handler id to tame the nearest animal it may, by way of the
// nearest food the animal eats; false when there is none, no food, or no
// way.
func (t *Taming) Assign(id entity.EntityID) bool {
	h := t.h
	now, l := h.Now(), h.Animals(id)
	if !h.AnyAnimal(func(_ entity.EntityID, a *animal.State) bool { return a.TameMark && !a.Tame }) {
		return false // nothing marked for taming (the job index)
	}
	aid, ax, ay, ok := h.Nearest(id, func(o entity.EntityID, a *animal.State) bool {
		return Tameable(a, l, now) && !claimed(h, o)
	})
	if !ok {
		return false
	}
	a, _ := h.Animal(aid)
	diet := animal.EcologyOf(a.Species).Diet
	food, fx, fy := h.NearestFood(id, func(k item.Kind) bool { return animal.TameFeed(diet, k) })
	if food == 0 {
		return false
	}
	if _, _, ok := h.BesideSpot(id, ax, ay); !ok {
		return false
	}
	return h.StartJob(id, job.Record{Kind: job.Tame, Target: aid, Dest: food, TX: fx, TY: fy, WorkLeft: animal.TameWork})
}

// Do runs one tick of taming job j: fetch the food, walk beside the
// animal, coax it, feed it and roll.
func (t *Taming) Do(id entity.EntityID, j job.Record) {
	h := t.h
	a, ok := h.Animal(j.Target)
	if !ok || a.Tame || !a.TameMark {
		done(h, id, j)
		return
	}
	if j.Dest != 0 {
		var there bool
		food, _ := h.Stack(j.Dest)
		if j.Carry != 0 {
			food, _ = h.Stack(j.Carry)
		}
		if j, there = fetch(h, id, j.Target, j, foodQty(a.Species, food.Kind)); !there {
			return
		}
	} else if !adjacent(h, id, j.Target) {
		fail(h, id, j, job.TargetGone)
		return
	}
	u := h.Work(id)
	if u == 0 {
		return
	}
	if j.WorkLeft -= u; j.WorkLeft > 0 {
		h.SetJob(id, j)
		return
	}
	t.feed(id, j)
	a, _ = h.Animal(j.Target)
	if h.Roll(1000) < Chance(h.Animals(id), a) {
		h.Tamed(id, j.Target)
	} else {
		h.TameFailed(id, j.Target)
	}
	j.Carry = 0
	done(h, id, j)
}

// feed gives the carried food to the animal: it eats it (its food rises
// by the meal, a young one's twice as far).
func (t *Taming) feed(id entity.EntityID, j job.Record) {
	if j.Carry == 0 {
		return
	}
	h := t.h
	st, ok := h.Stack(j.Carry)
	h.Delete(j.Carry)
	a, aok := h.Animal(j.Target)
	if !ok || !aok {
		return
	}
	eat(h, &a, st)
	h.SetAnimal(j.Target, a)
}

// eat feeds stack st to animal a: its food rises by the meal (a young
// one's twice as far), at most full.
func eat(h Host, a *animal.State, st item.Stack) {
	v := animal.Meal(a.Species, animal.FeedValue(animal.EcologyOf(a.Species).Diet, st.Kind)*st.Qty)
	if !grown(*a, h.Now()) {
		v *= animal.YoungMeal
	}
	a.Food = min(a.Food+v, animal.Full)
	if a.Food > 0 {
		a.Starving = 0
	}
}
