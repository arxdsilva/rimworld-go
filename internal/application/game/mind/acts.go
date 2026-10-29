package mind

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/sim/entity"
)

// act runs one tick of break k for id (its job j, the break ending at
// until).
func (m *Mind) act(id entity.EntityID, k mentalbreak.Kind, j job.Record, now, until uint64) {
	h := m.h
	switch k {
	case mentalbreak.SadWander:
		if !(j.Active() && j.Kind == job.Wander) {
			h.Wander(id, 200+h.Roll(300))
		}
	case mentalbreak.FoodBinge:
		if !(j.Active() && j.Kind == job.Wander) && !h.Binge(id) {
			h.Wander(id, 300) // nothing to eat: wanders, looks again after
		}
	case mentalbreak.HideInRoom:
		if !(j.Active() && j.Kind == job.Hide) {
			h.Hide(id, int(until-now))
		}
	case mentalbreak.InsultingSpree, mentalbreak.TargetedInsult:
		m.insult(id, k, j, now)
	case mentalbreak.Berserk:
		m.berserk(id, j, now)
	case mentalbreak.MurderousRage:
		m.rage(id, j, now)
	case mentalbreak.Catatonic:
		if h.Standing(id) {
			h.Down(id) // stood up early (a tend): it collapses again
		}
	}
}

// wander keeps id strolling unless it already is.
func (m *Mind) wander(id entity.EntityID, j job.Record) {
	if !(j.Active() && j.Kind == job.Wander) {
		m.h.Wander(id, 200+m.h.Roll(300))
	}
}

// insult runs an insulting spree (a new victim after every insult, at most
// every RetargetTicks) or a targeted one (the same colonist until it
// dies, then the break ends). It walks up to the victim and insults it
// every InsultGap; each insult may start a social fight, which ends the
// break.
func (m *Mind) insult(id entity.EntityID, k mentalbreak.Kind, j job.Record, now uint64) {
	h := m.h
	t := m.track[id]
	if k == mentalbreak.TargetedInsult {
		if t.Target == 0 || !h.Alive(t.Target) {
			m.end(id, k)
			return
		}
	} else {
		if t.Target != 0 && !h.Standing(t.Target) {
			t.Target, t.Next = 0, now+RetargetTicks // lost its victim
		}
		if t.Target == 0 && now >= t.Next {
			if t.Target = m.pick(m.others(id, false)); t.Target == 0 {
				t.Next = now + RetargetTicks // nobody about: look again later
			}
		}
		m.track[id] = t
	}
	if t.Target == 0 {
		m.wander(id, j)
		return
	}
	if !m.near(id, t.Target, InsultRange) {
		if j.Active() && j.Kind == job.Move && (now+uint64(id))%repathTicks != 0 {
			return
		}
		if !h.Approach(id, t.Target) && k == mentalbreak.InsultingSpree {
			t.Target, t.Next = 0, now+RetargetTicks // out of reach: someone else later
			m.track[id] = t
		}
		return
	}
	if now < t.Next {
		return // between insults: glares
	}
	h.Insult(id, t.Target)
	t.Next = now + InsultGap
	if k == mentalbreak.InsultingSpree {
		t.Target = 0
	}
	if b, _, ok := h.Break(id); ok && b == k {
		m.track[id] = t // (a fight replaced the break: nothing to keep)
	}
}

// berserk attacks the nearest creature until it is down, then moves on
// to the next, choosing at most every RetargetTicks; with nobody near it
// wanders. The break ends when the berserker is downed (Downed) or its
// time is up.
func (m *Mind) berserk(id entity.EntityID, j job.Record, now uint64) {
	h := m.h
	t := m.track[id]
	if t.Target != 0 && (!h.Attackable(t.Target) || !m.near(id, t.Target, 2*BerserkRadius)) {
		t.Target = 0
	}
	if t.Target == 0 && now >= t.Next {
		t.Target, t.Next = h.NearestCreature(id, BerserkRadius), now+RetargetTicks
	}
	if t.Target != 0 && !h.Assault(id, t.Target) {
		t.Target = 0 // out of reach
	}
	m.track[id] = t
	if t.Target == 0 {
		m.wander(id, j)
	}
}

// rage hunts its one target, standing or downed, until it is dead (then
// the break ends). Out of reach, it wanders and tries again every
// RetargetTicks.
func (m *Mind) rage(id entity.EntityID, j job.Record, now uint64) {
	h := m.h
	t := m.track[id]
	if t.Target == 0 || !h.Alive(t.Target) {
		m.end(id, mentalbreak.MurderousRage)
		return
	}
	if now >= t.Next && h.Assault(id, t.Target) {
		return
	}
	if now >= t.Next {
		t.Next = now + RetargetTicks
		m.track[id] = t
	}
	m.wander(id, j)
}

// near reports b within r tiles of a (Chebyshev).
func (m *Mind) near(a, b entity.EntityID, r int) bool {
	ax, ay, ok := m.h.Pos(a)
	bx, by, ok2 := m.h.Pos(b)
	return ok && ok2 && max(abs(ax-bx), abs(ay-by)) <= r
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
