package care

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// Offering help to a downed refugee (QST-055, QST-057): "Offer help to
// <name>" on the person a refugee quest lies on its site. The doer walks
// beside them and, on arriving, tells the host (Captives.Helped), which
// joins them to the colony and succeeds the quest. It is an interaction,
// not a carry, so there is no work time.
//
// Like the free order the job is a job.Strip record, marked with Count
// helpMark, and Care.Do tells the two apart by the mark.
const helpMark = 0x48454c50

// Helping is the offer-help component (Care.Help).
type Helping struct {
	c *Care
	r Captives
}

func newHelping(c *Care, r Captives) *Helping { return &Helping{c: c, r: r} }

// Do runs help job j of doer id: beside the person, help once and end. A
// person already helped, gone or not helpable ends the job.
func (f *Helping) Do(id entity.EntityID, j job.Record) {
	c := f.c
	t := j.Target
	if f.r != nil && f.r.Helpable(t) && c.adjacent(id, t) {
		f.r.Helped(id, t)
	}
	j.State = job.Complete
	c.h.SetJob(id, j)
}

// Judge is the menu verdict: offered for the person a refugee quest holds.
func (f *Helping) Judge(doer, target entity.EntityID) (bool, string) {
	c := f.c
	if f.r == nil || !f.r.Helpable(target) {
		return false, ReasonNoLonger
	}
	return true, firstWhy(c.doerWhy(doer, false), c.holdWhy(doer, target), c.reachWhy(doer, target))
}

// Force starts the order.
func (f *Helping) Force(doer, target entity.EntityID) string {
	if f.r == nil || !f.r.Helpable(target) {
		return ReasonNoLonger
	}
	return f.c.forceJob(doer, target, job.Record{Kind: job.Strip, Count: helpMark, Target: target, Forced: true})
}
