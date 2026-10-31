package herd

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Feed is handlers feeding downed tame animals (FOOD-3; the vanilla
// "feed animal patients" handling work, like care's feeding of downed
// colonists). A downed tame animal below animal.FeedStart is due; a
// handler fetches food it eats (animal feed first, meals only when there
// is none), enough to lift it to animal.FeedStop, walks beside it and
// feeds it. The band is wide (fed at 40 %, filled to 95 %), so it is not
// fed again before it has grown hungry again; a run that finds no food
// leaves it alone for FeedRetry.
type Feed struct {
	h     Host
	retry map[entity.EntityID]uint64
}

// FeedRetry: an animal no food could be found for waits this long
// (original tuning, as care.FeedRetry).
const FeedRetry = 1000

// Due reports a downed tame animal hungry enough to be fed.
func Due(a *animal.State) bool {
	return a.Tame && a.Downed && !a.Dead() && a.Food < animal.FeedStart && !animal.NeedsFree(a.Species)
}

// feedable is food diet d eats at tier t.
func feedable(d animal.Diet, t animal.Tier) func(item.Kind) bool {
	return func(k item.Kind) bool { return animal.FeedValue(d, k) > 0 && animal.FoodTier(k) == t }
}

// Assign sends handler id to feed the nearest due animal no one holds.
func (f *Feed) Assign(id entity.EntityID) bool {
	h := f.h
	now := h.Now()
	if !h.AnyAnimal(func(_ entity.EntityID, a *animal.State) bool { return Due(a) }) {
		return false
	}
	aid, ax, ay, ok := h.Nearest(id, func(o entity.EntityID, a *animal.State) bool {
		return Due(a) && !claimed(h, o) && now >= f.retry[o]
	})
	if !ok {
		return false
	}
	a, _ := h.Animal(aid)
	diet := animal.EcologyOf(a.Species).Diet
	food, fx, fy := h.NearestFood(id, feedable(diet, animal.TierFeed))
	if food == 0 {
		food, fx, fy = h.NearestFood(id, feedable(diet, animal.TierMeal))
	}
	if _, _, beside := h.BesideSpot(id, ax, ay); food == 0 || !beside {
		f.failed(aid, now)
		return false
	}
	return h.StartJob(id, job.Record{Kind: job.FeedPatient, Target: aid, Dest: food, TX: fx, TY: fy})
}

// failed holds animal aid back for FeedRetry.
func (f *Feed) failed(aid entity.EntityID, now uint64) {
	if f.retry == nil {
		f.retry = make(map[entity.EntityID]uint64)
	}
	f.retry[aid] = now + FeedRetry
}

// portions is how many of food kind k lift animal a to FeedStop.
func portions(h Host, a animal.State, k item.Kind) int {
	per := animal.Meal(a.Species, animal.FeedValue(animal.EcologyOf(a.Species).Diet, k))
	if !grown(a, h.Now()) {
		per *= animal.YoungMeal
	}
	if per <= 0 {
		return 1
	}
	return max((animal.FeedStop-a.Food+per-1)/per, 1)
}

// Do runs one tick of feeding job j: fetch the food, walk beside the
// animal and feed it.
func (f *Feed) Do(id entity.EntityID, j job.Record) {
	h := f.h
	a, ok := h.Animal(j.Target)
	if !ok || !a.Tame || !a.Downed {
		done(h, id, j)
		return
	}
	food, _ := h.Stack(j.Dest)
	var there bool
	if j, there = fetch(h, id, j.Target, j, portions(h, a, food.Kind)); !there {
		return
	}
	st, ok := h.Stack(j.Carry)
	h.Delete(j.Carry)
	if ok {
		eat(h, &a, st)
		h.SetAnimal(j.Target, a)
		h.FedAnimal(id, j.Target)
	}
	delete(f.retry, j.Target)
	j.Carry = 0
	done(h, id, j)
}
