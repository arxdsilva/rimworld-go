package herd

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Collect is milking and shearing (SIM-026, moved from the game's
// animal_products.go by FOOD-3). A grown producer's Product fills in the
// game's tickLife (animal.ProductPerDay, the species' wiki interval);
// handlers collect it: the nearest tame animal with a full product,
// reserved (it then holds still), worked CollectWork beside it. The
// species' wiki amount drops on its tile with the handler's animal gather
// yield as the chance (wiki: Animal gather yield; otherwise the product
// is wasted), Product goes back to 0 and haulers take it from there.
// Milk and ember oil are collected with job.Milk, wool with job.Shear.
type Collect struct{ h Host }

// ProductJob is the job that collects product p.
func ProductJob(p animal.Product) job.Kind {
	if p == animal.Wool {
		return job.Shear
	}
	return job.Milk
}

// Ready reports a tame animal whose product is full and free to collect.
func Ready(h Host, aid entity.EntityID, a *animal.State, now uint64) bool {
	if !a.Tame || a.Leaving || a.Enraged(now) || a.Product < animal.ProductFull ||
		!animal.Produces(a.Species, a.Female, grown(*a, now)) {
		return false
	}
	return !claimed(h, aid)
}

// Assign sends handler id to milk or shear the nearest tame animal with a
// full product.
func (c *Collect) Assign(id entity.EntityID) bool {
	h := c.h
	now := h.Now()
	keep := func(o entity.EntityID, a *animal.State) bool { return Ready(h, o, a, now) }
	if !h.AnyAnimal(keep) {
		return false // no product ready anywhere (the job index)
	}
	aid, ax, ay, ok := h.Nearest(id, keep)
	if !ok {
		return false
	}
	a, _ := h.Animal(aid)
	tx, ty, ok := h.BesideSpot(id, ax, ay)
	if !ok {
		return false
	}
	return h.StartJob(id, job.Record{Kind: ProductJob(animal.EcologyOf(a.Species).Product), Target: aid, TX: tx, TY: ty, WorkLeft: animal.CollectWork})
}

// Do works a milking or shearing job beside the animal.
func (c *Collect) Do(id entity.EntityID, j job.Record) {
	h := c.h
	a, ok := h.Animal(j.Target)
	switch {
	case !ok || !a.Tame || a.Product < animal.ProductFull:
		done(h, id, j)
		return
	case !adjacent(h, id, j.Target):
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
	p := animal.EcologyOf(a.Species).Product
	ax, ay, _ := h.Pos(j.Target)
	if h.Roll(1000) < animal.GatherYield(h.Animals(id)) {
		st := item.Stack{Kind: p.Item(), Qty: animal.LifeOf(a.Species).ProductYield, X: ax, Y: ay}
		if p == animal.Wool {
			st.Animal = uint8(a.Species) // wool keeps its animal's colour
		}
		h.Drop(st)
	} else {
		h.Message(ax, ay, fmt.Sprintf("%s wasted the %s of the %s.", h.Name(id), p, a.Species.Label()))
	}
	a.Product = 0
	h.SetAnimal(j.Target, a)
	done(h, id, j)
}
