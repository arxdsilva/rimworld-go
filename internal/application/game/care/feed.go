package care

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// Feeding patients (SIM-064, ENG-003 extended: wiki numbers).
//
// A pawn that cannot feed itself, lying downed (from its wounds, a
// collapse, starvation or a breakdown), is fed by a doctor once its food
// falls to Hungry (FeedHungry, the wiki's 25%, needs.CriticalHunger):
// the doctor fetches the best food the patient's policy allows (or a
// paste meal from a dispenser, FOOD-4), carries enough for the
// patient's hunger and feeds it beside the patient, which eats until
// nearly full. The band is wide: feeding starts at 25% and one meal
// lifts the patient to about 100%, so it is not fed again before it has
// grown hungry again. The patient is held (reserved) by its feeder from
// the start, so no second doctor races it; a run that finds no food
// leaves the patient alone for FeedRetry. Prisoners are the warden's
// (the warden's food run, prison/warden.go, uses the same driver: Do).
//
// Standing bed-rest patients get up to eat (Rest.Slot), so only the
// downed count as unable to feed themselves; colonists, and guests the
// colony takes in, are fed this way.

const (
	// FeedHungry: a patient who cannot feed itself is fed at or below
	// this food level (wiki: Hungry, 0.25; no published trigger above it).
	FeedHungry = needs.CriticalHunger
	// FeedOffer: "Feed <name>" is offered below this food level (the
	// prisoner food band's fed line, 80%): above it a meal is mostly left
	// over, so the menu hides the order.
	FeedOffer = needs.Full * 8 / 10
	// FeedRetry: a patient no food could be found for waits this long
	// before a doctor looks again (the warden's prisonerFoodRetry).
	FeedRetry = 1000
	// feedChase is how many times a feeder follows a patient that moved.
	feedChase = 4
	// ReasonNoFood: there is no food the feeder may fetch.
	ReasonNoFood = "no food"
	// ReasonNotHungry: the patient has enough food (the menu hides it).
	ReasonNotHungry = "not hungry"
)

// Meals is what feeding a patient needs from the simulation (SIM-064).
type Meals interface {
	// Hunger is id's food need, 0..needs.Full (needs.Full without one).
	Hunger(id entity.EntityID) int
	// BestFood is the best food feeder id could fetch for patient and
	// its tile (0 when none): a stack by the patient's food policy and
	// taste, or a paste dispenser whose meal is better (FOOD-4).
	BestFood(id, patient entity.EntityID) (food entity.EntityID, x, y int)
	// TakeFood: id, at stack food, picks up enough of it for patient's
	// hunger (split off when bigger); false when the stack is gone or
	// held.
	TakeFood(id, food, patient entity.EntityID) (carry entity.EntityID, ok bool)
	// FedBy logs that feeder fed patient.
	FedBy(patient, feeder entity.EntityID)
}

// Feeding is the feeding driver and the Doctor's feed giver; retry is
// when each patient no food was found for may be looked at again.
type Feeding struct {
	c     *Care
	h     Host
	m     Meals
	retry map[entity.EntityID]uint64
}

// newFeeding builds care c's feeding on host h and meals m (nil: nobody
// is fed, as in the unit tests of other components).
func newFeeding(c *Care, h Host, m Meals) *Feeding {
	return &Feeding{c: c, h: h, m: m}
}

// Export is the FeedRetry save field, in id order (nil: nothing to keep).
func (f *Feeding) Export() []Entry[uint64] { return exportEntries(f.retry) }

// Import restores Export's state (nil: an older save).
func (f *Feeding) Import(es []Entry[uint64]) {
	f.retry = nil
	if len(es) > 0 {
		f.retry = importEntries(es)
	}
}

// RetryAt is when patient id, no food found, may be fed again (0: now).
func (f *Feeding) RetryAt(id entity.EntityID) uint64 { return f.retry[id] }

// Helpless: id cannot feed itself: a colonist, or a guest the colony
// takes in, lying downed and not being carried. Prisoners are the
// warden's.
func (f *Feeding) Helpless(id entity.EntityID) bool {
	v, ok := f.h.Vital(id)
	if !ok || v.Status != health.Downed || f.c.Carry.Carrier(id) != 0 {
		return false
	}
	switch f.c.kindOf(id) {
	case targetColonist, targetGuest, targetStranger:
		return true
	}
	return false
}

// due: patient id cannot feed itself, is at or below FeedHungry, and no
// failed run holds it back.
func (f *Feeding) due(id entity.EntityID) bool {
	if !f.Helpless(id) || f.m.Hunger(id) > FeedHungry {
		return false
	}
	if t, ok := f.retry[id]; ok {
		if f.h.Now() < t {
			return false
		}
		delete(f.retry, id)
	}
	return true
}

// failed holds patient id back for FeedRetry: no food could be had.
func (f *Feeding) failed(id entity.EntityID) {
	if f.retry == nil {
		f.retry = make(map[entity.EntityID]uint64)
	}
	f.retry[id] = f.h.Now() + FeedRetry
}

// assign is the Doctor's feed giver: doctor id feeds the nearest patient
// that is due (lowest id on a tie), not itself, not held by anyone else,
// with a spot beside it.
func (f *Feeding) assign(id entity.EntityID) bool {
	if f.m == nil {
		return false
	}
	h := f.h
	x, y, _ := h.Pos(id)
	var best entity.EntityID
	bestD := 1 << 30
	for _, pid := range h.PawnIDs() { // ascending: ties go to the lower id
		if pid == id || !f.due(pid) {
			continue
		}
		if _, taken := h.Owner(pid); taken {
			continue
		}
		px, py, _ := h.Pos(pid)
		if _, _, ok := h.BesideSpot(id, px, py); !ok {
			continue
		}
		if d := manhattan(x, y, px, py); d < bestD {
			best, bestD = pid, d
		}
	}
	if best == 0 {
		return false
	}
	if !f.Start(id, best, job.FeedPatient, false) {
		f.failed(best)
		return false
	}
	return true
}

// Start sends feeder id for the best food it may fetch to bring patient
// p, as a job of kind k (job.FeedPatient for a doctor, job.DeliverFood
// for a warden), and holds p for it; false when there is no food to be
// had or the way to it is shut.
func (f *Feeding) Start(id, p entity.EntityID, k job.Kind, forced bool) bool {
	if f.m == nil {
		return false
	}
	food, fx, fy := f.m.BestFood(id, p)
	if food == 0 || !f.h.StartJob(id, job.Record{Kind: k, Target: food, TX: fx, TY: fy, Dest: p, Forced: forced}) {
		return false
	}
	f.h.Claim(id, p)
	return true
}

// Do runs one tick of feeding job j of feeder id (a doctor's FeedPatient
// or a warden's DeliverFood): at the food pick up a meal's worth and
// walk to the patient; beside it, feed it. A patient that no longer
// wants feeding ends the job (the host drops what the feeder carries).
func (f *Feeding) Do(id entity.EntityID, j job.Record) {
	h := f.h
	p := j.Dest
	if !f.wanted(j.Kind, p) || f.m == nil {
		j.State = job.Complete
		h.SetJob(id, j)
		return
	}
	if j.Carry == 0 {
		carry, ok := f.m.TakeFood(id, j.Target, p)
		if !ok {
			j.State, j.Fail = job.Fail, job.TargetGone
			h.SetJob(id, j)
			return
		}
		j.Carry, j.Target = carry, carry
		var there bool
		if j, there = f.WalkBeside(id, p, j); !there {
			return
		}
	}
	if !f.c.adjacent(id, p) {
		f.Chase(id, p, j, feedChase)
		return
	}
	h.Feed(p, j.Carry)
	f.m.FedBy(p, id)
	j.Carry = 0
	j.State = job.Complete
	h.SetJob(id, j)
}

// wanted: patient p still wants the feeding job of kind k: a living
// prisoner for the warden, a patient that still cannot feed itself for a
// doctor.
func (f *Feeding) wanted(k job.Kind, p entity.EntityID) bool {
	if k == job.DeliverFood {
		v, ok := f.h.Vital(p)
		return ok && v.Status != health.Dead && f.h.IsPrisoner(p)
	}
	return f.Helpless(p)
}

// Chase re-paths id beside p, which is out of reach of job j: after
// tries follows, or with no way there, j fails. The job keeps walking
// otherwise (the warden's visit uses it too).
func (f *Feeding) Chase(id, p entity.EntityID, j job.Record, tries int) {
	if j.Count++; j.Count > tries {
		j.State, j.Fail = job.Fail, job.TargetGone
		f.h.SetJob(id, j)
		return
	}
	f.WalkBeside(id, p, j)
}

// WalkBeside paths id to a tile beside p for job j and stores it; it
// returns the job and whether id is already there (the job goes on this
// tick). With no way there the job fails (Unreachable).
func (f *Feeding) WalkBeside(id, p entity.EntityID, j job.Record) (job.Record, bool) {
	h := f.h
	px, py, _ := h.Pos(p)
	sx, sy, ok := h.BesideSpot(id, px, py)
	var path []pathing.Point
	if ok {
		path, ok = h.PathTo(id, sx, sy)
	}
	if !ok {
		j.State, j.Fail = job.Fail, job.Unreachable
		h.SetJob(id, j)
		return j, false
	}
	j.TX, j.TY, j.Path, j.PathI = sx, sy, path, 0
	j.State = job.Goto
	if len(path) == 0 {
		j.State = job.Do
	}
	h.SetJob(id, j)
	return j, j.State == job.Do
}

// judge is "Feed <name>" (care's judge: the menu and the order share
// it): offered for a patient who cannot feed itself and is below
// FeedOffer. A doer unable to doctor, the target's hold, no food the
// doer may fetch or no way to the patient disables it.
func (f *Feeding) judge(doer, target entity.EntityID) (bool, string) {
	c := f.c
	switch {
	case f.m == nil || !f.Helpless(target):
		return false, ReasonNoLonger
	case f.m.Hunger(target) >= FeedOffer:
		return false, ReasonNotHungry
	}
	return true, firstWhy(c.doerWhy(doer, true), c.holdWhy(doer, target), f.foodWhy(doer, target), c.reachWhy(doer, target))
}

// foodWhy: no food doer may fetch for target.
func (f *Feeding) foodWhy(doer, target entity.EntityID) string {
	if food, _, _ := f.m.BestFood(doer, target); food == 0 {
		return ReasonNoFood
	}
	return ""
}

// force starts the ordered feeding once judge has passed: doer drops its
// job and fetches food for target, a forced job.
func (f *Feeding) force(doer, target entity.EntityID) string {
	f.h.Interrupt(doer, job.Cancelled) // a plain order already replaced the queue (tick.go)
	if f.Start(doer, target, job.FeedPatient, true) {
		return ""
	}
	if j, _ := f.h.Job(doer); j.State == job.Fail {
		return j.Fail.String()
	}
	return ReasonNoFood
}
