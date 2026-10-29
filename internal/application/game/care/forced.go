package care

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// ReasonNoLonger: the order's target no longer fits it (got up, healed,
// was laid in bed, was taken prisoner, left). Force returns it for a
// queued order that came due too late; the menu never shows it, since it
// hides such an option.
const ReasonNoLonger = "no longer possible"

// Force starts pawn-targeted order o (o.TargetsPawn()) for doer on
// target as a forced job, the player's order; it returns why it could
// not ("" when it started). The game calls it for a Prioritize command
// and when a queued order comes due (order_queue.go). It refuses exactly
// when PawnOrders would hide or disable the option (judge).
func (c *Care) Force(doer entity.EntityID, o command.Order, target entity.EntityID) string {
	if _, why := c.judge(doer, o, target); why != "" {
		return why
	}
	switch o {
	case command.OrderRescue:
		return c.forceJob(doer, target, job.Record{Kind: job.Rescue, Target: target, Forced: true})
	case command.OrderTendPatient:
		return c.forceJob(doer, target, job.Record{Kind: job.Tend, Target: target, WorkLeft: TendWork, Forced: true})
	case command.OrderCapture:
		return c.StartCapture(doer, target, true)
	case command.OrderArrest:
		return c.StartArrest(doer, target, true)
	case command.OrderFeedPatient:
		return c.Feed.force(doer, target) // feed.go (SIM-064)
	case command.OrderStrip:
		return c.Strip.Force(doer, target) // strip.go (SIM-065)
	case command.OrderCarryToPrisonBed:
		return c.PrisonBeds.Force(doer, target) // prisonbed.go (SIM-063)
	}
	return ReasonIncapable // judge refuses every other order
}

// forceJob drops doer's job and starts rec (a rescue, or a tend with its
// medicine fetch) from the spot beside target, the way the Doctor's own
// givers build it (assignRescue, assignTendWhere), and reserves target
// for doer so no autonomous doctor races it. The forced job survives a
// draft: the tick runs a drafted pawn's job to the end, and DoRescue and
// DoTend never look at the draft.
func (c *Care) forceJob(doer, target entity.EntityID, rec job.Record) string {
	h := c.h
	px, py, _ := h.Pos(target)
	rec.TX, rec.TY, _ = h.BesideSpot(doer, px, py) // judge checked the spot
	h.Interrupt(doer, job.Cancelled)               // a plain order already replaced the queue (tick.go)
	started := false
	if rec.Kind == job.Tend {
		started = c.Doctor.startTend(doer, rec)
	} else {
		started = h.StartJob(doer, rec)
	}
	if !started {
		if j, _ := h.Job(doer); j.State == job.Fail {
			return j.Fail.String()
		}
		return ReasonIncapable
	}
	h.Claim(doer, target)
	return ""
}

// judge is the one verdict on pawn-targeted order o by doer on target,
// shared by the menu (PawnOrders) and the order (Force), so they never
// disagree. offer false hides the option; why, when set, disables it (and
// always says why when offer is false). The doer's own reason comes
// first, then the target's hold, then the bed, then the path.
func (c *Care) judge(doer entity.EntityID, o command.Order, target entity.EntityID) (offer bool, why string) {
	if doer == target {
		return false, ReasonSelf
	}
	switch o {
	case command.OrderRescue:
		if !c.rescueTarget(target) {
			return false, ReasonNoLonger
		}
		return true, firstWhy(c.doerWhy(doer, true), c.holdWhy(doer, target), c.rescueBedWhy(target), c.reachWhy(doer, target))
	case command.OrderCapture:
		if !c.Capturable(target) || c.BeingCaptured(target) {
			return false, ReasonNoLonger
		}
		return true, firstWhy(c.doerWhy(doer, false), c.holdWhy(doer, target), c.prisonBedWhy(target), c.reachWhy(doer, target))
	case command.OrderArrest:
		if v, _ := c.h.Vital(target); !c.Arrestable(target) || v.Status != health.Alive || c.BeingCaptured(target) {
			return false, ReasonNoLonger
		}
		return true, firstWhy(c.doerWhy(doer, false), c.holdWhy(doer, target), c.prisonBedWhy(target), c.reachWhy(doer, target))
	case command.OrderTendPatient:
		return c.judgeTend(doer, target)
	case command.OrderFeedPatient:
		return c.Feed.judge(doer, target) // feed.go (SIM-064)
	case command.OrderStrip:
		return c.Strip.Judge(doer, target) // strip.go (SIM-065)
	case command.OrderCarryToPrisonBed:
		return c.PrisonBeds.Judge(doer, target) // prisonbed.go (SIM-063)
	}
	return false, ReasonIncapable
}

// judgeTend is "Prioritize tending": an undrafted doer (drafted, it gets
// "Tend <name>") on a colonist or prisoner that needs tending, whose care
// level allows a tend and whom no one carries; then CanEmergencyTend's
// reasons without the draft (tendWhy).
func (c *Care) judgeTend(doer, target entity.EntityID) (bool, string) {
	h := c.h
	switch {
	case !h.NeedsTending(target):
		return false, ReasonNothingToTend
	case c.Carry.Carrier(target) != 0:
		return false, ReasonCarried + c.personName(c.Carry.Carrier(target))
	case h.Drafted(doer) || !c.Level(target).Tends() || c.kindOf(target) == targetNone || c.kindOf(target) == targetHostile:
		return false, ReasonNoLonger
	}
	return true, firstWhy(c.doerWhy(doer, true), c.Doctor.tendWhy(doer, target))
}

// firstWhy is the first non-empty reason.
func firstWhy(whys ...string) string {
	for _, w := range whys {
		if w != "" {
			return w
		}
	}
	return ""
}

// rescueTarget: target is a colonist downed where a rescue helps (the
// Doctor's InjuredDowned), out of bed: Doctor.Rescuable without its "a
// free bed exists", which disables the option instead (rescueBedWhy).
func (c *Care) rescueTarget(target entity.EntityID) bool { return c.Doctor.rescueEligible(target) }

// doerWhy: doer can't act at all (not a standing colonist, or in a
// mental break), or, for a Doctor job (doctor), is incapable of Doctor
// work. Its Doctor work priority never gates a forced order.
func (c *Care) doerWhy(doer entity.EntityID, doctor bool) string {
	h := c.h
	if !h.IsColonist(doer) || !h.Standing(doer) || h.Breaking(doer) || doctor && h.DoctorBlocked(doer) {
		return ReasonIncapable
	}
	return ""
}

// holdWhy: someone carries target, or someone besides doer holds it
// reserved.
func (c *Care) holdWhy(doer, target entity.EntityID) string {
	if cr := c.Carry.Carrier(target); cr != 0 {
		return ReasonCarried + c.personName(cr)
	}
	if owner, taken := c.h.Owner(target); taken && owner != doer {
		return ReasonBusyTarget + c.personName(owner)
	}
	return ""
}

// rescueBedWhy: no safe bed to carry target to.
func (c *Care) rescueBedWhy(target entity.EntityID) string {
	px, py, _ := c.h.Pos(target)
	if c.SafeBedFor(target, px, py, RescueBed) == 0 {
		return ReasonNoBed
	}
	return ""
}

// prisonBedWhy: no free prisoner bed for target.
func (c *Care) prisonBedWhy(target entity.EntityID) string {
	px, py, _ := c.h.Pos(target)
	if _, ok := c.pr.PrisonBedFor(target, px, py); !ok {
		return ReasonNoPrisonBed
	}
	return ""
}

// reachWhy: doer can't get beside target.
func (c *Care) reachWhy(doer, target entity.EntityID) string {
	px, py, _ := c.h.Pos(target)
	if _, _, ok := c.h.BesideSpot(doer, px, py); !ok {
		return ReasonUnreachable
	}
	return ""
}

// tendWhy reports why colonist doctor cannot tend patient now, drafted
// or not ("" when it can): CanEmergencyTend without the draft.
func (d *Doctor) tendWhy(doctor, patient entity.EntityID) string {
	h := d.h
	switch {
	case !h.IsColonist(doctor) || h.DoctorBlocked(doctor):
		return ReasonIncapable
	case !h.NeedsTending(patient):
		return ReasonNothingToTend
	case doctor == patient && !d.c.SelfTendAllowed(doctor):
		return ReasonNoSelfTend
	}
	if owner, taken := h.Owner(patient); taken && owner != doctor {
		return ReasonBusyTarget + h.Name(owner)
	}
	px, py, _ := h.Pos(patient)
	if _, _, ok := h.BesideSpot(doctor, px, py); !ok {
		return ReasonUnreachable
	}
	return ""
}
