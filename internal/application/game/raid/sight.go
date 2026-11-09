package raid

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Sight and memory (EAI-010). wiki: Raid, raiders make for the colony and
// fight whoever they come across. A raider walks on the colony
// (Senses.Home) until it knows of a foe: a colonist or an ally in clear
// sight within SightRange, one it remembers, or anyone once it is at the
// colony (within SightRange of home). It remembers the foe it last saw for
// raiderMemory ticks, and the foe any raider of its group last saw for
// groupMemory ticks, so a mate behind a wall still knows where the fight
// is. A wound turns it on the shooter, remembered as seen. A raider in an
// attack keeps its target while it knows of it, looks again every
// targetEvery ticks (offset by id, EAI-021), and lets go of a target it has
// lost track of; it turns on a shooter only when that is switchMargin
// closer than its target, so two foes never trade it back and forth
// (TestRaiderTargetMemoryNoFlap). The memory is saved with the raider and
// its group. Without Senses (unit hosts) a raider knows of everyone.
const (
	// raiderMemory and groupMemory: how long a raider, and its group,
	// remember a foe last seen (original tuning).
	raiderMemory = 600
	groupMemory  = 1200
	// targetEvery: a raider on the move picks its target this often (and
	// an attacking one checks it still knows of its target).
	targetEvery = 15
	// switchMargin is combat.SwitchMargin: a shooter must be this much
	// closer than the target for a raider to turn on it.
	switchMargin = 3
	// homeSlack: a walk on the colony ends within this of its goal;
	// homeStop: the goal is this far short of home (home is often a bed
	// indoors), well inside SightRange, where it knows of everyone;
	// advanceLeg: it walks there this far at a time, so each search is
	// short and a far colony is reached leg by leg (original tuning).
	homeSlack  = 4
	homeStop   = SightRange / 2
	advanceLeg = 2 * SightRange
)

// eyes is what the raiders see: Senses, and home kept for the tick.
type eyes struct {
	h      Host
	s      Senses
	homeAt uint64
	homeOK bool
	hx, hy int
}

// home is where the colony lives, asked once a tick.
func (e *eyes) home(now uint64) (int, int) {
	if !e.homeOK || e.homeAt != now {
		e.hx, e.hy = e.s.Home()
		e.homeAt, e.homeOK = now, true
	}
	return e.hx, e.hy
}

// recall is the foes raider r remembers at now: its own and its group's
// (0: none).
func (e *eyes) recall(r *Raider, now uint64) [2]entity.EntityID {
	var m [2]entity.EntityID
	if r.Foe != 0 && now < r.FoeAt+raiderMemory {
		m[0] = r.Foe
	}
	if gs := e.h.Groups(); r.Group < len(gs) {
		if g := gs[r.Group]; g.Foe != 0 && g.Foe != m[0] && now < g.FoeAt+groupMemory {
			m[1] = g.Foe
		}
	}
	return m
}

// knows: raider id (record r) at (x, y) knows of foe t at (tx, ty).
func (e *eyes) knows(id entity.EntityID, r *Raider, t entity.EntityID, x, y, tx, ty int, now uint64) bool {
	if e.s == nil {
		return true
	}
	if m := e.recall(r, now); t == m[0] || t == m[1] {
		return true
	}
	return e.atColony(x, y, now) || e.sees(id, x, y, tx, ty)
}

// atColony: (x, y) is within SightRange of home.
func (e *eyes) atColony(x, y int, now uint64) bool {
	hx, hy := e.home(now)
	return weapon.Distance(x, y, hx, hy) <= SightRange
}

// sees: raider id at (x, y) has (tx, ty) within SightRange in clear sight.
func (e *eyes) sees(id entity.EntityID, x, y, tx, ty int) bool {
	return e.s != nil && weapon.Distance(x, y, tx, ty) <= SightRange && e.s.Visible(id, tx, ty)
}

// saw: raider r saw foe t at now; its group learns of it too.
func (e *eyes) saw(r *Raider, t entity.EntityID, now uint64) {
	r.Foe, r.FoeAt = t, now
	if gs := e.h.Groups(); r.Group < len(gs) {
		gs[r.Group].Foe, gs[r.Group].FoeAt = t, now
	}
}

// shot: raider id (record r) has injury hurt; more than it last felt is a
// wound, and the colonist or ally who dealt it is remembered as seen. It
// returns the shooter (0: none) and whether r changed (the caller saves
// it).
func (e *eyes) shot(id entity.EntityID, r *Raider, hurt int, now uint64) (entity.EntityID, bool) {
	if e.s == nil || hurt == r.Felt {
		return 0, false
	}
	more := hurt > r.Felt
	r.Felt = hurt
	if !more {
		return 0, true
	}
	by := e.s.Shooter(id)
	if by != 0 {
		e.saw(r, by, now)
	}
	return by, true
}

// turn: raider id (record r) at (x, y), attacking with j, lets go of its
// target: shot by another foe switchMargin closer, or, on its look, lost
// track of it. A target it still sees is remembered afresh.
func (rs *Raiders) turn(id entity.EntityID, r *Raider, j job.Record, shot entity.EntityID, x, y int, now uint64) bool {
	e, h := &rs.eye, rs.h
	if e.s == nil {
		return false
	}
	tx, ty, creature := h.Pos(j.Target)
	if shot != 0 && shot != j.Target {
		if sx, sy, ok := h.Pos(shot); ok && (!creature || weapon.Distance(x, y, sx, sy)+switchMargin <= weapon.Distance(x, y, tx, ty)) {
			h.Interrupt(id, job.Cancelled)
			return true
		}
	}
	if !creature || (now+uint64(id))%targetEvery != 0 {
		return false // a turret is kept while TurretFor offers it (targetAt)
	}
	if e.sees(id, x, y, tx, ty) {
		e.saw(r, j.Target, now)
		h.SetRaider(id, *r)
		return false
	}
	if m := e.recall(r, now); j.Target == m[0] || j.Target == m[1] || e.atColony(x, y, now) {
		return false
	}
	h.Interrupt(id, job.Cancelled)
	return true
}

// advanceHome walks raider id at (x, y), knowing of no foe, on the
// colony: a leg of at most advanceLeg towards homeStop short of home,
// planned again once it ends (or if it was walking away from home).
func (rs *Raiders) advanceHome(id entity.EntityID, r *Raider, j job.Record, x, y int, now uint64) {
	h := rs.h
	hx, hy := rs.eye.home(now)
	r.Idle, r.Advancing = 0, true
	if j.Kind != job.Move || !j.Active() || weapon.Distance(j.TX, j.TY, hx, hy) > weapon.Distance(x, y, hx, hy) {
		lx, ly := advanceGoal(x, y, hx, hy)
		if wx, wy, ok := h.WalkableNear(lx, ly, homeSlack); ok &&
			!h.StartJob(id, job.Record{Kind: job.Move, TX: wx, TY: wy}) && rs.unreachable(id) && !h.StartBreach(id, lx, ly) {
			r.RetryAt = now + RaiderRetry
		}
	}
	h.SetRaider(id, *r)
}

// advanceGoal is the end of the next leg from (x, y) on home (hx, hy):
// homeStop short of home, at most advanceLeg away.
func advanceGoal(x, y, hx, hy int) (int, int) {
	d := weapon.Distance(x, y, hx, hy)
	if d <= homeStop {
		return hx, hy
	}
	gx, gy := hx+(x-hx)*homeStop/d, hy+(y-hy)*homeStop/d
	if l := weapon.Distance(x, y, gx, gy); l > advanceLeg {
		gx, gy = x+(gx-x)*advanceLeg/l, y+(gy-y)*advanceLeg/l
	}
	return gx, gy
}

// Advancing: raider id knows of no foe and is walking on the colony (the
// sheet's "Advancing on the colony", EAI-010).
func (rs *Raiders) Advancing(id entity.EntityID) bool {
	r := rs.h.Raider(id)
	return r.Advancing && r.Mode != Fleeing
}
