package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Animal products (SIM-026, plan 4.4). A grown producer's Product fills
// in tickLife (animal_life.go): animal.ProductPerDay, full at
// ProductFull. Handlers (Handle work) collect it: the giver picks the
// tame animal with a full product nearest the handler, reserves it (it
// then holds still, beingTamed), walks beside it and works CollectWork
// ticks; the yield (LifeOf.ProductYield, by body size) drops on the
// animal's tile, Product goes back to 0 and haulers take it from there.
// Milk and ember oil are collected with job.Milk, wool with job.Shear.

// productJob is the job that collects product p.
func productJob(p animal.Product) job.Kind {
	if p == animal.Wool {
		return job.Shear
	}
	return job.Milk
}

// collectReady: a tame animal whose product is full and free to collect.
func (g *Game) collectReady(aid entity.EntityID, a *animal.State, now uint64) bool {
	if !a.Tame || a.Leaving || a.Enraged(now) || a.Product < animal.ProductFull ||
		!animal.Produces(a.Species, a.Female, g.grown(*a, now)) {
		return false
	}
	_, taken := g.reserve.EntityOwner(aid)
	return !taken
}

// assignCollect sends a handler to milk or shear the nearest tame animal
// with a full product (the creature index, nearest first).
func (g *Game) assignCollect(id entity.EntityID) bool {
	now := uint64(g.clock.Now())
	if !g.anyAnimal(func(o entity.EntityID, a *animal.State) bool { return g.collectReady(o, a, now) }) {
		return false // jobindex.go: no product ready anywhere
	}
	pos, _ := g.positions.Get(id)
	r := max(g.grid.Width(), g.grid.Height())
	best, bp, ok := g.nearestWhere(pos.X, pos.Y, r, ckAnimal, func(o entity.EntityID, _ Position) bool {
		a := g.animals.Ref(o)
		return a != nil && g.collectReady(o, a, now)
	})
	if !ok {
		return false
	}
	a, _ := g.animals.Get(best)
	tx, ty, ok := g.besideSpot(id, bp.X, bp.Y)
	if !ok {
		return false
	}
	kind := productJob(animal.EcologyOf(a.Species).Product)
	return g.startJob(id, job.Record{Kind: kind, Target: best, TX: tx, TY: ty, WorkLeft: animal.CollectWork})
}

// doCollect works a milking or shearing job beside the animal.
func (g *Game) doCollect(id entity.EntityID, j job.Record) {
	a, ok := g.animals.Get(j.Target)
	pos, _ := g.positions.Get(id)
	ap, _ := g.positions.Get(j.Target)
	switch {
	case !ok || !a.Tame || a.Product < animal.ProductFull:
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	case weapon.Distance(pos.X, pos.Y, ap.X, ap.Y) > 1:
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	u := g.workProgress(id, skAnimals, true) // skills.go
	if u == 0 {
		return
	}
	if j.WorkLeft -= u; j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	p := animal.EcologyOf(a.Species).Product
	if sid, err := g.SpawnStack(p.Item(), g.productYield(id, animal.LifeOf(a.Species).ProductYield), ap.X, ap.Y); err == nil {
		st, _ := g.stacks.Get(sid)
		if p == animal.Wool {
			st.Animal = uint8(a.Species) // wool keeps its animal's colour
		}
		g.mergeOrSet(sid, st)
	}
	a.Product = 0
	g.animals.Set(j.Target, a)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// collectVerb is the job line verb of a milking or shearing job ("collect
// ember oil" for an emberbuck).
func (g *Game) collectVerb(j job.Record) string {
	if a, ok := g.animals.Get(j.Target); ok && animal.EcologyOf(a.Species).Product == animal.EmberOil {
		return "collect ember oil"
	}
	return j.Kind.String()
}
