package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Kidnapping (ENG-003, M9b). A raider running for the edge, or standing
// about with no one left to fight, that finds a downed colonist within
// kidnapRange which no standing colonist guards, picks it up (job.Kidnap,
// g.carry, as a rescue does) and carries it to the nearest map edge.
// There both leave: the colonist is stashed (world_stash.go) for the
// raider's faction, Where Kidnapped, out of every pawnIDs loop and of the
// colony's losses; its relations grieve by the lost table (socialOnLost)
// and a letter says who took it. Down or kill the kidnapper on the way
// and the colonist is dropped where it fell (interrupt, dropPatient).
//
// No flap: a raider commits to the colonist it picked until the kidnap
// ends, and one that fails (the colonist got up, someone else took it,
// no way out) waits kidnapCooldown before it looks again.

const (
	// kidnapRange: how far (tiles) a raider looks for a downed colonist.
	kidnapRange = 10
	// kidnapScan: a raider looks once in this many ticks (staggered by id).
	kidnapScan = 60
	// kidnapCooldown: after a failed kidnap the raider waits this long.
	kidnapCooldown = clock.TicksPerHour
	// kidnapEdgeR: how far from its edge point the way out may end.
	kidnapEdgeR = 3
)

// tryKidnap lets standing raider id at pos look for a colonist to carry
// off; it reports whether the raider's turn went to it (a kidnap began, or
// one could not start this tick).
func (g *Game) tryKidnap(id entity.EntityID, pos Position, now uint64) bool {
	r := g.raiders[id]
	if now < r.KidnapAt || (now+uint64(id))%kidnapScan != 0 || g.care.Carry.Patient(id) != 0 {
		return false
	}
	t, tp, ok := g.kidnapTarget(id, pos)
	if !ok {
		return false
	}
	sx, sy, ok := pos.X, pos.Y, true // already beside it
	if !g.adjacent(id, t) {
		sx, sy, ok = g.besideSpot(id, tp.X, tp.Y)
	}
	if ok && g.startJob(id, job.Record{Kind: job.Kidnap, Target: t, TX: sx, TY: sy}) {
		return true
	}
	g.endKidnap(id, now)
	return true
}

// kidnapTarget is the nearest downed colonist raider id at pos may carry
// off: in range, not carried, held or tended for by anyone else, and with
// no standing colonist beside it.
func (g *Game) kidnapTarget(id entity.EntityID, pos Position) (entity.EntityID, Position, bool) {
	var best entity.EntityID
	var bp Position
	bestD := kidnapRange + 1
	for _, c := range g.pawnIDs {
		v, ok := g.vital.Get(c)
		if !ok || v.Status != health.Downed {
			continue
		}
		cp, ok := g.positions.Get(c)
		if !ok {
			continue
		}
		d := weapon.Distance(pos.X, pos.Y, cp.X, cp.Y)
		if d >= bestD || g.care.Carry.Carrier(c) != 0 || g.guarded(c, cp) {
			continue
		}
		if owner, held := g.reserve.EntityOwner(c); held && owner != id {
			continue
		}
		best, bp, bestD = c, cp, d
	}
	return best, bp, best != 0
}

// guarded reports a standing colonist stands beside downed colonist c.
func (g *Game) guarded(c entity.EntityID, cp Position) bool {
	for _, o := range g.pawnIDs {
		if o == c || !g.standingPawn(o) {
			continue
		}
		if op, ok := g.positions.Get(o); ok && abs(op.X-cp.X) <= 1 && abs(op.Y-cp.Y) <= 1 {
			return true
		}
	}
	return false
}

// stepKidnap runs raider id's kidnap job: walk to the colonist, pick it
// up and head for the edge, and at the edge leave with it.
func (g *Game) stepKidnap(id entity.EntityID, j job.Record, now uint64) {
	t := j.Target
	if j.State != job.Goto && j.State != job.Do {
		g.endKidnap(id, now) // the walk failed (blocked, no way)
		return
	}
	v, ok := g.vital.Get(t)
	if !ok || v.Status != health.Downed {
		g.endKidnap(id, now) // it got up, or died
		return
	}
	if g.care.Carry.Patient(id) != t {
		if c := g.care.Carry.Carrier(t); c != 0 && c != id || g.isStashed(t) {
			g.endKidnap(id, now) // someone else has it
			return
		}
		if j.State != job.Do {
			return // still walking over (phaseMovement)
		}
		g.pickUpKidnapped(id, t, j, now)
		return
	}
	if j.State == job.Do {
		g.kidnapOff(id, t)
	}
}

// pickUpKidnapped: raider id beside colonist t lifts it and heads for the
// nearest edge.
func (g *Game) pickUpKidnapped(id, t entity.EntityID, j job.Record, now uint64) {
	if !g.adjacent(id, t) {
		g.endKidnap(id, now)
		return
	}
	pos, _ := g.positions.Get(id)
	ex, ey := nearestEdge(pos, g.grid.Width(), g.grid.Height())
	x, y, ok := g.walkableNear(ex, ey, kidnapEdgeR)
	if !ok {
		g.endKidnap(id, now)
		return
	}
	path, ok := g.pathTo(id, x, y)
	if !ok {
		g.endKidnap(id, now)
		return
	}
	g.dropRaiderGear(t) // the colonist's weapon stays where it fell
	g.care.Carry.Pick(id, t)
	j.TX, j.TY, j.Path, j.PathI, j.State = x, y, path, 0, job.Goto
	if len(path) == 0 {
		j.State = job.Do
	}
	g.jobs.Set(id, j)
	g.message(pos.X, pos.Y, "%s is carrying %s away!", g.name(id), g.personName(t))
}

// endKidnap ends raider id's kidnap (put down whoever it carries) and
// starts its cooldown.
func (g *Game) endKidnap(id entity.EntityID, now uint64) {
	g.care.DropPatient(id)
	g.reserve.ReleaseOwner(id)
	g.jobs.Set(id, job.Record{})
	r := g.raiders[id]
	r.KidnapAt = now + kidnapCooldown
	g.raiders[id] = r
}

// kidnapOff: raider id reached the edge with colonist t; both leave the
// map, t held by the raider's faction.
func (g *Game) kidnapOff(id, t entity.EntityID) {
	g.care.Carry.Drop(id)
	pos, _ := g.positions.Get(id)
	g.positions.Set(t, pos)
	fac := 0
	if p := g.personOf(id); p != nil {
		fac = p.Faction
	}
	by := "raiders"
	if f := g.factionOf(fac); f != nil {
		by = f.Name
	}
	// a kidnapped pawn's stash holder is its captor faction (1 + index)
	if g.stashPawn(t, entity.EntityID(fac)) {
		g.setWhere(t, whereKidnapped)
		name := g.personName(t)
		g.socialOnLostAs(t, "kidnapped", name+" was kidnapped by "+by+".") // social_grief.go
		g.letter(letterThreat, "Kidnapped", name+" was kidnapped by "+by+".", t)
	}
	g.removeRaider(id) // raids.go: off the map, an off-map person again
	g.invalidateCreatures()
}
