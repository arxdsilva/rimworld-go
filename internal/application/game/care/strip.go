package care

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// Stripping (SIM-065): "Strip <name>" on a downed pawn or a prisoner. The
// doer walks beside the target; after StripWork ticks the target's
// apparel and weapon drop as item stacks where it lies (Gear.StripGear).
// Any kind of pawn may be stripped when downed, the colony's own
// included, as in the original game. Stripping a downed guest of a
// faction costs StripGoodwill with it (wiki; the wiki's two numbers
// conflict and the decision is -40).
const (
	// StripWork is the ticks a strip takes (original tuning).
	StripWork = 90
	// StripGoodwill is the goodwill a faction loses when its downed guest
	// is stripped.
	StripGoodwill = -40
	// ReasonNothingToStrip disables the option on a pawn with nothing on.
	ReasonNothingToStrip = "nothing to strip"
)

// Stripping is the strip component (SIM-065, Care.Strip).
type Stripping struct {
	c *Care
	g Gear
}

func newStripping(c *Care, g Gear) *Stripping { return &Stripping{c: c, g: g} }

// strippable: target is a person who lies downed or is a prisoner.
func (s *Stripping) strippable(target entity.EntityID) bool {
	c := s.c
	v, ok := c.h.Vital(target)
	if !ok || s.g == nil || c.kindOf(target) == targetNone {
		return false
	}
	return v.Status == health.Downed || v.Status == health.Alive && c.h.IsPrisoner(target)
}

// Do runs strip job j of doer id: beside the target, work for StripWork
// ticks, then drop its gear.
func (s *Stripping) Do(id entity.EntityID, j job.Record) {
	c := s.c
	h := c.h
	t := j.Target
	if !s.strippable(t) || !c.adjacent(id, t) {
		j.State = job.Complete
		h.SetJob(id, j)
		return
	}
	if j.WorkLeft--; j.WorkLeft <= 0 {
		x, y, _ := h.Pos(t)
		v, _ := h.Vital(t)
		if !s.g.StripGear(t) {
			h.Message(x, y, c.personName(t)+" has nothing to strip.")
		} else if v.Status == health.Downed && c.gu != nil && c.gu.IsGuest(t) && c.gu.FactionOf(t) != 0 {
			c.gu.Goodwill(c.gu.FactionOf(t), StripGoodwill, "stripped a downed guest")
		}
		j.State = job.Complete
	}
	h.SetJob(id, j)
}

// Judge: the doer's reason comes first,
// then the target's hold, then "nothing to strip", then the path.
// Judge is the menu verdict.
func (s *Stripping) Judge(doer, target entity.EntityID) (bool, string) {
	c := s.c
	if s.g == nil {
		return false, ReasonIncapable // no Gear role wired
	}
	if !s.strippable(target) {
		return false, ReasonNoLonger
	}
	nothing := ""
	if s.g != nil && !s.g.HasGear(target) {
		nothing = ReasonNothingToStrip
	}
	return true, firstWhy(c.doerWhy(doer, false), c.holdWhy(doer, target), nothing, c.reachWhy(doer, target))
}

// Force starts the order.
func (s *Stripping) Force(doer, target entity.EntityID) string {
	return s.c.forceJob(doer, target, job.Record{Kind: job.Strip, Target: target, WorkLeft: StripWork, Forced: true})
}
