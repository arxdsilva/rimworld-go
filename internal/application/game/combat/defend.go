// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package combat

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Threat reach for Defend: an animal attacking a colonist this close, or a
// raider shooting at it from this far. The release radii apply while a
// colonist is already fleeing (or fighting a target), so it gets well
// clear before turning back to work (no work/flee flapping).
const (
	AnimalReach   = 4
	RaiderReach   = 14
	AnimalRelease = 8
	RaiderRelease = 18
	// FleeReach, FleeRelease: an unarmed colonist runs from an animal
	// coming for it from this far off, and keeps running until it is past
	// FleeRelease (PLAY-010).
	FleeReach   = 12
	FleeRelease = 16
	// FleeCalm is how long an unarmed colonist who fled holds back from
	// work after it last saw the threat.
	FleeCalm = 600
)

// Guard is how an undrafted colonist meets an attacker: armed colonists
// fight back, unarmed ones run and then hold back until they calm down.
type Guard struct {
	h   Host
	fl  Flight
	buf []Seen // attacking animal candidates, reused
	// CBT-4: the threat response (drafts), hostiles that are not
	// attacking (st) and cover (tac); nil in tests without them.
	drafts *Drafts
	st     *Strikes
	tac    *Tactics
	tiles  *Tiles // free ground for a flight's goal
	foes   []Seen // hostile candidates, reused
	rbuf   []Seen // raiders near, reused (EAI-022)
	escape []Seen // nearby threats along a flight, reused
}

// Reach is how close an attacking animal or raider must be for Defend to
// react: armed colonists only meet close attackers, unarmed ones run from
// far off, and a calm (fleeing) colonist keeps running until the threat is
// past the wider release radius.
func Reach(armed, calm bool) (animalR, raiderR int) {
	switch {
	case armed:
		return AnimalReach, RaiderReach
	case calm:
		return FleeRelease, RaiderRelease
	}
	return FleeReach, RaiderReach
}

// ThreatTo reports t's tile and distance when it is attacking id at
// (x, y) within reach (animals from close by, raiders from further off).
func (gd *Guard) ThreatTo(id, t entity.EntityID, x, y, animalR, raiderR int) (tx, ty, d int, ok bool) {
	h := gd.h
	if isAnimal, attacking := gd.fl.AnimalOn(t, id); isAnimal {
		if !attacking {
			return 0, 0, 0, false
		}
		ax, ay, _ := h.Pos(t)
		d := weapon.Distance(x, y, ax, ay)
		return ax, ay, d, d < animalR
	}
	rx, ry, ok := h.RaiderOn(t, id)
	if !ok {
		return 0, 0, 0, false
	}
	d = weapon.Distance(x, y, rx, ry)
	return rx, ry, d, d < raiderR
}

// NearestThreat is the closest animal or raider attacking id at (x, y)
// within reach (threat 0: none).
func (gd *Guard) NearestThreat(id entity.EntityID, x, y, animalR, raiderR int) (threat entity.EntityID, tx, ty, best int) {
	h := gd.h
	// attacking animals from the creature index: the nearest within
	// animalR, ties to the lower id
	if animalR > 0 {
		gd.buf = h.Near(x, y, animalR-1, Animals, gd.buf[:0])
		s, ok := nearest(gd.buf, x, y, func(s Seen) bool {
			_, attacking := gd.fl.AnimalOn(s.ID, id)
			return attacking
		})
		if ok {
			threat, tx, ty, best = s.ID, s.X, s.Y, weapon.Distance(x, y, s.X, s.Y)
		}
	}
	gd.fl.CheckThreats(id, x, y, animalR, raiderR, threat, best)
	if raiderR <= 0 {
		return threat, tx, ty, best
	}
	// EAI-022: raiders from the creature index, within raiderR; an animal
	// keeps a tie, between raiders the lower id wins
	raider := false
	gd.rbuf = h.Near(x, y, raiderR-1, Raiders, gd.rbuf[:0])
	for _, s := range gd.rbuf {
		px, py, d, ok := gd.ThreatTo(id, s.ID, x, y, animalR, raiderR)
		if ok && (threat == 0 || d < best || d == best && raider && s.ID < threat) {
			threat, tx, ty, best, raider = s.ID, px, py, d, true
		}
	}
	return threat, tx, ty, best
}

// Defend reacts to an animal or raider attacking an undrafted colonist:
// armed colonists fight back, unarmed ones run and then hold back from
// work and orders until they calm down. It reports whether it acted.
func (gd *Guard) Defend(id entity.EntityID, j job.Record) bool {
	h, fl := gd.h, gd.fl
	px, py, _ := h.Pos(id)
	now := h.Now()
	armed := h.FightsBack(id) // its threat response: a pacifist, or one set to flee, runs instead
	calm := false
	if until, ok := fl.Calm(id); ok {
		if calm = now < until && !armed && !gd.ignores(id); !calm {
			fl.ForgetCalm(id)
		}
	}
	if gd.StepOut(id, j, px, py) {
		return true // turret_step.go (EAI-012): clear of a sparking turret
	}
	if gd.ignores(id) {
		return false // CBT-4: set to ignore threats, it carries on
	}
	ar, rr := Reach(armed, calm)
	threat, tx, ty, best := gd.NearestThreat(id, px, py, ar, rr)
	if threat == 0 && gd.st != nil && (calm || (now+uint64(id))%foeEvery == 0) {
		threat, tx, ty, best = gd.nearestFoe(id, px, py, armed, calm, now)
	}
	if threat == 0 {
		return calm // still shaken: hold position, start no work
	}
	if armed {
		// The automatic response cannot replace an active player order.
		// Explicit flight/ignore responses and immediate environmental danger
		// keep their existing precedence.
		if j.Forced && j.Active() {
			return true
		}
		if gd.tac != nil && gd.tac.Covering(id, j) {
			return true // CBT-4: stepping into cover
		}
		if j.Kind == job.Attack && j.State != job.Fail {
			if j.Target == threat {
				return true
			}
			// keep the current target (and its aim) unless the new one is much closer
			if d, ok := gd.holds(id, j.Target, px, py, now); ok && best > d-SwitchMargin {
				return true
			}
		}
		d := weapon.DefOf(h.Weapon(id))
		if d.Ranged && gd.tac != nil && !gd.st.sight.CanHit(px, py, d, tx, ty) {
			cx, cy, _, _, ok := gd.tac.BestCover(id, px, py, CoverRadius, d, tx, ty)
			if ok && (cx != px || cy != py) && gd.fl.Reachable(id, cx, cy) {
				h.Interrupt(id, job.Interrupted)
				h.StartJob(id, job.Record{Kind: job.Move, TX: cx, TY: cy})
				return true // CAI-5000-style cast position: move to a reachable firing tile first
			}
		}
		if gd.tac != nil && gd.tac.TakeCover(id, threat, px, py, d, tx, ty) {
			return true // CBT-4: a step into cover first
		}
		h.Interrupt(id, job.Interrupted)
		h.StartJob(id, job.Record{Kind: job.Attack, Target: threat, TX: tx, TY: ty, WorkLeft: AimTicks(h, id, d)})
		return true
	}
	if !calm {
		h.Fled(id, threat)
	}
	fl.SetCalm(id, now+FleeCalm) // calm down only once the threat is gone
	return gd.runFlight(id, j, px, py, Seen{ID: threat, X: tx, Y: ty}, calm, now)
}

// InCombat reports a colonist in a fight: drafted, attacking, or shaken
// after fleeing (a live calm window). Movement keeps such colonists off
// each other's tiles (PLAY-010); outside combat they may share one.
func (gd *Guard) InCombat(id entity.EntityID) bool {
	h := gd.h
	if h.Drafted(id) {
		return true
	}
	if j, ok := h.Job(id); ok && j.Kind == job.Attack && j.State != job.Fail {
		return true
	}
	until, ok := gd.fl.Calm(id)
	return ok && h.Now() < until
}

// ignores reports a colonist set to ignore threats (CBT-4).
func (gd *Guard) ignores(id entity.EntityID) bool {
	return gd.drafts != nil && gd.drafts.Response(id) == RespondIgnore
}

// holds reports the current target t still worth fighting and its
// distance: attacking id within release, or a hostile within the
// engage reach and release margin.
func (gd *Guard) holds(id, t entity.EntityID, x, y int, now uint64) (int, bool) {
	if _, _, d, ok := gd.ThreatTo(id, t, x, y, AnimalRelease, RaiderRelease); ok {
		return d, true
	}
	if !gd.potentialFoe(t, now) {
		return 0, false
	}
	tx, ty, ok := gd.h.Pos(t)
	d := weapon.Distance(x, y, tx, ty)
	return d, ok && d <= EngageReach(weapon.DefOf(gd.h.Weapon(id)).Range)+SwitchMargin
}

func (gd *Guard) potentialFoe(id entity.EntityID, now uint64) bool {
	if gd.h.FleeingRaider(id) {
		return false
	}
	return gd.st != nil && gd.st.foeID(id, now)
}

// nearestFoe is the nearest hostile in reach of colonist id at (x, y)
// that is not (yet) attacking it (CBT-4, threat response): visible animals
// and unarmed hostiles require sight, while armed raiders may be routed to a
// firing position within the engage reach of the weapon. Ties go to the
// lower id.
//
// The calm-flight reach remains the wider release reach.
// flight reach (the release reach while calm). Ties go to the lower id.
func (gd *Guard) nearestFoe(id entity.EntityID, x, y int, armed, calm bool, now uint64) (entity.EntityID, int, int, int) {
	h := gd.h
	animalR, raiderR := FleeReach, RaiderReach
	switch {
	case armed:
		animalR = EngageReach(weapon.DefOf(h.Weapon(id)).Range)
		raiderR = animalR
	case calm:
		animalR, raiderR = FleeRelease, RaiderRelease
	}
	gd.foes = h.Near(x, y, max(animalR, raiderR), Animals|Raiders, gd.foes[:0])
	s, ok := nearest(gd.foes, x, y, func(s Seen) bool {
		r := raiderR
		if _, beast := h.Animal(s.ID); beast {
			r = animalR
		}
		if weapon.Distance(x, y, s.X, s.Y) > r || !gd.potentialFoe(s.ID, now) {
			return false
		}
		// Armed raiders are selected by reach, not current line of sight; the
		// firing position search below can move the colonist around a blocker.
		_, beast := h.Animal(s.ID)
		if !armed || beast {
			return gd.st.sight.Line(x, y, s.X, s.Y)
		}
		return true
	})
	if !ok {
		return 0, 0, 0, 0
	}
	return s.ID, s.X, s.Y, weapon.Distance(x, y, s.X, s.Y)
}
