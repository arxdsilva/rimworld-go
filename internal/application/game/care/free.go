package care

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// Freeing a quest prisoner (QST-067): "Free <name>" on a guest an enemy
// camp holds in its cell. The doer walks beside the captive and, after
// FreeWork ticks, frees it; the game then joins it to the colony and tells
// the quest (Captives.Freed). Only a guest the quest holds can be freed
// (Captives.Held): a colonist's own prisoner is released by the prison.
//
// The job is a job.Strip record marked with Count freeMark, so the game's
// job switch (tick.go) already hands it to Care.Do and no job kind is added;
// Care.Do tells it from a strip by the mark.
const (
	// FreeWork is the ticks freeing takes (original tuning).
	FreeWork = 60
	// freeMark is the Count that marks a Strip record as a free order.
	freeMark = 0x46524545
)

// Captives is what freeing a quest prisoner needs from the simulation.
type Captives interface {
	// Held: id is a quest prisoner still held in its cell.
	Held(id entity.EntityID) bool
	// Freed: colonist by freed id. The host joins it to the colony and
	// signals the quest; Held(id) is false afterwards.
	Freed(by, id entity.EntityID)
}

// Freeing is the free-prisoner component (Care.Free).
type Freeing struct {
	c *Care
	r Captives
}

func newFreeing(c *Care, r Captives) *Freeing { return &Freeing{c: c, r: r} }

// Do runs free job j of doer id: beside the captive, work FreeWork ticks,
// then free it, once. A captive freed, gone or not held ends the job.
func (f *Freeing) Do(id entity.EntityID, j job.Record) {
	c := f.c
	t := j.Target
	if f.r == nil || !f.r.Held(t) || !c.adjacent(id, t) {
		j.State = job.Complete
		c.h.SetJob(id, j)
		return
	}
	if j.WorkLeft--; j.WorkLeft <= 0 {
		f.r.Freed(id, t)
		j.State = job.Complete
	}
	c.h.SetJob(id, j)
}

// Judge is the menu verdict: the option shows for a held captive; the doer's
// reason comes first, then the target's hold, then the path (a hostile-owned
// door or a wall that nobody has opened is "unreachable").
func (f *Freeing) Judge(doer, target entity.EntityID) (bool, string) {
	c := f.c
	if f.r == nil || !f.r.Held(target) {
		return false, ReasonNoLonger
	}
	return true, firstWhy(c.doerWhy(doer, false), c.holdWhy(doer, target), c.reachWhy(doer, target))
}

// Force starts the order.
func (f *Freeing) Force(doer, target entity.EntityID) string {
	return f.c.forceJob(doer, target, job.Record{Kind: job.Strip, Count: freeMark, Target: target, WorkLeft: FreeWork, Forced: true})
}
