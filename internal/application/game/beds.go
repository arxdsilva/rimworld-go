package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// Couples' beds and lovin' (ENG-003, M5). assignSleep asks partnerBed
// first: a partnered colonist takes the free slot of the double bed its
// partner holds, else keeps its own double bed, else claims a free double
// bed for the two (the partner joins it on its next sleep); otherwise the
// usual choice (its own bed, any bed) stands. Each rule keeps what the
// couple already holds, so the choice never flips between nights.
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

// slotOwner is who holds slot of bed b (0: free).
func slotOwner(b construction.Building, slot int) uint64 {
	if slot == 1 {
		return b.Claimant2
	}
	return b.Claimant
}

// withSlot is b with slot held by id (0 frees it).
func withSlot(b construction.Building, slot int, id uint64) construction.Building {
	if slot == 1 {
		b.Claimant2 = id
	} else {
		b.Claimant = id
	}
	return b
}

// partnerBed is the bed and slot pawn id should sleep in for its partner:
// the partner's double bed, its own double bed, or a free one (nearest).
func (g *Game) partnerBed(id entity.EntityID) (entity.EntityID, int, bool) {
	if !g.romanceable(id) {
		return 0, 0, false
	}
	p := g.partnerOf(id) // social_romance.go
	if p == 0 || !g.onMap(p) || !g.romanceable(p) {
		return 0, 0, false
	}
	pos, _ := g.positions.Get(id)
	var own, free entity.EntityID
	ownSlot, freeD := 0, 1<<30
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || b.Kind != construction.DoubleBed || b.ForPrisoners || g.isMedicalBed(b) {
			continue // medical beds are for patients (bedrest.go)
		}
		if ps := b.SlotOf(uint64(p)); ps >= 0 {
			if o := slotOwner(b, 1-ps); o == 0 || o == uint64(id) {
				return bid, 1 - ps, true
			}
			continue
		}
		if s := b.SlotOf(uint64(id)); s >= 0 && own == 0 {
			own, ownSlot = bid, s
			continue
		}
		if b.Claimant == 0 && b.Claimant2 == 0 {
			if d := pathing.Dist(pos.X, pos.Y, b.X, b.Y); d < freeD {
				free, freeD = bid, d
			}
		}
	}
	switch {
	case own != 0:
		return own, ownSlot, true
	case free != 0:
		return free, 0, true
	}
	return 0, 0, false
}

// sleepInPartnerBed sends pawn id to slot of bed bid, giving up any other
// bed; false (nothing changed) leaves assignSleep to choose as usual.
func (g *Game) sleepInPartnerBed(id, bid entity.EntityID, slot int) bool {
	b, ok := g.bldgs.Get(bid)
	if !ok || slot < 0 || slot >= b.Kind.Sleepers() {
		return false
	}
	if o := slotOwner(b, slot); o != 0 && o != uint64(id) {
		return false
	}
	orig := b
	prev, hadPrev := g.sleptAt[id]
	b = withSlot(b.Release(uint64(id)), slot, uint64(id))
	g.bldgs.Set(bid, b)
	g.sleptAt[id] = bid
	sx, sy := construction.SleepCell(b.Kind, b.Facing, b.X, b.Y, slot)
	if !g.startJob(id, job.Record{Kind: job.Sleep, Target: bid, TX: sx, TY: sy, WorkLeft: needs.SleepWorkMin}) {
		g.bldgs.Set(bid, orig)
		if hadPrev {
			g.sleptAt[id] = prev
		} else {
			delete(g.sleptAt, id)
		}
		return false
	}
	for _, other := range g.bldgIDs {
		if other == bid {
			continue
		}
		if ob, ok := g.bldgs.Get(other); ok && ob.SlotOf(uint64(id)) >= 0 {
			g.bldgs.Set(other, ob.Release(uint64(id)))
		}
	}
	return true
}

// cmdAssignBedOwner makes owner (0: nobody) hold slot of bed: the owner
// gives up any other bed, and whoever held the slot loses it. A medical
// bed has no owner (bedrest.go).
func (g *Game) cmdAssignBedOwner(bed, owner entity.EntityID, slot int) {
	b, ok := g.bldgs.Get(bed)
	if !ok || !b.Kind.Sleeps() || slot < 0 || slot >= b.Kind.Sleepers() {
		return
	}
	if owner != 0 {
		if g.isMedicalBed(b) {
			return
		}
		if !g.colonist(owner) {
			return
		}
		if v, ok := g.vital.Get(owner); !ok || v.Status == health.Dead {
			return
		}
		if slotOwner(b, slot) == uint64(owner) {
			return
		}
		g.releaseBed(owner) // tick.go
		b, _ = g.bldgs.Get(bed)
	}
	g.bldgs.Set(bed, withSlot(b, slot, uint64(owner)))
}

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
