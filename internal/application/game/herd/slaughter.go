package herd

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// Slaughter (FOOD-3, wiki: Animal husbandry): a handler kills a tame
// animal marked for slaughter (the slaughter mark is the hunt mark on a
// tame animal: State.Designated) in a quick melee cut, no weapon needed:
// it walks beside the animal, which holds still, works SlaughterWork and
// kills it. A slaughter is careful: the body yields in full (Careful).
type Slaughter struct{ hd *Herd }

// SlaughterWork is the ticks a slaughter takes (wiki: Animal husbandry,
// 180 ticks).
const SlaughterWork = 180

// Marked reports a tame animal marked for slaughter.
func Marked(a *animal.State) bool { return a.Tame && a.Designated && !a.Dead() }

// Assign sends handler id to slaughter the nearest marked animal no one
// holds.
func (s *Slaughter) Assign(id entity.EntityID) bool {
	h := s.hd.h
	if !h.AnyAnimal(func(_ entity.EntityID, a *animal.State) bool { return Marked(a) }) {
		return false
	}
	aid, ax, ay, ok := h.Nearest(id, func(o entity.EntityID, a *animal.State) bool { return Marked(a) && !claimed(h, o) })
	if !ok {
		return false
	}
	tx, ty, ok := h.BesideSpot(id, ax, ay)
	if !ok {
		return false
	}
	return h.StartJob(id, job.Record{Kind: job.Slaughter, Target: aid, TX: tx, TY: ty, WorkLeft: SlaughterWork})
}

// Do works a slaughter beside the animal, then kills it.
func (s *Slaughter) Do(id entity.EntityID, j job.Record) {
	h := s.hd.h
	a, ok := h.Animal(j.Target)
	switch {
	case !ok || !Marked(&a):
		done(h, id, j)
		return
	case !adjacent(h, id, j.Target):
		if j.Count++; j.Count > chaseTries {
			fail(h, id, j, job.TargetGone)
			return
		}
		walkBeside(h, id, j.Target, j)
		return
	}
	u := h.Work(id)
	if u == 0 {
		return
	}
	if j.WorkLeft -= u; j.WorkLeft > 0 {
		h.SetJob(id, j)
		return
	}
	s.hd.careful = j.Target
	h.Kill(j.Target, id)
	s.hd.careful = 0
	done(h, id, j)
}
