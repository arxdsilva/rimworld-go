package orders

import (
	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// Strip (AX-1, wiki: Orders): the Strip order marks downed people who
// wear or hold something; a hauler walks beside each and strips it with
// the care package's strip job (care/strip.go: StripWork ticks, then the
// apparel and weapon drop where it lies, a guest's faction minding).
// Corpses need no mark here: the dead shed what they wore when they die
// (PRD-2, combat.go). A mark comes off once its pawn has nothing left to
// strip, gets up or is gone.
type Strip struct {
	h     Host
	marks []entity.EntityID // ascending (saved through Orders.Export)
}

// Mark marks (on) or unmarks every strippable person in the rectangle.
func (s *Strip) Mark(minX, minY, maxX, maxY int, on bool) {
	for _, id := range s.h.PeopleIn(minX, minY, maxX, maxY) {
		if !on || s.h.Strippable(id) {
			s.marks = setMark(s.marks, id, on)
		}
	}
}

// Marked reports person id marked to strip and still strippable (the
// pawn pane's line and the map marker).
func (s *Strip) Marked(id entity.EntityID) bool { return marked(s.marks, id) && s.h.Strippable(id) }

// Assign starts hauler id on the nearest marked person it may take.
func (s *Strip) Assign(id entity.EntityID) bool {
	if len(s.marks) == 0 {
		return false
	}
	h := s.h
	px, py := h.Pos(id)
	var best entity.EntityID
	bestD := 1 << 30
	keep := s.marks[:0]
	for _, t := range s.marks {
		if !h.Strippable(t) {
			continue // stripped, up again or gone: the mark is done
		}
		keep = append(keep, t)
		if t == id || !h.Free(id, t, job.Strip) {
			continue
		}
		tx, ty := h.Pos(t)
		if d := dist(px, py, tx, ty); d < bestD {
			best, bestD = t, d
		}
	}
	s.marks = keep
	if best == 0 {
		return false
	}
	tx, ty := h.Pos(best)
	sx, sy, ok := h.Spot(id, tx, ty)
	if !ok {
		return false
	}
	return h.StartJob(id, job.Record{Kind: job.Strip, Target: best, TX: sx, TY: sy, WorkLeft: care.StripWork})
}
