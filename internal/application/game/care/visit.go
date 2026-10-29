package care

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/entity"
)

// Visiting the sick (SIM-065): an idle colonist whose recreation has run
// down walks to a friend lying awake in bed (bed rest, downed in bed, a
// tended patient) and keeps it company beside the bed. Both gain
// recreation: the wiki's 0.216 of the bar over 1,500 ticks, 2,160 of
// recreation.Full, spread evenly over the visit.
//
// The numbers other than the wiki's are original tuning:
//   - friend: the visitor's opinion of the patient is at least
//     VisitOpinion and the patient does not dislike the visitor;
//   - start line: the visitor's recreation below recreation.Entertained,
//     and the visit itself lifts it 2,160, so it does not chain visits
//     (the hysteresis);
//   - cooldown: a pair may start a visit only in the first visitStart
//     ticks of every second visitWindow, a schedule fixed by the pair and
//     the tick (nothing to save). A visit lasts VisitTicks > visitStart,
//     so it ends after the start slot closed, and the next slot of that
//     pair opens a whole window later: at least visitWindow-VisitTicks
//     ticks after a visit ended, never back to back.
const (
	// VisitTicks is how long a visit lasts (wiki).
	VisitTicks = 1500
	// VisitGain is the recreation a whole visit restores each of the two
	// (wiki: 0.216 of the bar).
	VisitGain = recreation.Full * 216 / 1000
	// VisitOpinion is the opinion that makes a friend (original tuning).
	VisitOpinion = 20
	visitWindow  = 2400
	visitStart   = 600
	visitReach   = 30 // how far (pathing.Dist) a visitor goes
)

// Visits is the visiting-the-sick component (SIM-065); the lead gives
// Care a field for it, until then Care's forwarders build one per call.
type Visits struct {
	c *Care
	s Social
}

func newVisits(c *Care, s Social) *Visits { return &Visits{c: c, s: s} }

// Assign gives idle colonist id a visit to a friend lying awake in bed
// (the game's free-time choice tries it first); false when there is none,
// so the free-time roll goes on as before.
func (v *Visits) Assign(id entity.EntityID) bool {
	h := v.c.h
	if v.s == nil || !h.Standing(id) || h.Drafted(id) || h.Breaking(id) || v.s.Recreation(id) >= recreation.Entertained {
		return false
	}
	x, y, _ := h.Pos(id)
	best, bestD := entity.EntityID(0), visitReach+1
	for _, p := range h.PawnIDs() {
		px, py, _ := h.Pos(p)
		if d := pathing.Dist(x, y, px, py); d < bestD && v.visitable(id, p) {
			best, bestD = p, d
		}
	}
	if best == 0 {
		return false
	}
	px, py, _ := h.Pos(best)
	sx, sy, ok := h.BesideSpot(id, px, py)
	return ok && h.StartJob(id, job.Record{Kind: job.VisitSick, Count: int(best), TX: sx, TY: sy, WorkLeft: VisitTicks})
}

// visitable: visitor v may start a visit to p now.
func (vs *Visits) visitable(v, p entity.EntityID) bool {
	if p == v || !vs.patientAwake(p) || vs.s.Opinion(v, p) < VisitOpinion || vs.s.Opinion(p, v) < 0 {
		return false
	}
	return visitSlot(vs.c.h.Now(), v, p)
}

// visitSlot: the pair's start slot is open at tick now.
func visitSlot(now uint64, v, p entity.EntityID) bool {
	return now%visitWindow < visitStart && (now/visitWindow+uint64(v)+uint64(p))%2 == 0
}

// patientAwake: p is a colonist lying in bed as a patient (bed rest,
// downed, or with something to tend), awake and conscious.
func (vs *Visits) patientAwake(p entity.EntityID) bool {
	c, h := vs.c, vs.c.h
	v, ok := h.Vital(p)
	if !ok || !h.IsColonist(p) || h.IsPrisoner(p) || h.Breaking(p) || h.BedUnder(p) == 0 || !vs.s.Awake(p) {
		return false
	}
	return v.Status == health.Downed || v.Status == health.Alive && (c.Rest.On(p) || h.NeedsTending(p))
}

// Do runs visit j of colonist id: beside the bed, stay VisitTicks,
// chat once halfway (Interact) and restore recreation each tick; it ends
// early when the patient is no longer lying awake in bed.
func (v *Visits) Do(id entity.EntityID, j job.Record) {
	h := v.c.h
	p := entity.EntityID(j.Count)
	if v.s == nil || !v.patientAwake(p) || !v.c.adjacent(id, p) {
		j.State = job.Complete
		h.SetJob(id, j)
		return
	}
	done := VisitTicks - j.WorkLeft // ticks already spent
	if j.WorkLeft == VisitTicks/2 {
		v.s.Interact(id, p)
	}
	step := VisitGain*(done+1)/VisitTicks - VisitGain*done/VisitTicks
	v.s.Recreate(id, step)
	v.s.Recreate(p, step)
	if j.WorkLeft--; j.WorkLeft <= 0 {
		j.State = job.Complete
	}
	h.SetJob(id, j)
}
