package mind

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// Tantrum (CBT-4). wiki: Mental break, a major break (the 20 % threshold,
// 0.8 days between breaks) lasting 8000 to 12000 ticks: the colonist takes
// its anger out on the colony's things, punching at them at random and
// maybe breaking them. Here it picks one building or item stack at random
// within TantrumRadius, walks up to it and punches it on its fist's cycle
// until it breaks, then picks another; with nothing in reach it wanders
// and looks again after RetargetTicks. It never switches mark while the
// old one stands (TestTantrumDoesNotFlap). The radius is original tuning.

// TantrumRadius is how far a tantrum looks for something to smash.
const TantrumRadius = 8

// tantrum runs one tick of id's tantrum (its job j).
func (m *Mind) tantrum(id entity.EntityID, j job.Record, now uint64) {
	h := m.h
	t := m.track[id]
	if t.Target != 0 {
		if b, ok := h.Breakable(t.X, t.Y); !ok || b != t.Target {
			t.Target, t.Next = 0, now // broken (or gone): look for the next
		}
	}
	if t.Target == 0 && now >= t.Next {
		if t.Target, t.X, t.Y = m.mark(id); t.Target == 0 {
			t.Next = now + RetargetTicks // nothing to smash near: look again later
		}
	}
	m.track[id] = t
	if t.Target == 0 {
		m.wander(id, j)
		return
	}
	x, y, _ := h.Pos(id)
	if max(abs(x-t.X), abs(y-t.Y)) > 1 {
		if j.Active() && j.Kind == job.Move && (now+uint64(id))%repathTicks != 0 {
			return
		}
		if !h.GoBeside(id, t.X, t.Y) {
			t.Target, t.Next = 0, now+RetargetTicks // out of reach: something else later
			m.track[id] = t
		}
		return
	}
	if now < t.Next {
		return // between blows
	}
	if j.Active() {
		h.SetJob(id, job.Record{})
	}
	h.Smash(id, t.Target)
	t.Next = now + uint64(max(h.PunchTicks(id), 1))
	m.track[id] = t
}

// mark draws something to smash within TantrumRadius of id: every
// breakable building or stack in the square, picked at random (0: none).
func (m *Mind) mark(id entity.EntityID) (entity.EntityID, int, int) {
	h := m.h
	x, y, ok := h.Pos(id)
	if !ok {
		return 0, 0, 0
	}
	type spot struct {
		id   entity.EntityID
		x, y int
	}
	var marks []spot
	for dy := -TantrumRadius; dy <= TantrumRadius; dy++ {
		for dx := -TantrumRadius; dx <= TantrumRadius; dx++ {
			if b, ok := h.Breakable(x+dx, y+dy); ok {
				marks = append(marks, spot{b, x + dx, y + dy})
			}
		}
	}
	if len(marks) == 0 {
		return 0, 0, 0
	}
	s := marks[h.Roll(len(marks))]
	return s.id, s.x, s.y
}
