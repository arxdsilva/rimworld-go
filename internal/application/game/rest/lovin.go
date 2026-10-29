package rest

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// Lovin is couples' lovin' in a shared double bed (ENG-003, M5; moved
// from game/beds.go by BED-1). Every LovinPeriod ticks, staggered by id
// (LovinStagger, the game's social duty), a couple lying asleep in the
// same double bed may make love: an age-based mean interval sets the
// chance and the cooldown after it. Both run job.Lovin for LovinTicks,
// then each gets mood.Lovin. Never while downed, in pain or dazed, or
// while either thinks badly of the other. No pregnancy. The cooldown is
// saved with the gatherings and the roll is on the social stream: both
// stay on the host (Couples).
type Lovin struct{ h Host }

const (
	LovinPeriod  = 2500
	LovinStagger = 37
	LovinTicks   = 250
	// lovinMaxPain and lovinMinConscious gate lovin' (permille).
	lovinMaxPain      = 400
	lovinMinConscious = 700
)

// lovinInterval is the mean ticks between a couple's lovin' by age: an
// hour and a half up to 22, then rarer with the years.
var lovinInterval = social.Curve{{X: 16, Y: 3750}, {X: 22, Y: 3750}, {X: 30, Y: 10000}, {X: 50, Y: 30000}, {X: 75, Y: 90000}}

// Check is couple member id's staggered lovin' turn (the lower id of the
// two decides for both).
func (l *Lovin) Check(id entity.EntityID, now uint64) {
	p := l.h.Mate(id)
	if p == 0 || p < id || l.h.Mate(p) != id {
		return
	}
	bid, ok := l.Bed(id, p)
	if !ok || !l.fit(id, p) {
		return
	}
	if now < l.h.LovinAt(id) || now < l.h.LovinAt(p) {
		return
	}
	age := (l.h.Age(id) + l.h.Age(p)) / 2
	mean := max(int64(lovinInterval.Eval(int32(age))), LovinPeriod)
	if int64(l.h.SocialRoll(1000)) >= LovinPeriod*1000/mean {
		return
	}
	l.h.SetLovinAt(id, now+uint64(mean))
	l.h.SetLovinAt(p, now+uint64(mean))
	for _, x := range [2]entity.EntityID{id, p} {
		px, py := l.h.Pos(x)
		o := p
		if x == p {
			o = id
		}
		l.h.SetJob(x, job.Record{Kind: job.Lovin, State: job.Do, Target: o, Dest: bid, TX: px, TY: py, WorkLeft: LovinTicks})
	}
}

// Bed is the double bed a and b both lie asleep in.
func (l *Lovin) Bed(a, b entity.EntityID) (entity.EntityID, bool) {
	bid, _ := l.h.SleptAt(a)
	if other, _ := l.h.SleptAt(b); bid == 0 || other != bid {
		return 0, false
	}
	bed, ok := l.h.Building(bid)
	if !ok || bed.Kind.Sleepers() != 2 || bed.SlotOf(uint64(a)) < 0 || bed.SlotOf(uint64(b)) < 0 {
		return 0, false
	}
	for _, x := range [2]entity.EntityID{a, b} {
		if j := l.h.Job(x); j.Kind != job.Sleep || j.State != job.Do {
			return 0, false
		}
	}
	return bid, true
}

// fit: both standing (not downed), not in pain, clear-headed and fond
// enough of each other.
func (l *Lovin) fit(a, b entity.EntityID) bool {
	for _, x := range [2]entity.EntityID{a, b} {
		if !l.h.Standing(x) {
			return false
		}
		if pain, conscious, ok := l.h.Hurt(x); ok && (pain > lovinMaxPain || conscious < lovinMinConscious) {
			return false
		}
	}
	return l.h.Opinion(a, b) >= 0 && l.h.Opinion(b, a) >= 0
}

// Do runs a lovin' job: when it ends with the partner still there, both
// remember it fondly; a partner who left ends it without one.
func (l *Lovin) Do(id entity.EntityID, j job.Record) {
	if pj := l.h.Job(j.Target); pj.Kind != job.Lovin || pj.Target != id {
		j.State = job.Complete
		l.h.SetJob(id, j)
		return
	}
	j.WorkLeft--
	if j.WorkLeft <= 0 {
		j.State = job.Complete
		l.h.RememberWith(id, mood.Lovin, j.Target)
	}
	l.h.SetJob(id, j)
}
