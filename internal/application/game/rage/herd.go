package rage

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// Herd defence (EAI-003): a wild animal hurt by someone calls its herd
// (its spawn group, animal.State.Herd). Fighter mates within herdRadius
// (territorial animals, predators, legendaries) that can reach the foe
// join the fight; the rest of the herd runs from it.
//
// Hysteresis: a herd calls at most once per callEvery ticks
// (Memory.Call), however many blows land; a mate already in a fight keeps
// it; the joiners are let go by the leashes every fight already has (a
// territorial animal past territorialLeash, a break-off when badly hurt),
// and only a new call, callEvery later, brings them back
// (TestHerdDefenceNoFlap).
const (
	// callEvery: a herd calls to arms at most this often.
	callEvery = 300
	// herdRadius: herd mates this close answer a call.
	herdRadius = 12
)

// HerdCall: wild animal id (state a), hurt by by at now, calls its herd.
func (r *Rage) HerdCall(id entity.EntityID, a animal.State, by entity.EntityID, now uint64) {
	if a.Tame || a.Herd == 0 || by == 0 || by == id {
		return
	}
	if c, ok := r.m.Call[a.Herd]; ok && now < c.At+callEvery {
		return
	}
	if o := r.h.Ref(by); o != nil && !o.Tame && o.Herd == a.Herd {
		return // a squabble inside the herd
	}
	pos := r.pos(id)
	r.m.call(a.Herd, Call{Foe: by, X: pos.X, Y: pos.Y, At: now})
	for _, mid := range r.h.Within(pos.X, pos.Y, herdRadius, Animals) {
		m, ok := r.h.Animal(mid)
		if mid == id || !ok || m.Herd != a.Herd || m.Tame || m.Dead() || m.Downed || m.Enraged(now) {
			continue
		}
		if !answer(r, mid, &m, by, now) {
			continue
		}
		m.NextDecide = now
		r.h.SetAnimal(mid, m)
	}
}

// answer is herd mate id's (state m) answer to a call against by: a
// fighter that can reach it joins the fight (one already fighting keeps
// its own), anyone else runs. It reports whether m changed.
func answer(r *Rage, id entity.EntityID, m *animal.State, by entity.EntityID, now uint64) bool {
	if animal.Fighter(m.Species) {
		if m.Mode == animal.Attack && m.Target != 0 || !r.CanFightBack(id, *m, by) {
			return false
		}
		FightBack(m, by, false)
		return true
	}
	if m.Mode == animal.Flee && now < m.FleeUntil {
		return false // already running
	}
	m.Target, m.HuntSince, m.Attacker = 0, 0, by
	m.Mode, m.FleeUntil = animal.Flee, now+animal.HurtFleeTicks
	m.Feed, m.FeedLeft, m.Path, m.PathI = animal.FeedTarget{}, 0, nil, 0 // a sleeper wakes, a grazer drops its meal
	return true
}

// Defending: wild animal a fights the foe its herd called against (the
// inspector's "Defending its herd").
func (r *Rage) Defending(a animal.State) bool {
	if a.Tame || a.Herd == 0 || a.Mode != animal.Attack || a.Target == 0 {
		return false
	}
	c, ok := r.m.Call[a.Herd]
	return ok && c.Foe == a.Target
}
