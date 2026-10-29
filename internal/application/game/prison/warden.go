package prison

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/sim/entity"
)

// Warden work (ENG-003, M8): bring a hungry prisoner a meal, and visit
// each prisoner once every prisoner.VisitInterval (staggered by id) to do
// what its mode says: talk its resistance down (Reduce, Recruit), try to
// recruit it once none is left (Recruit), let it go (Release) or execute
// it (Execute; Release and Execute do not wait for the interval).

const (
	// chatWork and executeWork are a visit's work ticks.
	chatWork    = 240
	executeWork = 120
	// foodRetry is how long a hungry prisoner waits for another try when
	// no warden could bring food.
	foodRetry = 1000
	// chaseEvery is how often a warden follows a prisoner that moved.
	chaseEvery = 4
)

// task is the visit prisoner q wants now (job.None: nothing).
func (p *Prison) task(q entity.EntityID, st *prisoner.State, now uint64) job.Kind {
	h := p.h
	v, _ := h.Vital(q)
	if v.Status == health.Dead {
		return job.None
	}
	if r := h.Held(q); r.Hungry && now >= r.FoodRetry {
		return job.DeliverFood
	}
	if st.Mode == prisoner.Execute {
		return job.Warden
	}
	switch st.Mode {
	case prisoner.Release:
		if v.Status == health.Alive { // it has to walk out
			return job.Warden
		}
	case prisoner.Recruit, prisoner.Reduce: // in bed is fine, asleep or senseless is not
		if qj, _ := h.Job(q); now >= st.NextVisit && h.ConsciousEnough(q) && !(qj.Kind == job.Sleep && qj.State == job.Do) {
			return job.Warden
		}
	}
	return job.None
}

// Assign sends warden id to the nearest prisoner that wants a meal or a
// visit (a downed prisoner back to bed first).
func (p *Prison) Assign(id entity.EntityID) bool {
	if len(p.st) == 0 {
		return false
	}
	h := p.h
	if h.BedBack(id) {
		return true
	}
	now := h.Now()
	x, y, _ := h.Pos(id)
	var best entity.EntityID
	var kind job.Kind
	bestD := 1 << 30
	for _, q := range h.RaiderIDs() { // ascending: ties go to the lower id
		st := p.st[q]
		if st == nil || h.Captured(q) {
			continue
		}
		if owner, held := h.Owner(q); held && owner != id {
			continue
		}
		k := p.task(q, st, now)
		if k == job.None {
			continue
		}
		qx, qy, _ := h.Pos(q)
		if d := abs(x-qx) + abs(y-qy); d < bestD {
			best, kind, bestD = q, k, d
		}
	}
	if best == 0 {
		return false
	}
	if kind == job.DeliverFood {
		return p.foodRun(id, best, now)
	}
	qx, qy, _ := h.Pos(best)
	sx, sy, ok := h.BesideSpot(id, qx, qy)
	if !ok {
		return false
	}
	work := chatWork
	if p.st[best].Mode == prisoner.Execute {
		work = executeWork
	}
	return h.StartJob(id, job.Record{Kind: job.Warden, Target: best, TX: sx, TY: sy, WorkLeft: work})
}

// foodRun sends warden id for the best free food to bring prisoner q;
// with none to be had q waits foodRetry.
func (p *Prison) foodRun(id, q entity.EntityID, now uint64) bool {
	if p.h.FeedRun(id, q) {
		return true
	}
	r := p.h.Held(q)
	r.FoodRetry = now + foodRetry
	p.h.SetHeld(q, r)
	return false
}

// Do is a warden's visit, beside the prisoner.
func (p *Prison) Do(id entity.EntityID, j job.Record) {
	h, q := p.h, j.Target
	st := p.st[q]
	if v, ok := h.Vital(q); st == nil || !ok || v.Status == health.Dead {
		j.State = job.Complete
		h.SetJob(id, j)
		return
	}
	ix, iy, _ := h.Pos(id)
	if qx, qy, _ := h.Pos(q); abs(ix-qx) > 1 || abs(iy-qy) > 1 {
		h.Chase(id, q, j, chaseEvery)
		return
	}
	if st.Mode == prisoner.Execute {
		j.WorkLeft--
	} else {
		j.WorkLeft -= h.SocialWork(id)
	}
	if j.WorkLeft > 0 {
		h.SetJob(id, j)
		return
	}
	now := h.Now()
	r := h.Held(q)
	r.Warden = id
	h.SetHeld(q, r)
	switch st.Mode {
	case prisoner.Recruit, prisoner.Reduce:
		p.Chat(id, q, now)
	case prisoner.Release:
		p.Release(q, id)
	case prisoner.Execute:
		h.Execute(q, id)
		delete(p.st, q)
	}
	j.State = job.Complete
	h.SetJob(id, j)
}

// Chat is one Reduce or Recruit visit of warden w to prisoner q:
// resistance falls by the warden's negotiation times the prisoner's
// opinion and mood factor; a Recruit visit to a prisoner with none left
// rolls the recruit chance instead. It counts as a chat for both.
func (p *Prison) Chat(w, q entity.EntityID, now uint64) {
	h, st := p.h, p.st[q]
	h.Meet(w, q)
	st.NextVisit = now + prisoner.VisitInterval
	switch {
	case st.Resistance > 0:
		f := prisoner.ResistanceFactor(h.Opinion(q, w), h.Mood(q))
		st.Resistance = max(0, st.Resistance-prisoner.Reduction(h.Negotiation(w), f))
	case st.Mode == prisoner.Recruit && int32(h.Roll(1000)) < prisoner.RecruitChance(h.SocialLevel(w)):
		delete(p.st, q)
		h.Recruit(q, w)
	}
	h.Chat(w, q, now)
}
