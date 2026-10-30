package game

import (
	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// Unarmed colonists flee (PLAY-010, decision 6): an undrafted colonist
// with no weapon, or one who won't fight, runs from any animal coming for
// it from combat.FleeReach tiles off (FleeRelease once fleeing; combat's
// Guard, which also picks the flight's goal), ahead of its orders, its
// work and its sleep.

// fightsBack reports a colonist that meets an attacker with a weapon
// rather than running: armed, able to do violence (skills.go) and not set
// to flee or ignore (CBT-4, combat/response.go).
func (g *Game) fightsBack(id entity.EntityID) bool {
	_, armed := g.equip[id]
	return g.combat.Drafts.Fights(id, armed && g.canViolence(id))
}

// dropForFlight stops colonist id's job j so it can run; a player order
// it was following goes back to the front of its queue, to be picked up
// again once the colonist has calmed down.
func (g *Game) dropForFlight(id entity.EntityID, j job.Record) {
	if j.Forced && j.Active() {
		if q, ok := g.postponed.orders[id]; ok {
			delete(g.postponed.orders, id)
			q.retryAt = 0
			g.orderQueue[id] = append([]queuedOrder{q}, g.orderQueue[id]...)
		}
	}
	g.interrupt(id, job.Interrupted)
}

// fleeViewSlack widens the release radii for the read model: it is read
// after movement, when the colonist and its attacker may each have taken
// a step since defend last looked.
const fleeViewSlack = 2

// fillFleeView marks a colonist that is fleeing, or still shaken with
// its attacker in release range, in the read model: Fleeing, and the job
// line "Fleeing from <creature>".
func (g *Game) fillFleeView(pv *PawnView, id entity.EntityID) {
	until, ok := g.calm[id]
	if !ok || uint64(g.clock.Now()) >= until || g.drafted(id) || g.fightsBack(id) {
		return
	}
	j, _ := g.jobs.Get(id)
	running := j.Kind == job.Move && j.State == job.Goto && !j.Forced
	pos, _ := g.positions.Get(id)
	threat, _, _, _ := g.combat.Guard.NearestThreat(id, pos.X, pos.Y, combat.FleeRelease+fleeViewSlack, combat.RaiderRelease+fleeViewSlack)
	switch {
	case threat != 0:
		pv.Job = "Fleeing from " + g.creatureName(threat)
	case running:
		pv.Job = "Fleeing"
	default:
		return
	}
	pv.Fleeing = true
}
