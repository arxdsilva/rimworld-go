package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/entity"
)

// A prisoner's life (ENG-003, M8), run from phaseThreats for every
// prisoner on raiderIDs: its needs fall and its wounds heal as a
// colonist's do, it sleeps in its bed when tired and potters about its
// room (it paths like a raider: doors hold it), and it never works. A
// warden brings it food (the food band below) and talks to it. Released,
// it walks to the map edge and goes home.

const (
	// prisonerFoodLow and prisonerFoodFed are the food band: below Low a
	// prisoner wants a meal brought, until it is fed past Fed.
	prisonerFoodLow = needs.CriticalHunger + 1500
	prisonerFoodFed = 8000
	// prisonerWander is the ticks between an idle prisoner's strolls
	// (staggered by id); prisonerRoam how far from its bed it goes.
	prisonerWander = 900
	prisonerRoam   = 3
)

// prisonerThoughts are a prisoner's own thoughts: being held (none for
// anyone else).
func (g *Game) prisonerThoughts(id entity.EntityID) []mood.Thought {
	if !g.isPrisoner(id) {
		return nil
	}
	return mood.Active(nil, mood.Imprisoned)
}

// prisonerThoughtList is a prisoner's whole thought list: thoughts plus
// prisonerThoughts unless thoughts already carry them.
func (g *Game) prisonerThoughtList(id entity.EntityID) []mood.Thought {
	ts := g.thoughts(id)
	for _, t := range ts {
		if t.Kind == mood.Imprisoned {
			return ts
		}
	}
	return append(ts, g.prisonerThoughts(id)...)
}

// prisonerMood is the mood a prisoner's resistance answers to.
func (g *Game) prisonerMood(id entity.EntityID) int { return mood.Level(g.prisonerThoughtList(id)) }

// thinkPrisoner is one tick of prisoner (or released pawn) id.
func (g *Game) thinkPrisoner(id entity.EntityID, now uint64) {
	r := g.raiders[id]
	v, _ := g.vital.Get(id)
	if v.Status == health.Dead {
		delete(g.prisoners, id) // the body stays as a raider's does
		r.Leaving = false
		g.raiders[id] = r
		return
	}
	if g.beingCaptured(id) {
		return // in a captor's arms
	}
	g.heal(id, &v, now) // combat.go: faster in bed and tended
	n, _ := g.need.Get(id)
	if v.Status == health.Downed {
		g.prisonerStarve(id, v, n, now)
		g.prisonerFoodBand(id, n)
		return
	}
	j, _ := g.jobs.Get(id)
	asleep := j.Kind == job.Sleep && j.State == job.Do
	if asleep {
		n = needs.SleepTick(n, now)
	} else {
		n = g.decayNeeds(id, n, now) // skills.go
	}
	g.need.Set(id, n)
	if n.Starving() {
		g.down(id, health.CauseNoFood)
		return
	}
	if r.Leaving {
		g.walkOut(id, r, j, now)
		return
	}
	g.prisonerFoodBand(id, n)
	switch {
	case asleep:
		if n.Rest >= needs.WakeRest {
			g.jobs.Set(id, job.Record{})
		}
	case j.State == job.Complete || j.State == job.Fail:
		g.reserve.ReleaseOwner(id)
		g.jobs.Set(id, job.Record{})
	case j.Active():
	case n.Tired():
		g.prisonerSleep(id)
	case (now+uint64(id)*7)%prisonerWander == 0:
		if _, held := g.reserve.EntityOwner(id); !held { // a warden is on the way: stay put
			g.prisonerStroll(id)
		}
	}
}

// prisonerStarve is phaseNeeds' starvation for a downed prisoner.
func (g *Game) prisonerStarve(id entity.EntityID, v health.Record, n needs.Pair, now uint64) {
	if n.Starving() && v.Cause != health.CauseNoFood {
		v.Cause, v.DownedTick = health.CauseNoFood, now
		g.vital.Set(id, v)
	}
	if v.Cause == health.CauseNoFood && now-v.DownedTick >= health.StarvationTicks {
		g.die(id, health.CauseNoFood)
	}
}

// prisonerFoodBand moves the Hungry flag with hysteresis: on at or below
// prisonerFoodLow, off at or above prisonerFoodFed.
func (g *Game) prisonerFoodBand(id entity.EntityID, n needs.Pair) {
	r := g.raiders[id]
	switch {
	case !r.Hungry && n.Hunger <= prisonerFoodLow:
		r.Hungry = true
	case r.Hungry && n.Hunger >= prisonerFoodFed:
		r.Hungry = false
	default:
		return
	}
	g.raiders[id] = r
}

// prisonerBed is the bed prisoner id holds, if it still stands.
func (g *Game) prisonerBed(id entity.EntityID) (entity.EntityID, bool) {
	bid := g.sleptAt[id]
	if bid == 0 {
		return 0, false
	}
	b, ok := g.bldgs.Get(bid)
	return bid, ok && b.SlotOf(uint64(id)) >= 0
}

// prisonerSleep sends a tired prisoner to its bed (or lies down where it
// is without one).
func (g *Game) prisonerSleep(id entity.EntityID) {
	if bid, ok := g.prisonerBed(id); ok {
		b, _ := g.bldgs.Get(bid)
		sx, sy := g.sleepCellFor(id, b)
		if g.startJob(id, job.Record{Kind: job.Sleep, Target: bid, TX: sx, TY: sy, WorkLeft: needs.SleepWorkMin}) {
			return
		}
	}
	pos, _ := g.positions.Get(id)
	g.jobs.Set(id, job.Record{Kind: job.Sleep, State: job.Do, TX: pos.X, TY: pos.Y, WorkLeft: needs.SleepWorkMin})
}

// prisonerStroll walks an idle prisoner to a free tile near its bed in
// the same room (prisoners path like raiders: doors stop them).
func (g *Game) prisonerStroll(id entity.EntityID) {
	bid, ok := g.prisonerBed(id)
	if !ok {
		return
	}
	b, _ := g.bldgs.Get(bid)
	room := g.roomAt(b.X, b.Y)
	r := g.stream(streamPrison)
	x, y := b.X+r.IntN(2*prisonerRoam+1)-prisonerRoam, b.Y+r.IntN(2*prisonerRoam+1)-prisonerRoam
	if !g.grid.InBounds(x, y) || !g.dryWalkable(x, y) || g.roomAt(x, y) != room {
		return
	}
	if _, taken := g.reserve.TileOwner(x, y); taken {
		return
	}
	g.startJob(id, job.Record{Kind: job.Move, TX: x, TY: y})
}

// walkOut takes a released pawn to the map edge; there (or after a day
// of trying) it goes home.
func (g *Game) walkOut(id entity.EntityID, r raiderState, j job.Record, now uint64) {
	pos, _ := g.positions.Get(id)
	w, h := g.grid.Width(), g.grid.Height()
	if pos.X == 0 || pos.Y == 0 || pos.X == w-1 || pos.Y == h-1 || now-r.Idle >= raiderGiveUp {
		g.departReleased(id)
		return
	}
	if j.Active() && j.Kind == job.Move || now < r.RetryAt {
		return
	}
	ex, ey := nearestEdge(pos, w, h)
	if x, y, ok := g.walkableNear(ex, ey, 3); ok && g.startJob(id, job.Record{Kind: job.Move, TX: x, TY: y}) {
		return
	}
	r.RetryAt = now + raiderRetry
	g.raiders[id] = r
}

// releasePrisoner frees prisoner id (by: the warden who let it go, 0 for
// the player's order): it gets up from its bed and walks off the map.
func (g *Game) releasePrisoner(id, by entity.EntityID) {
	if !g.isPrisoner(id) {
		return
	}
	now := uint64(g.clock.Now())
	delete(g.prisoners, id)
	g.interrupt(id, job.Cancelled)
	g.releaseBed(id)
	r := g.raiders[id]
	r.Leaving, r.Hungry, r.Idle, r.RetryAt = true, false, now, 0
	if by != 0 {
		r.Warden = by
	}
	g.raiders[id] = r
	pos, _ := g.positions.Get(id)
	g.message(pos.X, pos.Y, "%s has been released.", g.personName(id))
}

// departReleased: a released pawn reached the edge. Its faction is
// grateful, and it is off the map.
func (g *Game) departReleased(id entity.EntityID) {
	if p := g.personOf(id); p != nil {
		if f := g.factionOf(p.Faction); f != nil {
			faction.Adjust(f, releaseGoodwill, "prisoner released")
		}
	}
	g.setWhere(id, whereOffMap)
	g.addHistory("released", g.personName(id)+" was released and went home.", id)
	g.removeRaider(id) // raids.go: off the map
}

// prisonerPawnView is the read model of a prisoner (or released pawn):
// not hostile, Prisoner while held.
func (g *Game) prisonerPawnView(id entity.EntityID) PawnView {
	pos, _ := g.positions.Get(id)
	name, _ := g.names.Get(id)
	j, _ := g.jobs.Get(id)
	v, _ := g.vital.Get(id)
	n, _ := g.need.Get(id)
	pv := PawnView{ID: id, Name: name, X: pos.X, Y: pos.Y, Job: j.Line(), Status: v.Status.String(), Reason: v.Cause.String(),
		Hunger: needs.Percent(n.Hunger), Rest: needs.Percent(n.Rest), Health: v.HP() * 100 / health.MaxHP,
		Weapon: weaponDefOf(g, id).Name, Prisoner: g.isPrisoner(id), Visitor: g.visitor(id), SleptInBed: g.sleptAt[id] != 0,
		Recreation: g.joy[id] * 100 / recreation.Full}
	ts := g.prisonerThoughtList(id)
	pv.Mood = mood.Level(ts)
	pv.MoodTarget, pv.MoodLabel, pv.Thoughts = pv.Mood, mood.Label(pv.Mood), thoughtViews(ts)
	pv.Lying = v.Status != health.Alive || j.Kind == job.Sleep && (j.State == job.Do || j.State == job.Complete)
	pv.InBed = g.inBed(id)
	pv.CarriedBy = g.carriedBy(id)
	pv.Tended = g.tended(id)
	pv.Infection, pv.Wounded = g.wounds[id].Infection, g.wounds[id].At != 0 && v.Injury > 0
	g.fillPawnAction(&pv, id, j)
	return pv
}
