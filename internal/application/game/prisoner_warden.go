package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// Warden work (ENG-003, M8): bring a hungry prisoner a meal, and visit
// each prisoner once every prisoner.VisitInterval (staggered by id) to do
// what its mode says: talk its resistance down (Reduce, Recruit), try to
// recruit it once none is left (Recruit), let it go (Release) or execute
// it (Execute; Release and Execute do not wait for the interval).

const (
	// wardenChatWork and wardenExecuteWork are a visit's work ticks.
	wardenChatWork    = 240
	wardenExecuteWork = 120
	// prisonerFoodRetry is how long a hungry prisoner waits for another
	// try when no warden could bring food.
	prisonerFoodRetry = 1000
	// wardenChase is how often a warden follows a prisoner that moved.
	wardenChase = 4
)

// wardenTask is the visit prisoner p wants now (job.None: nothing).
func (g *Game) wardenTask(p entity.EntityID, st *prisoner.State, now uint64) job.Kind {
	v, _ := g.vital.Get(p)
	if v.Status == health.Dead {
		return job.None
	}
	if r := g.raiders[p]; r.Hungry && now >= r.FoodRetry {
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
		if pj, _ := g.jobs.Get(p); now >= st.NextVisit && g.consciousEnough(p) && !(pj.Kind == job.Sleep && pj.State == job.Do) {
			return job.Warden
		}
	}
	return job.None
}

// assignWarden sends warden id to the nearest prisoner that wants a meal
// or a visit.
func (g *Game) assignWarden(id entity.EntityID) bool {
	if len(g.prisoners) == 0 {
		return false
	}
	now := uint64(g.clock.Now())
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	var kind job.Kind
	bestD := 1 << 30
	for _, p := range g.raiderIDs { // ascending: ties go to the lower id
		st := g.prisoners[p]
		if st == nil || g.care.BeingCaptured(p) {
			continue
		}
		if owner, held := g.reserve.EntityOwner(p); held && owner != id {
			continue
		}
		k := g.wardenTask(p, st, now)
		if k == job.None {
			continue
		}
		pp, _ := g.positions.Get(p)
		if d := manhattan(pos.X, pos.Y, pp.X, pp.Y); d < bestD {
			best, kind, bestD = p, k, d
		}
	}
	if best == 0 {
		return false
	}
	if kind == job.DeliverFood {
		return g.startFoodRun(id, best, now)
	}
	pp, _ := g.positions.Get(best)
	sx, sy, ok := g.besideSpot(id, pp.X, pp.Y)
	if !ok {
		return false
	}
	work := wardenChatWork
	if g.prisoners[best].Mode == prisoner.Execute {
		work = wardenExecuteWork
	}
	return g.startJob(id, job.Record{Kind: job.Warden, Target: best, TX: sx, TY: sy, WorkLeft: work})
}

// startFoodRun sends warden id for the best free food to bring prisoner
// p; with none to be had p waits prisonerFoodRetry.
func (g *Game) startFoodRun(id, p entity.EntityID, now uint64) bool {
	food, fx, fy := g.bestFood(id)
	if food != 0 && g.startJob(id, job.Record{Kind: job.DeliverFood, Target: food, TX: fx, TY: fy, Dest: p}) {
		g.reserve.TryEntity(id, p)
		return true
	}
	r := g.raiders[p]
	r.FoodRetry = now + prisonerFoodRetry
	g.raiders[p] = r
	return false
}

// doDeliverFood: at the food pick up a meal's worth and walk to the
// prisoner; beside it, feed it.
func (g *Game) doDeliverFood(id entity.EntityID, j job.Record) {
	p := j.Dest
	if v, ok := g.vital.Get(p); !ok || v.Status == health.Dead || !g.isPrisoner(p) {
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	if j.Carry == 0 {
		st, ok := g.stacks.Get(j.Target)
		if !ok || st.Carrier != 0 || st.Qty <= 0 {
			j.State, j.Fail = job.Fail, job.TargetGone
			g.jobs.Set(id, j)
			return
		}
		n, _ := g.need.Get(p)
		take := min(st.Qty, portionsFor(n.Hunger, st.Kind))
		carry := j.Target
		if st.Qty > take {
			st.Qty -= take
			g.stacks.Set(j.Target, st)
			carry = g.entities.Next()
			g.itemIDs = append(g.itemIDs, carry)
			cs := st
			cs.Qty, cs.Carrier = take, id
			g.stacks.Set(carry, cs)
		} else {
			st.Carrier = id
			g.stacks.Set(j.Target, st)
		}
		j.Carry, j.Target = carry, carry
		if !g.walkBeside(id, p, &j) {
			return
		}
	}
	if !g.adjacent(id, p) {
		if j.Count++; j.Count > wardenChase || !g.walkBeside(id, p, &j) {
			if j.State != job.Fail {
				j.State, j.Fail = job.Fail, job.TargetGone
				g.jobs.Set(id, j)
			}
		}
		return
	}
	g.feedPrisoner(p, j.Carry)
	j.Carry = 0
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// walkBeside paths id to a tile beside p for job j (saved); it reports
// that id is already there (the job goes on this tick).
func (g *Game) walkBeside(id, p entity.EntityID, j *job.Record) bool {
	pp, _ := g.positions.Get(p)
	sx, sy, ok := g.besideSpot(id, pp.X, pp.Y)
	var path []pathing.Point
	if ok {
		path, ok = g.pathTo(id, sx, sy)
	}
	if !ok {
		j.State, j.Fail = job.Fail, job.Unreachable
		g.jobs.Set(id, *j)
		return false
	}
	j.TX, j.TY, j.Path, j.PathI = sx, sy, path, 0
	j.State = job.Goto
	if len(path) == 0 {
		j.State = job.Do
	}
	g.jobs.Set(id, *j)
	return j.State == job.Do
}

// feedPrisoner: p eats the carried stack carry (portions until full,
// leftovers dropped where it is).
func (g *Game) feedPrisoner(p, carry entity.EntityID) {
	st, ok := g.stacks.Get(carry)
	if !ok {
		return
	}
	n, _ := g.need.Get(p)
	nut := st.Kind.Nutrition()
	for st.Qty > 0 && n.Hunger+nut/2 <= needs.Full {
		st.Qty--
		n.Hunger += nut
	}
	n.Hunger = min(n.Hunger, needs.Full)
	g.need.Set(p, n)
	if st.Qty <= 0 {
		g.deleteItem(carry)
	} else {
		pp, _ := g.positions.Get(p)
		st.Carrier, st.X, st.Y = 0, pp.X, pp.Y
		g.mergeOrSet(carry, st)
	}
	g.lastMeal[p] = st.Kind
	g.ateAt[p] = true
	g.logEvent(p, logEntry{Kind: logAte, Name: item.Kind(st.Kind).String()})
	g.prisonerFoodBand(p, n)
}

// doWarden is a warden's visit, beside the prisoner.
func (g *Game) doWarden(id entity.EntityID, j job.Record) {
	p := j.Target
	st := g.prisoners[p]
	if v, ok := g.vital.Get(p); st == nil || !ok || v.Status == health.Dead {
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	if !g.adjacent(id, p) {
		if j.Count++; j.Count > wardenChase || !g.walkBeside(id, p, &j) {
			if j.State != job.Fail {
				j.State, j.Fail = job.Fail, job.TargetGone
				g.jobs.Set(id, j)
			}
		}
		return
	}
	if st.Mode == prisoner.Execute {
		j.WorkLeft--
	} else {
		j.WorkLeft -= g.workProgress(id, skSocial, true) // skills.go: Social trains
	}
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	now := uint64(g.clock.Now())
	r := g.raiders[p]
	r.Warden = id
	g.raiders[p] = r
	switch st.Mode {
	case prisoner.Recruit, prisoner.Reduce:
		g.wardenChat(id, p, st, now)
	case prisoner.Release:
		g.releasePrisoner(p, id)
	case prisoner.Execute:
		g.executePrisoner(p, id)
	}
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// negotiation is warden id's negotiation ability, permille: its Social
// impact (Social skill and Talking).
func (g *Game) negotiation(id entity.EntityID) int32 {
	return social.SocialImpact(int(g.skillOf(id, skSocial).Level), g.capacity(id, body.Talking))
}

// wardenChat is one Reduce or Recruit visit: resistance falls by the
// warden's negotiation times the prisoner's opinion and mood factor; a
// Recruit visit to a prisoner with none left rolls the recruit chance
// instead. It counts as a chat for both.
func (g *Game) wardenChat(w, p entity.EntityID, st *prisoner.State, now uint64) {
	g.ensurePerson(w)
	g.ensurePerson(p)
	st.NextVisit = now + prisoner.VisitInterval
	switch {
	case st.Resistance > 0:
		f := prisoner.ResistanceFactor(g.opinion(p, w), g.prisonerMood(p))
		st.Resistance = max(0, st.Resistance-prisoner.Reduction(g.negotiation(w), f))
	case st.Mode == prisoner.Recruit &&
		int32(g.stream(streamPrison).IntN(1000)) < prisoner.RecruitChance(int(g.skillOf(w, skSocial).Level)):
		g.recruitPrisoner(p, w)
	}
	g.applyInteraction(w, p, social.Chitchat, now) // social.go: memories, log
}

// recruitPrisoner: prisoner p joins the colony, won over by warden w. It
// becomes a colonist with its own bio and every relation it had.
func (g *Game) recruitPrisoner(p, w entity.EntityID) {
	now := uint64(g.clock.Now())
	delete(g.prisoners, p)
	g.interrupt(p, job.Cancelled)
	g.releaseBed(p)
	delete(g.raiders, p)
	g.raiderIDs = withoutID(g.raiderIDs, p)
	g.pawnIDs = withID(g.pawnIDs, p)
	if per := g.personOf(p); per != nil {
		if _, has := g.sk.bios[p]; !has && per.HasBio {
			g.giveBio(p, per.Bio) // skills.go: its skills and stats
		}
		per.Bio, per.HasBio = g.sk.bios[p], false // the bio is the colonist's now
		per.Where, per.Faction = whereColony, 0
	}
	if _, ok := g.prio[p]; !ok {
		pr := work.DefaultPriorities()
		inc := g.incapableWork(p)
		for t := work.Type(0); t < work.NumTypes; t++ {
			if inc.Enabled(t) {
				pr[t] = 0
			}
		}
		g.prio[p] = pr
	}
	g.labor.Set(p, g.prio[p].Mask())
	if _, ok := g.scheds[p]; !ok {
		g.scheds[p] = g.sched
	}
	g.refreshSig(p)
	g.addOpinionMemory(p, w, social.MemRecruitedMe, now)
	pos, _ := g.positions.Get(p)
	text := g.personName(p) + " has joined the colony, won over by " + g.personName(w) + "."
	g.post(MessageView{Tick: now, Text: text, X: pos.X, Y: pos.Y, Focus: p, Letter: true, Title: "Recruited", Kind: letterGood})
	g.addHistory("recruit", text, p, w)
}

// executePrisoner: warden w kills prisoner p. Its relatives hold it
// against w (socialOnDeath).
func (g *Game) executePrisoner(p, w entity.EntityID) {
	name := g.personName(p)
	pos, _ := g.positions.Get(p)
	g.dieBy(p, health.CauseExecution, w) // combat.go
	delete(g.prisoners, p)
	g.message(pos.X, pos.Y, "%s executed %s.", g.personName(w), name)
	g.addHistory("execution", name+" was executed by "+g.personName(w)+".", p, w)
}
