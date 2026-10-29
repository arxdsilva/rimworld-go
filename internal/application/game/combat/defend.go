package combat

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
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
	if a, ok := h.Animal(t); ok {
		if a.Mode != animal.Attack || a.Target != id {
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
			a, ok := h.Animal(s.ID)
			return ok && a.Mode == animal.Attack && a.Target == id
		})
		if ok {
			threat, tx, ty, best = s.ID, s.X, s.Y, weapon.Distance(x, y, s.X, s.Y)
		}
	}
	gd.fl.CheckThreats(id, x, y, animalR, raiderR, threat, best)
	for _, rid := range h.RaiderIDs() {
		if px, py, d, ok := gd.ThreatTo(id, rid, x, y, animalR, raiderR); ok && (threat == 0 || d < best) {
			threat, tx, ty, best = rid, px, py, d
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
	armed := h.FightsBack(id) // a pacifist runs instead
	calm := false
	if until, ok := fl.Calm(id); ok {
		if calm = now < until && !armed; !calm {
			fl.ForgetCalm(id)
		}
	}
	ar, rr := Reach(armed, calm)
	threat, tx, ty, best := gd.NearestThreat(id, px, py, ar, rr)
	if threat == 0 {
		return calm // still shaken: hold position, start no work
	}
	if armed {
		if j.Kind == job.Attack && j.State != job.Fail {
			if j.Target == threat {
				return true
			}
			// keep the current target (and its aim) unless the new one is much closer
			if _, _, d, ok := gd.ThreatTo(id, j.Target, px, py, AnimalRelease, RaiderRelease); ok && best > d-SwitchMargin {
				return true
			}
		}
		h.Interrupt(id, job.Interrupted)
		h.StartJob(id, job.Record{Kind: job.Attack, Target: threat, TX: tx, TY: ty, WorkLeft: AimTicks(weapon.DefOf(h.Weapon(id)))})
		return true
	}
	if !calm {
		h.Fled(id, threat)
	}
	fl.SetCalm(id, now+FleeCalm) // calm down only once the threat is gone
	running := calm && j.Kind == job.Move && j.State == job.Goto && !j.Forced
	if running && (now+uint64(id))%30 != 0 {
		return true // keep running
	}
	fx, fy, ok := fl.FleeGoal(id, px, py, tx, ty)
	if !ok {
		// cornered: stay put rather than work, sleep or walk an order
		if !running {
			fl.DropForFlight(id, j)
		}
		return true
	}
	fl.DropForFlight(id, j)
	h.StartJob(id, job.Record{Kind: job.Move, TX: fx, TY: fy})
	return true
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
