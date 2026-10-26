package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-020 medical beds and bed rest (bedrest.go).

// cmdSetBedMedical keeps bed id for the sick and injured (on) or not; it
// changes that bed only. Turning it on frees the bed of its owners: a
// colonist who is a patient keeps its slot for its stay; any other gets
// up if it sleeps there. A prisoner keeps its prison bed.
func (g *Game) cmdSetBedMedical(id entity.EntityID, on bool) {
	b, ok := g.bldgs.Get(id)
	if !ok || !b.Kind.Sleeps() || b.Medical == on {
		return
	}
	b.Medical = on
	var up []entity.EntityID
	if on {
		for _, c := range [2]uint64{b.Claimant, b.Claimant2} {
			cid := entity.EntityID(c)
			switch {
			case c == 0 || g.isPrisoner(cid):
			case g.injuredDowned(cid) || g.mayUseMedicalBed(cid):
				g.holdRestBed(cid, id)
			default:
				b = b.Release(c)
				if j, _ := g.jobs.Get(cid); j.Kind == job.Sleep && j.Target == id && j.Active() {
					up = append(up, cid)
				}
			}
		}
	}
	g.bldgs.Set(id, b)
	for _, cid := range up {
		g.interrupt(cid, job.Interrupted)
	}
}

// cmdSetPawnBedRest turns colonist id's rest when hurt on or off; off ends
// a rest at once.
func (g *Game) cmdSetPawnBedRest(id entity.EntityID, on bool) {
	if !g.colonist(id) {
		return
	}
	if on {
		delete(g.rest.off, id)
		return
	}
	g.rest.ensure()
	g.rest.off[id] = true
	delete(g.rest.since, id)
}
