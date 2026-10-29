package game

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/entity"
)

// Capture and arrest (ENG-003, M8). A captor walks to a downed raider (or
// visitor), picks it up and carries it, the doctor's way (carrying,
// followCarriers), to a free prisoner bed; laid in it, the pawn becomes a
// prisoner. An arrester walks to a colonist or visitor and rolls: on
// success the target is a prisoner on the spot and is carried off the
// same way; on failure it shakes free.

// Values follow the published RimWorld wiki by explicit user decision (ENG-003 balance override).
const (
	// arrestBase is the arrest chance, permille, before the arrester's
	// Social skill (arrestPerSocial a level) and ArrestSuccess stat.
	arrestBase      = 600
	arrestPerSocial = 20
	arrestMax       = 980
	// arrestChase is how often an arrester re-paths after a target that
	// walked off before it gives up.
	arrestChase = 6
	// prisonerVisitStagger spreads first visits over the interval by id.
	prisonerVisitStagger = 7919
)

// capturable reports target is a downed pawn the colony may take
// prisoner: a raider or a visitor, not yet a prisoner.
func (g *Game) capturable(target entity.EntityID) bool {
	v, ok := g.vital.Get(target)
	if !ok || v.Status != health.Downed || g.isPrisoner(target) {
		return false
	}
	return g.isRaider(target) || g.visitor(target)
}

// visitor reports id is a friendly visitor on the map (M9).
func (g *Game) visitor(id entity.EntityID) bool {
	p := g.personOf(id)
	return p != nil && p.Where == whereVisitor && g.onMap(id)
}

// arrestable reports target is a colonist or visitor that can be arrested.
func (g *Game) arrestable(target entity.EntityID) bool {
	v, ok := g.vital.Get(target)
	if !ok || v.Status == health.Dead || g.isPrisoner(target) {
		return false
	}
	if g.visitor(target) {
		return true
	}
	return slices.Contains(g.pawnIDs, target)
}

// handler is who carries out a capture or arrest of target: by when it is
// a standing colonist able to, else (by 0) the nearest such colonist.
func (g *Game) handler(by, target entity.EntityID) (entity.EntityID, bool) {
	able := func(id entity.EntityID) bool {
		return slices.Contains(g.pawnIDs, id) && id != target && g.standingPawn(id) && !g.breaking(id)
	}
	if by != 0 {
		return by, able(by)
	}
	tp, _ := g.positions.Get(target)
	best, bestD := entity.EntityID(0), 1<<30
	for _, id := range g.pawnIDs {
		if !able(id) {
			continue
		}
		p, _ := g.positions.Get(id)
		if d := manhattan(p.X, p.Y, tp.X, tp.Y); d < bestD {
			best, bestD = id, d
		}
	}
	return best, best != 0
}

// cmdCapture orders captor (0: the nearest able colonist) to take the
// downed target prisoner. It needs a free prisoner bed.
func (g *Game) cmdCapture(captor, target entity.EntityID) {
	if !g.capturable(target) || g.beingCaptured(target) {
		return
	}
	captor, ok := g.handler(captor, target)
	if !ok {
		return
	}
	tp, _ := g.positions.Get(target)
	if _, ok := g.prisonBedFor(target, tp); !ok {
		g.message(tp.X, tp.Y, "No free prisoner bed for %s.", g.personName(target))
		return
	}
	sx, sy, ok := g.besideSpot(captor, tp.X, tp.Y)
	if !ok {
		return
	}
	g.interrupt(captor, job.Interrupted)
	g.startJob(captor, job.Record{Kind: job.Capture, Target: target, TX: sx, TY: sy})
}

// cmdArrest orders arrester (0: the nearest able colonist) to arrest the
// colonist or visitor target. It needs a free prisoner bed.
func (g *Game) cmdArrest(arrester, target entity.EntityID) {
	if !g.arrestable(target) || g.beingCaptured(target) {
		return
	}
	arrester, ok := g.handler(arrester, target)
	if !ok {
		return
	}
	tp, _ := g.positions.Get(target)
	if _, ok := g.prisonBedFor(target, tp); !ok {
		g.message(tp.X, tp.Y, "No free prisoner bed for %s.", g.personName(target))
		return
	}
	sx, sy, ok := g.besideSpot(arrester, tp.X, tp.Y)
	if !ok {
		return
	}
	g.interrupt(arrester, job.Interrupted)
	g.startJob(arrester, job.Record{Kind: job.Arrest, Target: target, TX: sx, TY: sy})
}

// beingCaptured reports a captor or arrester holds target (it does not
// bleed out, walk or work meanwhile).
func (g *Game) beingCaptured(target entity.EntityID) bool {
	owner, ok := g.reserve.EntityOwner(target)
	if !ok {
		return false
	}
	j, _ := g.jobs.Get(owner)
	return (j.Kind == job.Capture || j.Kind == job.Arrest) && j.Target == target && j.Active()
}

// adjacent reports a and b stand within one tile of each other.
func (g *Game) adjacent(a, b entity.EntityID) bool {
	pa, _ := g.positions.Get(a)
	pb, _ := g.positions.Get(b)
	return abs(pa.X-pb.X) <= 1 && abs(pa.Y-pb.Y) <= 1
}

// doCapture: beside the captive pick it up and head for a prisoner bed;
// at the bed lay it down, a prisoner.
func (g *Game) doCapture(id entity.EntityID, j job.Record) {
	t := j.Target
	if v, ok := g.vital.Get(t); !ok || v.Status == health.Dead || !g.capturable(t) && !g.isPrisoner(t) {
		g.care.DropPatient(id)
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	if j.Dest == 0 {
		if g.care.Carry.Patient(id) != t && !g.adjacent(id, t) {
			j.State, j.Fail = job.Fail, job.TargetGone
			g.jobs.Set(id, j)
			return
		}
		pos, _ := g.positions.Get(id)
		bid, ok := g.prisonBedFor(t, pos)
		if !ok || !g.reserve.TryEntity(id, bid) {
			tp, _ := g.positions.Get(t)
			g.message(tp.X, tp.Y, "No free prisoner bed for %s.", g.personName(t))
			j.State, j.Fail = job.Fail, job.Reserved
			g.jobs.Set(id, j)
			return
		}
		b, _ := g.bldgs.Get(bid)
		sx, sy, ok := g.besideSpot(id, b.X, b.Y)
		var path []pathing.Point
		if ok {
			path, ok = g.pathTo(id, sx, sy)
		}
		if !ok {
			j.State, j.Fail = job.Fail, job.Unreachable
			g.jobs.Set(id, j)
			return
		}
		g.dropRaiderGear(t) // a captive's weapon stays where it was taken
		g.care.Carry.Pick(id, t)
		j.Dest, j.TX, j.TY, j.Path, j.PathI = bid, sx, sy, path, 0
		j.State = job.Goto
		if len(path) == 0 {
			j.State = job.Do
		}
		g.jobs.Set(id, j)
		if j.State == job.Goto {
			return
		}
	}
	b, ok := g.bldgs.Get(j.Dest)
	if !ok || !b.ForPrisoners {
		g.care.DropPatient(id)
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	g.care.Carry.Drop(id)
	if !g.isPrisoner(t) {
		g.makePrisoner(t, id)
	}
	g.layInBed(t, j.Dest)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// layInBed puts pawn id in bed bid (its slot claimed, on its sleep cell).
func (g *Game) layInBed(id, bid entity.EntityID) {
	g.releaseBed(id)
	b, _ := g.bldgs.Get(bid)
	b = b.Claim(uint64(id))
	g.bldgs.Set(bid, b)
	cx, cy := g.sleepCellFor(id, b)
	g.positions.Set(id, Position{X: cx, Y: cy})
	g.sleptAt[id] = bid
}

// doArrest: beside the target roll the arrest; on success carry the new
// prisoner off (the job becomes a capture), else the target shakes free.
func (g *Game) doArrest(id entity.EntityID, j job.Record) {
	t := j.Target
	if !g.arrestable(t) {
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	if !g.adjacent(id, t) {
		tp, _ := g.positions.Get(t)
		sx, sy, ok := g.besideSpot(id, tp.X, tp.Y)
		var path []pathing.Point
		if ok {
			path, ok = g.pathTo(id, sx, sy)
		}
		if j.Count++; !ok || j.Count > arrestChase {
			j.State, j.Fail = job.Fail, job.TargetGone
			g.jobs.Set(id, j)
			return
		}
		j.TX, j.TY, j.Path, j.PathI, j.State = sx, sy, path, 0, job.Goto
		if len(path) == 0 {
			j.State = job.Do
		}
		g.jobs.Set(id, j)
		return
	}
	pos, _ := g.positions.Get(id)
	if g.stream(streamPrison).IntN(1000) >= g.arrestChance(id, t) {
		g.message(pos.X, pos.Y, "%s resisted arrest by %s.", g.personName(t), g.personName(id))
		j.State, j.Fail = job.Fail, job.Interrupted
		g.jobs.Set(id, j)
		g.startSocialFight(t, id) // social_fight.go: the target fights back
		return
	}
	if g.visitor(t) {
		if f := g.factionOf(g.personOf(t).Faction); f != nil {
			faction.Adjust(f, -arrestGoodwill, "visitor arrested")
		}
	}
	g.interrupt(t, job.Interrupted)
	g.makePrisoner(t, id)
	g.doCapture(id, job.Record{Kind: job.Capture, Target: t, State: job.Do})
}

// arrestChance is the permille chance arrester takes target in: certain
// when the target is down or asleep, else the base plus the arrester's
// Social skill, times its ArrestSuccess stat.
func (g *Game) arrestChance(arrester, target entity.EntityID) int {
	v, _ := g.vital.Get(target)
	tj, _ := g.jobs.Get(target)
	if v.Status != health.Alive || tj.Kind == job.Sleep && tj.State == job.Do {
		return 1000
	}
	c := int64(arrestBase + arrestPerSocial*int(g.skillOf(arrester, skSocial).Level))
	c = c * int64(g.statOf(arrester, stat.ArrestSuccess)) / 1000
	return int(min(max(c, 0), arrestMax))
}

// makePrisoner turns target into the colony's prisoner, taken by captor:
// a colonist leaves the roster (keeping its relations and bio), every
// prisoner joins raiderIDs out of the raid AI, with a fresh resistance
// and the Recruit mode.
func (g *Game) makePrisoner(target, captor entity.EntityID) {
	now := uint64(g.clock.Now())
	g.ensurePerson(target)
	if slices.Contains(g.pawnIDs, target) {
		g.pawnIDs = withoutID(g.pawnIDs, target)
		delete(g.draft, target)
		g.releaseBed(target)
		g.raiderIDs = withID(g.raiderIDs, target)
	}
	r := g.raiders[target]
	r.Leaving, r.Hungry, r.FoodRetry, r.Warden = false, false, 0, 0
	r.Mode, r.Idle, r.RetryAt = raidArrive, 0, 0
	if r.Arrived == 0 {
		r.Arrived = now
	}
	g.raiders[target] = r
	if g.prisoners == nil {
		g.prisoners = map[entity.EntityID]*prisoner.State{}
	}
	roll := int32(g.stream(streamPrison).IntN(1001))
	g.prisoners[target] = &prisoner.State{
		Mode: prisoner.Recruit, Resistance: prisoner.InitialResistance(roll),
		NextVisit: now + uint64(target)*prisonerVisitStagger%prisoner.VisitInterval, Captor: uint64(captor),
	}
	g.setWhere(target, wherePrisoner)
	if _, ok := g.joy[target]; !ok {
		g.joy[target] = recreation.Start
	}
	pos, _ := g.positions.Get(target)
	g.message(pos.X, pos.Y, "%s is now the colony's prisoner.", g.personName(target))
	g.addHistory("prisoner", g.personName(target)+" was taken prisoner.", target, captor)
}
