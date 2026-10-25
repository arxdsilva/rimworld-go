package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Roping (PLAY-015 §2.4, Handling work). A handler walks to a penned
// animal that is outside the pen it belongs in, ropes it (ropeWork), and
// leads it there: the animal follows a cell behind, through the gates it
// can't pass alone, and is let go once inside. The job record carries the
// leg: Count 0 while roping, 1 while leading; Dest is the target pen's
// marker. A handler who can't reach the animal or the pen puts the job
// off (postpone.go).

const (
	// ropeWork is the work of putting a rope on an animal.
	ropeWork = 60
	// ropeWait: at the pen, the handler waits this long for a lagging
	// animal before giving up.
	ropeWait = 600
	// ropeReplan: a following animal re-aims at its handler this often.
	ropeReplan = 10
)

// ropeHandler is the colonist roping or leading animal id (0: none).
func (g *Game) ropeHandler(id entity.EntityID) entity.EntityID {
	owner, ok := g.reserve.EntityOwner(id)
	if !ok {
		return 0
	}
	if j, _ := g.jobs.Get(owner); j.Kind == job.Rope && j.Target == id {
		return owner
	}
	return 0
}

// ledBy is the handler leading animal id on a rope (0: not led).
func (g *Game) ledBy(id entity.EntityID) entity.EntityID {
	h := g.ropeHandler(id)
	if h == 0 {
		return 0
	}
	if j, _ := g.jobs.Get(h); j.Count == 1 {
		return h
	}
	return 0
}

// gatesOpen reports that animal id passes the colony's doors and gates
// (fences.go): tame area animals always, penned livestock only while led
// on a rope, wild animals never.
func (g *Game) gatesOpen(id entity.EntityID, a *animal.State) bool {
	if !a.Tame {
		return false
	}
	if !animal.RosterOf(a.Species).Penned {
		return true
	}
	return g.ledBy(id) != 0
}

// assignRope sends handler id to rope the nearest penned animal that is
// outside the pen it belongs in: within the handler's allowed area (the
// animal's cell and the pen's marker), reachable, and not put off.
func (g *Game) assignRope(id entity.EntityID) bool {
	if !g.anyAnimal(func(_ entity.EntityID, a *animal.State) bool { return penned(a) }) || !g.anyEnclosedPen() {
		return false
	}
	pos, _ := g.positions.Get(id)
	var best, bestPen entity.EntityID
	bestD := 1 << 30
	for _, aid := range g.animalIDs {
		a := g.animals.Ref(aid)
		if a == nil || !penned(a) || a.Dead() || a.Dormant || a.Enraged(uint64(g.clock.Now())) {
			continue
		}
		if _, taken := g.reserve.EntityOwner(aid); taken || g.postponedFor(id, job.Rope, aid, 0, 0) {
			continue
		}
		ap, _ := g.positions.Get(aid)
		d := octileDist(pos.X, pos.Y, ap.X, ap.Y)
		if d > bestD || d == bestD && aid > best {
			continue
		}
		if m, _ := g.homePen(aid, a, ap); m != 0 {
			continue // home already
		}
		m := g.penTarget(aid, a, ap)
		if m == 0 {
			continue
		}
		c := g.penRegion(m)
		if !g.inArea(id, ap.X, ap.Y) || !g.inArea(id, c.x, c.y) {
			continue
		}
		best, bestPen, bestD = aid, m, d
	}
	if best == 0 {
		return false
	}
	ap, _ := g.positions.Get(best)
	tx, ty, ok := g.besideSpot(id, ap.X, ap.Y)
	c := g.penRegion(bestPen)
	if !ok || !g.reachable(id, c.x, c.y) {
		g.postpone(postKey{Pawn: id, Kind: job.Rope, Target: best}, job.Unreachable)
		return false
	}
	return g.startJob(id, job.Record{Kind: job.Rope, Target: best, TX: tx, TY: ty, WorkLeft: ropeWork, Dest: bestPen})
}

// doRope is the rope job's work: roping beside the animal, then (once the
// handler stands in the pen) waiting for it to come in.
func (g *Game) doRope(id entity.EntityID, j job.Record) {
	a := g.animals.Ref(j.Target)
	done := func(st job.State, why job.FailReason) {
		j.State, j.Fail = st, why
		g.jobs.Set(id, j)
	}
	if a == nil || !penned(a) || a.Dead() {
		done(job.Complete, job.FailNone)
		return
	}
	pos, _ := g.positions.Get(id)
	ap, _ := g.positions.Get(j.Target)
	if j.Count == 0 {
		if weapon.Distance(pos.X, pos.Y, ap.X, ap.Y) > 1 {
			done(job.Fail, job.TargetGone)
			return
		}
		if u := g.workProgress(id, skAnimals, true); u == 0 {
			return
		} else if j.WorkLeft -= u; j.WorkLeft > 0 {
			g.jobs.Set(id, j)
			return
		}
		g.roped(j.Target, a)
		m := g.penTarget(j.Target, a, ap)
		if m == 0 {
			done(job.Complete, job.FailNone) // nowhere to go any more: let it be
			return
		}
		c := g.penRegion(m)
		path, ok := g.pathTo(id, c.x, c.y)
		if !ok {
			g.postpone(postKey{Pawn: id, Kind: job.Rope, Target: j.Target}, job.Unreachable)
			done(job.Fail, job.Unreachable)
			return
		}
		j.Count, j.Dest, j.TX, j.TY, j.Path, j.PathI, j.WorkLeft = 1, m, c.x, c.y, path, 0, ropeWait
		j.State = job.Goto
		if len(path) == 0 {
			j.State = job.Do
		}
		g.jobs.Set(id, j)
		return
	}
	// leading: at the pen, wait for the animal to come in
	if c := g.penRegion(j.Dest); c == nil || !c.enclosed {
		done(job.Fail, job.TargetGone)
		return
	}
	if g.inPenCells(g.penRegion(j.Dest), ap.X, ap.Y) {
		a.Path, a.PathI, a.NextThink = nil, 0, uint64(g.clock.Now())+60
		done(job.Complete, job.FailNone)
		return
	}
	if j.WorkLeft--; j.WorkLeft <= 0 {
		g.postpone(postKey{Pawn: id, Kind: job.Rope, Target: j.Target}, job.Blocked)
		done(job.Fail, job.Blocked)
		return
	}
	g.jobs.Set(id, j)
}

// roped: the rope is on. A leaving animal stays; the unpenned count
// resets.
func (g *Game) roped(id entity.EntityID, a *animal.State) {
	a.Leaving, a.Mode, a.Path, a.PathI = false, animal.Wander, nil, 0
	a.Feed, a.FeedLeft = animal.FeedTarget{}, 0
	delete(g.pen.since, id)
}

// ropedStep is a roped animal's tick (thinkAnimal): it holds still while
// the rope goes on, then follows its handler a cell behind. It reports
// false when no handler has it.
func (g *Game) ropedStep(id entity.EntityID, a *animal.State, pos Position, now uint64) bool {
	h := g.ropeHandler(id)
	if h == 0 {
		return false
	}
	a.Mode, a.Feed, a.FeedLeft, a.Leaving = animal.Wander, animal.FeedTarget{}, 0, false
	if j, _ := g.jobs.Get(h); j.Count != 1 {
		a.Path, a.PathI = nil, 0 // holds still for the handler
		return true
	}
	hp, _ := g.positions.Get(h)
	if weapon.Distance(pos.X, pos.Y, hp.X, hp.Y) <= 1 {
		a.Path, a.PathI = nil, 0
		return true
	}
	if a.PathI >= len(a.Path) || (now+uint64(id))%ropeReplan == 0 && (a.GoalX != hp.X || a.GoalY != hp.Y) {
		a.RetryAt = 0
		g.animalGoTo(id, a, hp.X, hp.Y)
	}
	return true
}
