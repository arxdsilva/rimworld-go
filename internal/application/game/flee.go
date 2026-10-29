package game

import (
	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Unarmed colonists flee (PLAY-010, decision 6): an undrafted colonist
// with no weapon, or one who won't fight, runs from any animal coming for
// it from combat.FleeReach tiles off (FleeRelease once fleeing; the
// combat package's Guard), ahead of its
// orders, its work and its sleep. The flight's goal is fleeDist tiles
// away from the threat, turned by 45° and then 90° when that way is shut,
// and always one the colonist can actually walk to.

// fleeDist is how far a flight's goal lies from the colonist.
const fleeDist = 8

// fightsBack reports a colonist that meets an attacker with a weapon
// rather than running: armed and able to do violence (skills.go).
func (g *Game) fightsBack(id entity.EntityID) bool {
	_, armed := g.equip[id]
	return armed && g.canViolence(id)
}

// fleeGoal picks where colonist id at pos runs from a threat at tp:
// straight away, then ±45°, then ±90°. A candidate must have a free
// walkable tile within 3 that the colonist can reach and that is no
// nearer the threat; of those, the first inside the home area wins, else
// the first. ok is false when the colonist is cornered.
func (g *Game) fleeGoal(id entity.EntityID, pos, tp Position) (int, int, bool) {
	dx, dy := pos.X-tp.X, pos.Y-tp.Y
	if dx == 0 && dy == 0 {
		dx = 1
	}
	dirs := [...][2]int{
		{dx, dy},
		{dx - dy, dx + dy}, {dx + dy, dy - dx}, // ±45°
		{-dy, dx}, {dy, -dx}, // ±90°
	}
	now := weapon.Distance(pos.X, pos.Y, tp.X, tp.Y)
	w := g.grid.Width()
	bx, by, found := 0, 0, false
	for _, d := range dirs {
		fx, fy := animal.FleeGoal(pos.X, pos.Y, pos.X-d[0], pos.Y-d[1], fleeDist)
		fx, fy, ok := g.combat.Tiles.WalkableNear(fx, fy, 3)
		if !ok || fx == pos.X && fy == pos.Y || weapon.Distance(fx, fy, tp.X, tp.Y) < now ||
			!g.reachable(id, fx, fy) {
			continue
		}
		if g.inHome(fy*w + fx) {
			return fx, fy, true
		}
		if !found {
			bx, by, found = fx, fy, true
		}
	}
	return bx, by, found
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
