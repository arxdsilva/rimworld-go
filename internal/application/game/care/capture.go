package care

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// Capture and arrest (ENG-003, M8). A captor walks to a downed raider (or
// visitor), picks it up and carries it, the rescuer's way (carryToBed),
// to a free prisoner bed; laid in it, the pawn becomes a prisoner. An
// arrester walks to a colonist or visitor and rolls (Prison.ArrestRoll):
// on success the target is a prisoner on the spot and is carried off the
// same way; on failure it shakes free and fights back.

// arrestChase is how often an arrester re-paths after a target that
// walked off before it gives up.
const arrestChase = 6

// Capturable reports target is a downed pawn the colony may take
// prisoner: a raider or a visitor, not yet a prisoner.
func (c *Care) Capturable(target entity.EntityID) bool {
	v, ok := c.h.Vital(target)
	if !ok || v.Status != health.Downed || c.h.IsPrisoner(target) {
		return false
	}
	return c.pr.IsRaider(target) || c.pr.IsVisitor(target)
}

// Arrestable reports target is a living colonist (on the roster) or
// visitor that can be arrested.
func (c *Care) Arrestable(target entity.EntityID) bool {
	v, ok := c.h.Vital(target)
	if !ok || v.Status == health.Dead || c.h.IsPrisoner(target) {
		return false
	}
	return c.pr.IsVisitor(target) || slices.Contains(c.h.PawnIDs(), target)
}

// BeingCaptured reports a captor or arrester holds target (it does not
// bleed out, walk or work meanwhile).
func (c *Care) BeingCaptured(target entity.EntityID) bool {
	owner, ok := c.h.Owner(target)
	if !ok {
		return false
	}
	j, _ := c.h.Job(owner)
	return (j.Kind == job.Capture || j.Kind == job.Arrest) && j.Target == target && j.Active()
}

// StartCapture sends by (0: the nearest able colonist) to take the downed
// target prisoner; forced marks the player's order. It needs a free
// prisoner bed. It returns why it could not ("" when the captor set off).
func (c *Care) StartCapture(by, target entity.EntityID, forced bool) string {
	if !c.Capturable(target) {
		return ReasonIncapable
	}
	return c.startHold(by, target, job.Capture, forced)
}

// StartArrest sends by (0: the nearest able colonist) to arrest the
// colonist or visitor target; forced marks the player's order. It needs a
// free prisoner bed. It returns why it could not.
func (c *Care) StartArrest(by, target entity.EntityID, forced bool) string {
	if !c.Arrestable(target) {
		return ReasonIncapable
	}
	return c.startHold(by, target, job.Arrest, forced)
}

// startHold starts a capture or arrest (kind) of target once nobody else
// holds it, a handler is able and a prisoner bed is free.
func (c *Care) startHold(by, target entity.EntityID, kind job.Kind, forced bool) string {
	h := c.h
	if c.BeingCaptured(target) {
		owner, _ := h.Owner(target)
		return ReasonBusyTarget + c.pr.PersonName(owner)
	}
	if by == target {
		return ReasonSelf
	}
	by, ok := c.handler(by, target)
	if !ok {
		return ReasonIncapable
	}
	tx, ty, _ := h.Pos(target)
	if _, ok := c.pr.PrisonBedFor(target, tx, ty); !ok {
		h.Message(tx, ty, "No free prisoner bed for "+c.pr.PersonName(target)+".")
		return ReasonNoPrisonBed
	}
	sx, sy, ok := h.BesideSpot(by, tx, ty)
	if !ok {
		return ReasonUnreachable
	}
	h.Interrupt(by, job.Interrupted)
	if !h.StartJob(by, job.Record{Kind: kind, Target: target, TX: sx, TY: sy, Forced: forced}) {
		return ReasonIncapable
	}
	return ""
}

// handler is who carries out a capture or arrest of target: by when it is
// a standing colonist able to, else (by 0) the nearest such colonist (the
// lowest id on a tie).
func (c *Care) handler(by, target entity.EntityID) (entity.EntityID, bool) {
	if by != 0 {
		return by, c.ableHandler(by, target)
	}
	tx, ty, _ := c.h.Pos(target)
	best, bestD := entity.EntityID(0), 1<<30
	for _, id := range c.h.PawnIDs() {
		if !c.ableHandler(id, target) {
			continue
		}
		x, y, _ := c.h.Pos(id)
		if d := manhattan(x, y, tx, ty); d < bestD {
			best, bestD = id, d
		}
	}
	return best, best != 0
}

// ableHandler: id is a colonist on its feet, not in a mental break, and
// not target itself.
func (c *Care) ableHandler(id, target entity.EntityID) bool {
	return slices.Contains(c.h.PawnIDs(), id) && id != target && c.h.Standing(id) && !c.h.Breaking(id)
}

// adjacent reports a and b stand within one tile of each other.
func (c *Care) adjacent(a, b entity.EntityID) bool {
	ax, ay, _ := c.h.Pos(a)
	bx, by, _ := c.h.Pos(b)
	return abs(ax-bx) <= 1 && abs(ay-by) <= 1
}

// DoCapture runs capture job j of captor id: beside the captive pick it
// up and head for a prisoner bed; at the bed lay it down, a prisoner.
func (c *Care) DoCapture(id entity.EntityID, j job.Record) {
	t := j.Target
	if v, ok := c.h.Vital(t); !ok || v.Status == health.Dead || !c.Capturable(t) && !c.h.IsPrisoner(t) {
		c.DropPatient(id)
		j.State = job.Complete
		c.h.SetJob(id, j)
		return
	}
	if j.Dest == 0 && c.Carry.Patient(id) != t && !c.adjacent(id, t) {
		j.State, j.Fail = job.Fail, job.TargetGone
		c.h.SetJob(id, j)
		return
	}
	c.carryToBed(id, j, tripCapture)
}

// DoArrest runs arrest job j of arrester id: beside the target roll the
// arrest; on success carry the new prisoner off (the job becomes a
// capture), else the target shakes free and fights back.
func (c *Care) DoArrest(id entity.EntityID, j job.Record) {
	h := c.h
	t := j.Target
	if !c.Arrestable(t) {
		j.State = job.Complete
		h.SetJob(id, j)
		return
	}
	if !c.adjacent(id, t) {
		c.chase(id, j)
		return
	}
	x, y, _ := h.Pos(id)
	if !c.pr.ArrestRoll(id, t) {
		h.Message(x, y, c.pr.PersonName(t)+" resisted arrest by "+c.pr.PersonName(id)+".")
		j.State, j.Fail = job.Fail, job.Interrupted
		h.SetJob(id, j)
		c.pr.StartFight(t, id) // the target fights back
		return
	}
	h.Interrupt(t, job.Interrupted)
	c.pr.MakePrisoner(t, id, true)
	c.DoCapture(id, job.Record{Kind: job.Capture, Target: t, State: job.Do, Forced: j.Forced})
}

// chase re-paths arrester id beside its target, which walked off; after
// arrestChase tries (or with no way there) the arrest fails.
func (c *Care) chase(id entity.EntityID, j job.Record) {
	h := c.h
	tx, ty, _ := h.Pos(j.Target)
	sx, sy, ok := h.BesideSpot(id, tx, ty)
	var path []pathing.Point
	if ok {
		path, ok = h.PathTo(id, sx, sy)
	}
	if j.Count++; !ok || j.Count > arrestChase {
		j.State, j.Fail = job.Fail, job.TargetGone
		h.SetJob(id, j)
		return
	}
	j.TX, j.TY, j.Path, j.PathI, j.State = sx, sy, path, 0, job.Goto
	if len(path) == 0 {
		j.State = job.Do
	}
	h.SetJob(id, j)
}
