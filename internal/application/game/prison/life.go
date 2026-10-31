package prison

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// A prisoner's life (ENG-003, M8), run for every prisoner on the raid
// roster: its needs fall and its wounds heal as a colonist's do, it
// sleeps in its bed when tired and potters about its room (it paths like
// a raider: doors hold it), and it never works. A warden brings it food
// (the food band below) and talks to it. Released, it walks to the map
// edge and goes home.

const (
	// FoodLow and FoodFed are the food band: below Low a prisoner wants a
	// meal brought, until it is fed past Fed.
	FoodLow = needs.CriticalHunger + 1500
	FoodFed = 8000
	// wanderEvery is the ticks between an idle prisoner's strolls
	// (staggered by id); roam how far from its bed it goes.
	wanderEvery = 900
	roam        = 3
)

// Live is one tick of prisoner (or released pawn, or visitor) id.
func (p *Prison) Live(id entity.EntityID, now uint64) {
	h := p.h
	r := h.Held(id)
	v, _ := h.Vital(id)
	if v.Status == health.Dead {
		delete(p.st, id) // the body stays as a raider's does
		r.Leaving = false
		h.SetHeld(id, r)
		return
	}
	if h.Captured(id) {
		return // in a captor's arms
	}
	v = h.Heal(id, v, now) // faster in bed and tended
	n := h.Needs(id)
	if v.Status == health.Downed {
		p.FoodBand(id, h.DownedHunger(id, now))
		return
	}
	j, _ := h.Job(id)
	asleep := j.Kind == job.Sleep && j.State == job.Do
	if asleep {
		n = needs.SleepTick(n, now)
	} else {
		n = h.Decay(id, n, now)
	}
	h.SetNeeds(id, n)
	if r.Leaving {
		p.walkOut(id, r, j, now)
		return
	}
	if p.breakOut(id, now) { // escape.go (CBT-4)
		return
	}
	p.FoodBand(id, n)
	switch {
	case asleep:
		if n.Rest >= needs.WakeRest {
			h.SetJob(id, job.Record{})
		}
	case j.State == job.Complete || j.State == job.Fail:
		h.ReleaseOwner(id)
		h.SetJob(id, job.Record{})
	case j.Active():
	case n.Tired():
		p.sleep(id)
	case (now+uint64(id)*7)%wanderEvery == 0:
		if _, held := h.Owner(id); !held { // a warden is on the way: stay put
			p.stroll(id)
		}
	}
}

// FoodBand moves the Hungry flag with hysteresis: on at or below FoodLow,
// off at or above FoodFed.
func (p *Prison) FoodBand(id entity.EntityID, n needs.Pair) {
	r := p.h.Held(id)
	switch {
	case !r.Hungry && n.Hunger <= FoodLow:
		r.Hungry = true
	case r.Hungry && n.Hunger >= FoodFed:
		r.Hungry = false
	default:
		return
	}
	p.h.SetHeld(id, r)
}

// sleep sends a tired prisoner to its bed (or lies down where it is
// without one).
func (p *Prison) sleep(id entity.EntityID) {
	h := p.h
	if bed, x, y, ok := h.SleepSpot(id); ok {
		if h.StartJob(id, job.Record{Kind: job.Sleep, Target: bed, TX: x, TY: y, WorkLeft: needs.SleepWorkMin}) {
			return
		}
	}
	x, y, _ := h.Pos(id)
	h.SetJob(id, job.Record{Kind: job.Sleep, State: job.Do, TX: x, TY: y, WorkLeft: needs.SleepWorkMin})
}

// stroll walks an idle prisoner to a free tile near its bed in the same
// room (prisoners path like raiders: doors stop them).
func (p *Prison) stroll(id entity.EntityID) {
	h := p.h
	bx, by, room, ok := h.BedAt(id)
	if !ok {
		return
	}
	x, y := bx+h.Roll(2*roam+1)-roam, by+h.Roll(2*roam+1)-roam
	if h.FreeInRoom(x, y, room) {
		h.StartJob(id, job.Record{Kind: job.Move, TX: x, TY: y})
	}
}

const (
	// giveUp and retry are the raid package's RaiderGiveUp and
	// RaiderRetry: a released pawn still on the map after a day goes
	// home; one that found no way out waits before trying again.
	giveUp = clock.TicksPerDay
	retry  = 120
)

// walkOut takes a released pawn (or visitor, record r) to the map edge;
// there, or after a day of trying, it goes home (moved from the game's
// prisoners.go, CBT-4).
func (p *Prison) walkOut(id entity.EntityID, r Held, j job.Record, now uint64) {
	h := p.h
	x, y, _ := h.Pos(id)
	tx, ty, edge, ok := h.Exit(x, y)
	if edge || now-r.Idle >= giveUp {
		h.Depart(id)
		return
	}
	if j.Active() && j.Kind == job.Move || now < r.RetryAt {
		return
	}
	if ok && h.StartJob(id, job.Record{Kind: job.Move, TX: tx, TY: ty}) {
		return
	}
	r.RetryAt = now + retry
	h.SetHeld(id, r)
}
