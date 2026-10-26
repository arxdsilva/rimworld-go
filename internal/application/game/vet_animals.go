package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Veterinary care (ENG-003, M10; Doctor work). A doctor tends a hurt tame
// animal like a colonist: with the medicine its care level allows (MED-03;
// No care keeps the vet away), at the doctor's tend quality (MED-02). With
// a body (MED-09, animal_health.go) the tend closes every open wound (it
// stops bleeding and festers less) and treats a due infection, so the vet
// comes back only for a new wound or the infection's next window; without
// one (an older save, starvation) the tend mends Injury at once and holds
// for TendTicks. A tend may bond them (training.BondTend). Tame
// animals that are fed also heal on their own, one point every
// animalHealEvery ticks of their decision cycle (twice as fast while
// tended); wild ones heal only their wounds (animal_health.go).

// animalHealEvery: a fed tame animal heals one point this often (a
// multiple of animal.DecideEvery: it runs on the decision cycle).
const animalHealEvery = 10 * animal.DecideEvery

// needsVet reports whether tame animal id (state a) wants tending now.
func (g *Game) needsVet(id entity.EntityID, a *animal.State, now uint64) bool {
	return a.Tame && !a.Dead() && !a.Enraged(now) && g.animalTendDue(id, a, now) && // animal_health.go
		a.Mode != animal.Flee && a.Mode != animal.Attack && !a.Leaving &&
		g.careOf(id).Tends() // care.go (MED-03): no care keeps the vet away
}

// assignTendAnimal sends doctor id to tend the nearest hurt tame animal.
func (g *Game) assignTendAnimal(id entity.EntityID) bool {
	now := uint64(g.clock.Now())
	if !g.anyAnimal(func(aid entity.EntityID, a *animal.State) bool { return g.needsVet(aid, a, now) }) {
		return false
	}
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	for _, aid := range g.animalIDs {
		if a := g.animals.Ref(aid); a == nil || !g.needsVet(aid, a, now) {
			continue
		}
		if _, taken := g.reserve.EntityOwner(aid); taken {
			continue
		}
		p, _ := g.positions.Get(aid)
		if d := pathing.Dist(pos.X, pos.Y, p.X, p.Y); d < bestD || (d == bestD && aid < best) {
			best, bestD = aid, d
		}
	}
	if best == 0 {
		return false
	}
	p, _ := g.positions.Get(best)
	tx, ty, ok := g.besideSpot(id, p.X, p.Y)
	if !ok {
		return false
	}
	rec := g.withMedicine(id, job.Record{Kind: job.TendAnimal, Target: best, TX: tx, TY: ty, WorkLeft: TendWork}) // infection.go
	if !g.startJob(id, rec) {
		return false
	}
	if rec.Dest != 0 {
		g.reserve.TryEntity(id, rec.Dest)
	}
	return true
}

// doTendAnimal tends a hurt animal: fetch the medicine, work beside it,
// then mend it.
func (g *Game) doTendAnimal(id entity.EntityID, j job.Record) {
	now := uint64(g.clock.Now())
	a, ok := g.animals.Get(j.Target)
	if !ok || !a.Tame || a.Body == nil && a.Injury == 0 || a.Body != nil && !a.Body.TendDue(now) {
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	if j.Dest != 0 && j.Carry == 0 && !g.pickUpMedicine(id, &j) {
		return // walking to the animal (or the job ended)
	}
	pos, _ := g.positions.Get(id)
	if ap, _ := g.positions.Get(j.Target); weapon.Distance(pos.X, pos.Y, ap.X, ap.Y) > 1 {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	u := g.workProgress(id, skMedicine, true) // skills.go
	if u == 0 {
		g.jobs.Set(id, j)
		return
	}
	if j.WorkLeft -= u; j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	q := g.tendQualityFor(id, j.Target, g.tendMedicine(j)) // infection.go (MED-02)
	if j.Carry != 0 {
		g.deleteItem(j.Carry)
		j.Carry = 0
	}
	tendAnimalBody(&a, q, now) // animal_health.go: its wounds, or (no body) Injury
	g.tendedUntil[j.Target] = now + TendTicks
	g.tryBond(j.Target, &a, id, training.BondTend) // bonds.go
	g.animals.Set(j.Target, a)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// naturalHeal: a fed tame animal heals a point every animalHealEvery
// ticks (half that while tended). Called on the decision cycle only.
func (g *Game) naturalHeal(id entity.EntityID, a *animal.State, now uint64) {
	if a.Injury <= 0 || a.HasSex && !animal.NeedsFree(a.Species) && a.Food < animal.FeedStart {
		return
	}
	every := uint64(animalHealEvery)
	if g.tended(id) {
		every /= 2
	}
	if (now+uint64(id))%every == 0 {
		a.Injury--
		reconcileAnimalBody(a) // animal_health.go: the wounds follow
	}
}
