package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// Couples' lovin' (ENG-003, M5). Couples' beds and bed ownership live in
// the rest package (rest/beds.go); lovin' stays here with the social
// layer: its cooldown is saved with the gatherings and its roll is on the
// social stream.
//
// Lovin': every lovinPeriod ticks, staggered by id, a couple lying asleep
// in the same double bed may make love (an age-based mean interval sets
// the chance and the cooldown after it): both run job.Lovin for lovinTicks,
// then each gets mood.Lovin. Never while downed, in pain or dazed, or
// while either thinks badly of the other. No pregnancy.

const (
	lovinPeriod  = 2500
	lovinStagger = 37
	lovinTicks   = 250
	// lovinMaxPain and lovinMinConscious gate lovin' (permille).
	lovinMaxPain      = 400
	lovinMinConscious = 700
)

// lovinInterval is the mean ticks between a couple's lovin' by age: an
// hour and a half up to 22, then rarer with the years.
var lovinInterval = social.Curve{{X: 16, Y: 3750}, {X: 22, Y: 3750}, {X: 30, Y: 10000}, {X: 50, Y: 30000}, {X: 75, Y: 90000}}

// BedForPrisoners reports bed is marked for prisoners (the inspector's
// toggle state; M8 owns the mark).
func (g *Game) BedForPrisoners(bed entity.EntityID) bool {
	b, ok := g.bldgs.Get(bed)
	return ok && b.ForPrisoners
}

// lovinCheck is couple member id's staggered lovin' turn (the lower id of
// the two decides for both).
func (g *Game) lovinCheck(id entity.EntityID, now uint64) {
	p := g.partnerOf(id)
	if p == 0 || p < id || g.partnerOf(p) != id {
		return
	}
	bid, ok := g.lovinBed(id, p)
	if !ok || !g.lovinFit(id, p) {
		return
	}
	s := g.gatherSt()
	if now < s.lovinAt[id] || now < s.lovinAt[p] {
		return
	}
	age := (g.socialPerson(id).AgeYears + g.socialPerson(p).AgeYears) / 2
	mean := max(int64(lovinInterval.Eval(int32(age))), lovinPeriod)
	if int64(g.stream(streamSocial).IntN(1000)) >= lovinPeriod*1000/mean {
		return
	}
	s.lovinAt[id], s.lovinAt[p] = now+uint64(mean), now+uint64(mean)
	for _, x := range [2]entity.EntityID{id, p} {
		pos, _ := g.positions.Get(x)
		o := p
		if x == p {
			o = id
		}
		g.jobs.Set(x, job.Record{Kind: job.Lovin, State: job.Do, Target: o, Dest: bid, TX: pos.X, TY: pos.Y, WorkLeft: lovinTicks})
	}
}

// lovinBed is the double bed a and b both lie asleep in.
func (g *Game) lovinBed(a, b entity.EntityID) (entity.EntityID, bool) {
	bid := g.sleptAt[a]
	if bid == 0 || g.sleptAt[b] != bid {
		return 0, false
	}
	bed, ok := g.bldgs.Get(bid)
	if !ok || bed.Kind != construction.DoubleBed || bed.SlotOf(uint64(a)) < 0 || bed.SlotOf(uint64(b)) < 0 {
		return 0, false
	}
	for _, x := range [2]entity.EntityID{a, b} {
		if j, _ := g.jobs.Get(x); j.Kind != job.Sleep || j.State != job.Do {
			return 0, false
		}
	}
	return bid, true
}

// lovinFit: both standing (not downed), not in pain, clear-headed and
// fond enough of each other.
func (g *Game) lovinFit(a, b entity.EntityID) bool {
	for _, x := range [2]entity.EntityID{a, b} {
		if !g.alive(x) || g.breaking(x) || g.drafted(x) {
			return false
		}
		if bd := g.bodies[x]; bd != nil && (bd.Pain() > lovinMaxPain || bd.Capacity(body.Consciousness) < lovinMinConscious) {
			return false
		}
	}
	return g.opinion(a, b) >= 0 && g.opinion(b, a) >= 0
}

// doLovin runs a lovin' job: when it ends with the partner still there,
// both remember it fondly; a partner who left ends it without one.
func (g *Game) doLovin(id entity.EntityID, j job.Record) {
	if pj, _ := g.jobs.Get(j.Target); pj.Kind != job.Lovin || pj.Target != id {
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft--
	if j.WorkLeft <= 0 {
		j.State = job.Complete
		g.memories[id] = g.memories[id].AddFor(mood.Lovin, uint64(j.Target), uint64(g.clock.Now()))
	}
	g.jobs.Set(id, j)
}
