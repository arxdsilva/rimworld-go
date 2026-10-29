package mind

import (
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Thresholds are id's break thresholds: the standard ones shifted by its
// traits (wiki: Mental_break, ENG-003 extended).
func (m *Mind) Thresholds(id entity.EntityID) mentalbreak.Thresholds {
	return mentalbreak.ShiftThresholds(mentalbreak.DefaultThresholds(), m.h.BreakShift(id))
}

// maybeBreak is the hourly check: below a threshold a colonist breaks
// with a mean time between breaks of its tier (MTBTicks), unless it still
// feels the catharsis of its last one (the calm window). The break is
// drawn from the tier, among the kinds that make sense where it is.
func (m *Mind) maybeBreak(id entity.EntityID) bool {
	h := m.h
	if until, ok := m.calm[id]; ok {
		if h.Now() < until {
			return false
		}
		delete(m.calm, id)
	}
	tier := mentalbreak.TierAt(h.Mood(id), m.Thresholds(id))
	mtb := mentalbreak.MTBTicks(tier)
	if mtb == 0 || uint64(h.Roll(int(mtb))) >= clock.TicksPerHour {
		return false
	}
	k := mentalbreak.Pick(tier, m.roll(), func(k mentalbreak.Kind) bool { return m.allowed(id, k) })
	if k == mentalbreak.None {
		return false
	}
	m.start(id, k, mentalbreak.DurationFor(k, m.roll()))
	return true
}

// allowed reports break k makes sense for id now.
func (m *Mind) allowed(id entity.EntityID, k mentalbreak.Kind) bool {
	h := m.h
	switch k {
	case mentalbreak.SadWander, mentalbreak.Catatonic:
		return true
	case mentalbreak.FoodBinge:
		return h.CanBinge(id)
	case mentalbreak.HideInRoom:
		return h.CanHide(id)
	case mentalbreak.InsultingSpree:
		return len(m.others(id, false)) >= 2 // wiki: 2+ other colonists
	case mentalbreak.TargetedInsult:
		return len(m.others(id, false)) >= 1
	case mentalbreak.Berserk:
		return h.CanViolence(id)
	case mentalbreak.MurderousRage:
		return h.CanViolence(id) && len(m.others(id, true)) > 0
	}
	return false
}

// others are the colonists other than id that are standing (with
// prisoners: alive, colonists and prisoners both), in the host's order.
// The slice is reused: read it before the next call.
func (m *Mind) others(id entity.EntityID, prisoners bool) []entity.EntityID {
	h := m.h
	m.buf = m.buf[:0]
	for _, c := range h.Colonists() {
		if c != id && (prisoners && h.Alive(c) || !prisoners && h.Standing(c)) {
			m.buf = append(m.buf, c)
		}
	}
	if prisoners {
		for _, p := range h.Prisoners() {
			if p != id && h.Alive(p) {
				m.buf = append(m.buf, p)
			}
		}
	}
	return m.buf
}

// pick draws one of ids (0 when empty).
func (m *Mind) pick(ids []entity.EntityID) entity.EntityID {
	if len(ids) == 0 {
		return 0
	}
	return ids[m.h.Roll(len(ids))]
}

// start puts id into break k for dur ticks: it drops what it was doing,
// a targeted break picks its target, a catatonic pawn goes down.
func (m *Mind) start(id entity.EntityID, k mentalbreak.Kind, dur uint64) {
	h := m.h
	var target entity.EntityID
	switch k {
	case mentalbreak.TargetedInsult:
		target = m.pick(m.others(id, false))
	case mentalbreak.MurderousRage:
		target = m.pick(m.others(id, true))
	}
	h.Seize(id)
	h.SetBreak(id, k, h.Now()+dur)
	m.forget(id)
	if target != 0 {
		m.track[id] = Track{Target: target}
	}
	if k == mentalbreak.Catatonic {
		h.Down(id)
	}
	h.Started(id, k, target)
}
