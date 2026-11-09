package rage

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// Tame guards: a tame predator fights off raiders and hostile or enraged
// wild animals near it.

// guardTarget is the nearest threat a tame guard animal should fight: a
// standing raider, or a wild animal attacking the colony.
func (r *Rage) guardTarget(id entity.EntityID, pos Pos) entity.EntityID {
	best, _, _ := r.h.Nearest(pos.X, pos.Y, guardRadius, Raiders|Animals, Sight{R: r, Self: id, Now: r.h.Now(), Keep: guardThreat})
	return best
}

// guardThreat keeps a standing raider or a wild animal attacking the
// colony.
func guardThreat(s Sight, o entity.EntityID) bool {
	h := s.R.h
	if h.IsRaider(o) {
		return h.StandingRaider(o)
	}
	a, ok := h.Animal(o)
	return o != s.Self && ok && (!a.Tame || a.Manhunter > s.Now) && !a.Dead() && (h.Hostile(a) || a.Enraged(s.Now)) // a tame manhunter too (EAI-0)
}

// GuardCheck: a tame guard (a predator) fights off threats near it,
// looking for one every animal.ThreatEvery ticks. It reports true while it
// fights.
func (r *Rage) GuardCheck(id entity.EntityID, a *animal.State, pos Pos, now uint64) bool {
	if a.Target != 0 && !r.CombatAlive(a.Target) {
		a.Target = 0
	}
	if a.Target == 0 && (now+uint64(id))%animal.ThreatEvery == 0 {
		a.Target = r.guardTarget(id, pos)
	}
	if a.Target == 0 {
		if a.Mode == animal.Attack {
			a.Mode, a.Path, a.PathI = animal.Wander, nil, 0
		}
		return false
	}
	a.Feed, a.FeedLeft = animal.FeedTarget{}, 0 // a sleeping guard wakes
	r.Chase(id, a, now)
	return true
}

// GuardBreaksHunt: a tame predator out hunting (PLAY-017) drops the hunt
// for a threat its guard duty would fight, looked for on GuardCheck's
// cadence; GuardCheck then takes the threat on.
func (r *Rage) GuardBreaksHunt(id entity.EntityID, a *animal.State, pos Pos, now uint64) {
	if (now+uint64(id))%animal.ThreatEvery != 0 {
		return
	}
	if t := r.guardTarget(id, pos); t != 0 {
		EndFight(a)
		a.Target = t
	}
}

// CombatAlive: a raider still standing or a living animal.
func (r *Rage) CombatAlive(id entity.EntityID) bool {
	if o, ok := r.h.Animal(id); ok {
		return !o.Dead()
	}
	return r.h.StandingRaider(id)
}
