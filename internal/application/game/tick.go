package game

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// ApplyPending applies every queued command now, in order: what the next
// Step would do first. While paused the clock does not move, so this is the
// same state the first phase of that Step would reach (the UI calls it so
// paused actions show at once).
func (g *Game) ApplyPending() {
	g.viewGen++ // view cache (indexes.go): commands may change the state
	g.phaseCommands()
}

func (g *Game) phaseCommands() {
	cmds := g.pending
	g.pending = nil
	for _, c := range cmds {
		g.applyOne(c)
	}
}

func (g *Game) applyOne(c command.Command) {
	if c.Kind == command.MovePawn || c.Kind == command.Prioritize || c.Kind == command.DraftPawn {
		if _, breaking := g.breaks[entity.EntityID(c.Pawn)]; breaking {
			return // a colonist in a mental break won't take orders
		}
	}
	if c.Kind == command.MovePawn || c.Kind == command.Prioritize {
		id := entity.EntityID(c.Pawn)
		if c.Queue {
			g.queueOrder(id, c)
			return
		}
		delete(g.orderQueue, id)        // a plain order replaces the queue
		defer g.followPlainOrder(id, c) // order_queue.go: kept to retry if blocked
	}
	switch c.Kind {
	case command.AddOperation, command.RemoveOperation, command.SetDrugPolicy, command.AssignBedOwner,
		command.SetAnimalMaster, command.SetTraining, command.GiftAnimal, command.Capture, command.Arrest,
		command.SetPrisonerMode, command.SetBedForPrisoners, command.FormCaravan, command.CaravanReturn,
		command.PaintArea, command.RenameArea, command.AssignArea, command.ReleasePrisoner:
		g.applyHostCommand(c) // host_commands.go (ENG-003)
	case command.SetStoryteller:
		g.cmdSetStoryteller(c.Name) // story.go (NG-1)
	case command.SetDifficulty:
		g.cmdSetDifficulty(c.Name) // story.go (NG-1)
	case command.ClearOrders:
		delete(g.orderQueue, entity.EntityID(c.Pawn))
	case command.MovePawn:
		g.cmdMove(entity.EntityID(c.Pawn), c.X, c.Y)
	case command.DesignateStockpile:
		g.paintZone(zone.KindStockpile, zone.Zone{Food: true, Wood: true, Stone: true}, c.X, c.Y, c.X2, c.Y2)
	case command.DesignateWall:
		g.placeFrame(construction.Wall, c.X, c.Y)
	case command.DesignateBed:
		g.placeFrame(construction.Bed, c.X, c.Y)
	case command.DesignateHarvest:
		minX, minY, maxX, maxY := zone.Normalize(c.X, c.Y, c.X2, c.Y2)
		for _, id := range g.plantIDs {
			p, ok := g.plants.Get(id)
			if !ok || !p.Harvestable() {
				continue
			}
			if p.X >= minX && p.X <= maxX && p.Y >= minY && p.Y <= maxY {
				p.Designated = true
				g.plants.Set(id, p)
				g.notePlantMark(id) // jobindex.go
			}
		}
	case command.DesignateCancel:
		g.cancelRect(c.X, c.Y, c.X2, c.Y2)
	case command.DesignateBuild:
		kind, stuff := construction.Kind(c.Build), item.Kind(c.Stuff)
		if !construction.StuffOK(kind, stuff) {
			stuff = construction.DefaultStuff(kind) // zero value or a mismatch: default material
		}
		if !g.buildLocked(kind) {
			g.placeFrameStuff(kind, stuff, construction.ParseFacing(c.Facing), c.X, c.Y)
		}
	case command.DesignateCut:
		g.markCut(c.X, c.Y, c.X2, c.Y2, true)
	case command.DesignateMine:
		g.markMine(c.X, c.Y, c.X2, c.Y2, true)
	case command.DesignateSmooth:
		g.markSmooth(c.X, c.Y, c.X2, c.Y2, true) // smooth.go
	case command.AddBill, command.SetBill, command.RemoveBill:
		g.applyBill(c)
	case command.DesignateGrowZone:
		if !g.cropLocked(plant.Species(c.Build)) {
			g.addGrowZone(c.X, c.Y, c.X2, c.Y2, c.Build)
		}
	case command.SetResearch:
		g.setResearch(research.Project(c.Build))
	case command.DesignateTame:
		g.markTame(c.X, c.Y, c.X2, c.Y2, true)
	case command.DesignatePen:
		g.paintZone(zone.KindPen, zone.Zone{}, c.X, c.Y, c.X2, c.Y2)
	case command.ReleaseAnimal:
		g.releaseAnimal(entity.EntityID(c.Pawn))
	case command.SetTroughFilter:
		g.setTroughFilter(entity.EntityID(c.Target), c.Mode)
	case command.CancelFrame:
		g.cancelFrame(entity.EntityID(c.Target)) // frame_view.go
	case command.SetForbidden:
		g.setForbidden(entity.EntityID(c.Target), c.Enabled) // forbid.go
	case command.DesignateForbid:
		g.markForbidden(c.X, c.Y, c.X2, c.Y2, c.Enabled)
	case command.DesignateDeconstruct:
		g.markDeconstruct(c.X, c.Y, c.X2, c.Y2, true)
	case command.DesignateHunt:
		g.markHunt(c.X, c.Y, c.X2, c.Y2, true)
	case command.DraftPawn:
		g.setDraft(entity.EntityID(c.Pawn), c.Enabled)
	case command.Prioritize:
		g.prioritize(entity.EntityID(c.Pawn), c.Order, c.X, c.Y, construction.Layer(c.Layer))
	case command.SetWorkEnabled:
		t, ok := workTypeOf(c.Work)
		if !ok {
			return
		}
		v := 0
		if c.Enabled {
			v = work.DefaultPriority
		}
		g.setPriority(entity.EntityID(c.Pawn), t, v)
	case command.SetWorkPriority:
		if t, ok := workTypeOf(c.Work); ok {
			g.setPriority(entity.EntityID(c.Pawn), t, c.Count)
		}
	case command.SetSchedule:
		g.setSchedule(entity.EntityID(c.Pawn), c.Index, schedule.Activity(c.Mode))
	case command.DesignateRoof:
		g.designateRoof(c.X, c.Y, c.X2, c.Y2, true)
	case command.DesignateRemoveRoof:
		g.designateRoof(c.X, c.Y, c.X2, c.Y2, false)
	case command.Undesignate:
		g.undesignate(c.Order, c.X, c.Y, c.X2, c.Y2)
	case command.MoveBuilding:
		g.moveBuilding(entity.EntityID(c.Target), construction.Facing(c.Facing), c.X, c.Y) // move.go
	case command.DesignateHome:
		g.paintHome(c.X, c.Y, c.X2, c.Y2, c.Enabled) // home.go
	}
}

func (g *Game) cmdMove(id entity.EntityID, x, y int) {
	if !g.alive(id) {
		return
	}
	g.interrupt(id, job.Cancelled)
	rec := job.Record{Kind: job.Move, TX: x, TY: y, Forced: true}
	g.startJob(id, rec)
}

// undesignate clears only the designation named by o in the rectangle;
// zones, blueprints and other marks stay (unlike cancelRect).
func (g *Game) undesignate(o command.Order, x1, y1, x2, y2 int) {
	minX, minY, maxX, maxY := zone.Normalize(x1, y1, x2, y2)
	switch o {
	case command.OrderHarvest:
		g.unmarkHarvest(minX, minY, maxX, maxY)
	case command.OrderCut:
		g.markCut(minX, minY, maxX, maxY, false)
	case command.OrderHunt:
		g.markHunt(minX, minY, maxX, maxY, false)
	case command.OrderTame:
		g.markTame(minX, minY, maxX, maxY, false)
	case command.OrderMine:
		g.markMine(minX, minY, maxX, maxY, false)
	case command.OrderSmooth:
		g.markSmooth(minX, minY, maxX, maxY, false)
	case command.OrderDeconstruct:
		g.markDeconstruct(minX, minY, maxX, maxY, false)
	}
}

// unmarkHarvest drops harvest marks on plants in the (normalized) rectangle.
func (g *Game) unmarkHarvest(minX, minY, maxX, maxY int) {
	for _, id := range g.plantIDs {
		p, ok := g.plants.Get(id)
		if !ok {
			continue
		}
		if p.X >= minX && p.X <= maxX && p.Y >= minY && p.Y <= maxY {
			p.Designated = false
			g.plants.Set(id, p)
		}
	}
}

func (g *Game) cancelRect(x1, y1, x2, y2 int) {
	minX, minY, maxX, maxY := zone.Normalize(x1, y1, x2, y2)
	keptFrames := g.frameIDs[:0]
	var refund []entity.EntityID
	for _, id := range g.frameIDs {
		fr, ok := g.frames.Get(id)
		if !ok {
			continue
		}
		if fr.X >= minX && fr.X <= maxX && fr.Y >= minY && fr.Y <= maxY {
			refund = append(refund, id)
			continue
		}
		keptFrames = append(keptFrames, id)
	}
	g.frameIDs = keptFrames
	for _, id := range refund {
		fr, _ := g.frames.Get(id)
		g.unmarkReplaced(fr)
		g.frames.Delete(id)
		g.refundFrame(id, fr) // frame_view.go: delivered materials back
	}
	g.markDeconstruct(x1, y1, x2, y2, false)
	g.markCut(x1, y1, x2, y2, false)
	g.markMine(x1, y1, x2, y2, false)
	g.markSmooth(x1, y1, x2, y2, false)
	g.markHunt(x1, y1, x2, y2, false)
	g.markTame(x1, y1, x2, y2, false)
	g.zones.Clear(zone.Rect{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}) // zones.go: shrinks or splits
	g.clearRoofPlans(minX, minY, maxX, maxY)
	g.unmarkGrass(minX, minY, maxX, maxY) // grass.go

	g.unmarkHarvest(minX, minY, maxX, maxY)

}

func (g *Game) phaseNeeds() {
	now := uint64(g.clock.Now())
	for _, id := range g.pawnIDs {
		v, _ := g.vital.Get(id)
		if v.Status == health.Dead {
			continue
		}
		g.wakeFromAnaesthetic(id, &v, now) // surgery.go
		g.heal(id, &v, now)
		g.drugTick(id, now) // drugs.go: hourly, staggered
		if v.Status == health.Downed {
			g.stainDowned(id, now) // stains.go: an untended body pools
			n, _ := g.need.Get(id)
			if n.Starving() && v.Cause != health.CauseNoFood {
				v.Cause = health.CauseNoFood
				v.DownedTick = now
				g.vital.Set(id, v)
			}
			if v.Cause == health.CauseNoFood && now-v.DownedTick >= health.StarvationTicks {
				v.Status = health.Dead
				g.vital.Set(id, v)
				g.interrupt(id, job.Cancelled)
				g.releaseBed(id)
			}
			continue
		}
		j, _ := g.jobs.Get(id)
		n, _ := g.need.Get(id)
		if j.Kind != job.Sleep || j.State != job.Do {
			n = g.decayNeeds(id, n, now) // skills.go: HungerRate, RestRate
			g.need.Set(id, n)
			g.drainJoy(id, now) // skills.go: RecreationRate, passion work
		}
		g.skillsDaily(id, now) // skills.go: decay, age, birthdays
		if n.Starving() {
			g.down(id, health.CauseNoFood)
			continue
		}
		if n.Exhausted() {
			// collapse: drop and sleep right here, this tick
			g.down(id, health.CauseNoBed)
			g.assignSleepInPlace(id)
		}
	}
}

func (g *Game) down(id entity.EntityID, cause health.Cause) {
	g.interrupt(id, job.Cancelled)
	if cause == health.CauseNoBed {
		g.logEvent(id, logEntry{Kind: logCollapsed})
	} else {
		g.logEvent(id, logEntry{Kind: logDowned})
	}
	delete(g.calm, id) // lying down ends any flight
	v, _ := g.vital.Get(id)
	v.Status, v.Cause, v.DownedTick = health.Downed, cause, uint64(g.clock.Now())
	g.vital.Set(id, v)
}

func (g *Game) releaseBed(id entity.EntityID) {
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || b.SlotOf(uint64(id)) < 0 {
			continue
		}
		g.bldgs.Set(bid, b.Release(uint64(id)))
	}
}

// sleepCellFor is where pawn id lies in bed b: its own slot's head cell
// (the slot it holds, else the slot it would get).
func (g *Game) sleepCellFor(id entity.EntityID, b construction.Building) (int, int) {
	slot := b.SlotOf(uint64(id))
	if slot < 0 {
		slot = b.Claim(uint64(id)).SlotOf(uint64(id))
	}
	return construction.SleepCell(b.Kind, b.Facing, b.X, b.Y, slot)
}

func (g *Game) alive(id entity.EntityID) bool {
	v, ok := g.vital.Get(id)
	return ok && v.Status == health.Alive
}

func (g *Game) interrupt(id entity.EntityID, reason job.FailReason) {
	j, ok := g.jobs.Get(id)
	if !ok {
		return
	}
	g.dropCarry(id)
	g.dropPatient(id)
	g.reserve.ReleaseOwner(id)
	g.releaseDelivery(id)
	if j.Kind != job.None && j.State != job.Idle && j.State != job.Complete {
		j.State = job.Fail
		j.Fail = reason
		j.Path = nil
		g.jobs.Set(id, j)
	}
	g.jobs.Set(id, job.Record{})
}

func (g *Game) dropCarry(id entity.EntityID) {
	j, ok := g.jobs.Get(id)
	if !ok || j.Carry == 0 {
		return
	}
	pos, _ := g.positions.Get(id)
	st, ok := g.stacks.Get(j.Carry)
	if !ok {
		return
	}
	st.Carrier = 0
	st.X, st.Y = pos.X, pos.Y
	g.mergeOrSet(j.Carry, st)
	j.Carry = 0
	g.jobs.Set(id, j)
}

func (g *Game) phaseAI() {
	g.postSweep() // postpone.go: hourly
	for _, id := range g.pawnIDs {
		act := g.activityOf(id)
		if act != schedule.Sleep {
			delete(g.restedFree, id) // free time ends with the Sleep block
		}
		v, _ := g.vital.Get(id)
		if v.Status == health.Dead {
			delete(g.calm, id)
			delete(g.restedFree, id)
			g.reserve.ReleaseOwner(id)
			g.releaseDelivery(id)
			continue
		}
		if v.Status == health.Downed {
			j, _ := g.jobs.Get(id)
			if v.Cause == health.CauseNoBed && j.Kind != job.Sleep {
				g.assignSleepInPlace(id)
			}
			continue
		}
		j, _ := g.jobs.Get(id)
		if j.State == job.Complete || j.State == job.Fail {
			g.postponeJob(id, j) // postpone.go: blocked or out of reach
			if j.Forced {
				g.forcedJobEnded(id, j) // a blocked order goes back on the queue
			}
			g.dropCarry(id)
			g.dropPatient(id)
			g.reserve.ReleaseOwner(id)
			g.releaseDelivery(id)
			g.jobs.Set(id, job.Record{})
			j = job.Record{}
		}
		g.releaseStaleDelivery(id, j) // a delivery swapped out without an interrupt
		if g.mentalBreakTick(id, j) {
			continue // acting out: no orders, no work
		}
		if g.socialDuty(id, j) {
			continue // gathering.go: a party, wedding or social fight
		}
		// an unarmed colonist runs from an attacker before its orders,
		// work or sleep; queued orders wait for it to calm down (flee.go)
		if !g.drafted(id) && !g.fightsBack(id) && g.defend(id, j) {
			continue
		}
		if g.orderDue(id) && !g.followingOrder(id) && (g.drafted(id) || !needs.SleepScheduled(act)) {
			g.nextOrder(id)
			continue
		}
		if g.drafted(id) {
			// drafted pawns only follow direct orders, but fire at
			// hostiles in range when standing idle
			if j.Kind == job.None || j.State == job.Idle {
				g.autoEngage(id)
			}
			continue
		}
		if g.fightsBack(id) && g.defend(id, j) {
			continue
		}
		// Warmth only wakes a sleeper when the exposure turns dangerous, and
		// nothing is interrupted unless a bearable spot was actually found.
		if j.Kind != job.Warm && j.Kind != job.Eat && g.needsComfort(id) && !g.noComfortUntil(id) &&
			(j.Kind != job.Sleep || g.exposureDanger(id)) {
			if x, y, ok := g.warmSpot(id); ok {
				if j.Kind != job.None && j.State != job.Idle {
					g.interrupt(id, job.Interrupted)
				}
				if g.startJob(id, job.Record{Kind: job.Warm, TX: x, TY: y, WorkLeft: warmWait}) {
					continue
				}
				j, _ = g.jobs.Get(id)
			}
			g.noComfort[id] = uint64(g.clock.Now()) + warmRetry
		}
		if j.Kind != job.Sleep && j.Kind != job.Eat && j.Kind != job.Warm && !j.Forced && g.patientRest(id) {
			if j.Kind != job.None && j.State != job.Idle {
				g.interrupt(id, job.Interrupted)
			}
			g.assignSleep(id) // lie down and wait for a doctor
			continue
		}
		n, _ := g.need.Get(id)

		busy := j.Kind != job.None && j.State != job.Idle
		// A tired colonist sleeps whenever its schedule allows it (not in
		// Work hours, where it keeps going until it collapses). Rest at
		// zero never reaches here: phaseNeeds has already collapsed it.
		tired := n.Tired() && needs.SleepAllowed(act)
		// The schedule comes first: at bedtime a colonist drops whatever it
		// is doing (even a prioritized order) and goes to bed, eating first
		// if it is hungry. Only drafting overrides it. One that already
		// woke fully rested this block has free time until it tires.
		if needs.SleepScheduled(act) && n.Rest >= needs.Full {
			g.restedFree[id] = true
		}
		bedtime := needs.SleepScheduled(act) && (!g.restedFree[id] || n.Tired())
		if bedtime && j.Kind != job.Sleep && j.Kind != job.Eat && j.Kind != job.Warm {
			if busy {
				g.interrupt(id, job.Interrupted)
			}
			if n.Hungry() && g.seekFood(id) {
				g.assignEat(id)
			} else {
				g.assignSleep(id)
			}
			continue
		}
		if busy && j.Kind.IsIdle() {
			needy := n.Hungry() && g.seekFood(id) || tired
			if !needy && (uint64(g.clock.Now())+uint64(id))%idleRecheck != 0 {
				continue // keep idling; look for real work now and then
			}
			busy = false
		}
		if busy && j.Forced {
			continue // player-prioritized: finish it first
		}
		if busy {
			if n.Hungry() && j.Kind != job.Eat && g.seekFood(id) {
				g.interrupt(id, job.Interrupted)
				g.assignEat(id)
				continue
			}
			if tired && j.Kind != job.Sleep && j.Kind != job.Eat && j.Kind != job.Warm {
				g.interrupt(id, job.Interrupted)
				g.assignSleep(id)
				continue
			}
			continue
		}

		if n.Hungry() && g.seekFood(id) {
			g.assignEat(id)
			continue
		}
		if tired || bedtime {
			g.assignSleep(id)
			continue
		}
		// a free-time activity still running when work was looked for
		var idling job.Record
		if j.Kind.IsIdle() && j.Active() {
			idling = j
		}
		if !needs.WorkScheduled(act) {
			if g.assignWear(id) || g.assignDrugUse(id) { // drugs.go: a drink or a craving first
				continue
			}
			g.keepIdling(id, idling) // recreation: free time
			continue
		}
		if g.assignWorkByPriority(id) {
			continue
		}
		if g.assignWear(id) || g.assignDrugUse(id) {
			continue
		}
		g.keepIdling(id, idling) // nothing to do: free time
	}
}

// keepIdling carries on with a running free-time activity, or picks a new
// one. A work giver that tried and failed may have written a failed job
// over the activity; that must not restart it every idle recheck.
func (g *Game) keepIdling(id entity.EntityID, idling job.Record) {
	cur, _ := g.jobs.Get(id)
	if cur.Kind.IsIdle() && cur.Active() {
		return
	}
	if idling.Kind != job.None && (cur.Kind == job.None || cur.State == job.Fail) &&
		(idling.Target == 0 || g.reserve.TryEntity(id, idling.Target)) {
		g.jobs.Set(id, idling)
		return
	}
	g.assignIdle(id)
}

func (g *Game) startJob(id entity.EntityID, rec job.Record) bool {
	g.reserve.ReleaseOwner(id)
	g.releaseDelivery(id) // a replaced delivery gives its shares back
	// areas.go: a job outside the pawn's allowed area fails fast
	if g.areaRejects(id, rec) {
		g.rejectOutOfArea(id, rec)
		return false
	}
	if g.areaBypass(id, rec) { // areas.go: a forced or exempt job paths anywhere
		defer g.endAreaBypass()
	}
	if rec.Target != 0 && rec.Kind != job.Attack && rec.Kind != job.Sleep && !g.reserve.TryEntity(id, rec.Target) {
		rec.State = job.Fail
		rec.Fail = job.Reserved
		g.jobs.Set(id, rec)
		return false
	}
	if rec.Kind == job.Move && !g.reserve.TryTile(id, rec.TX, rec.TY) {
		g.reserve.ReleaseOwner(id)
		rec.State = job.Fail
		rec.Fail = job.Reserved
		g.jobs.Set(id, rec)
		return false
	}
	// a delivery route (HAUL-001) books frames through incoming shares
	// and holds only its spare stockpile cell
	if rec.Kind == job.Haul && len(rec.Route) == 0 && !g.reserve.TryTile(id, rec.DestX, rec.DestY) ||
		rec.HasSpare && !g.reserve.TryTile(id, rec.SpareX, rec.SpareY) {
		g.reserve.ReleaseOwner(id)
		rec.State = job.Fail
		rec.Fail = job.Reserved
		g.jobs.Set(id, rec)
		return false
	}
	path, ok := g.pathToJob(id, rec.TX, rec.TY) // paths.go: long searches are budgeted
	if !ok && g.pathDeferred() {
		g.reserve.ReleaseOwner(id)
		rec.State = job.Fail
		rec.Fail = job.Reserved // not unreachable: tried again next tick
		g.jobs.Set(id, rec)
		return false
	}
	if !ok {
		g.reserve.ReleaseOwner(id)
		rec.State = job.Fail
		rec.Fail = job.Unreachable
		g.jobs.Set(id, rec)
		g.postponeJob(id, rec) // postpone.go: givers move on to the next target
		return false
	}
	rec.Path = path
	rec.PathI = 0
	if len(path) == 0 {
		if rec.Kind == job.Move {
			rec.State = job.Complete
		} else {
			rec.State = job.Do
		}
	} else {
		rec.State = job.Goto
	}
	g.jobs.Set(id, rec)
	return true
}

func (g *Game) pathTo(id entity.EntityID, tx, ty int) ([]pathing.Point, bool) {
	return g.pathToVia(id, tx, ty, !g.noRegions())
}

// pathToVia is pathTo; with regions, targets in another component fail at
// once instead of costing a flood of the start's (paths.go).
func (g *Game) pathToVia(id entity.EntityID, tx, ty int, regions bool) ([]pathing.Point, bool) {
	pos, _ := g.positions.Get(id)
	if pos.X == tx && pos.Y == ty {
		return nil, true
	}
	q := g.pathQuery(id)
	reach := g.reachFrom(id, pos, regions)
	if !regions {
		q = plainQuery{q} // regions.go: a plain search (self-check reference)
	}
	if q.Walkable(tx, ty) {
		if !reach.ok(tx, ty) {
			return nil, false
		}
		return g.findPathVia(id, q, regions, pos.X, pos.Y, tx, ty)
	}
	if pathing.Adjacent(pos.X, pos.Y, tx, ty) {
		return nil, true
	}
	var best []pathing.Point
	bestOK := false
	bestLen := 1 << 30
	bestY, bestX := 1<<30, 1<<30
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			ax, ay := tx+dx, ty+dy
			if !q.Walkable(ax, ay) || !reach.ok(ax, ay) {
				continue
			}
			p, ok := g.findPathVia(id, q, regions, pos.X, pos.Y, ax, ay)
			if !ok {
				continue
			}
			endY, endX := ay, ax
			if len(p) == bestLen && (endY > bestY || (endY == bestY && endX >= bestX)) {
				continue
			}
			if len(p) > bestLen {
				continue
			}
			best, bestOK, bestLen = p, true, len(p)
			bestY, bestX = endY, endX
		}
	}
	return best, bestOK
}

func (g *Game) hasEdible() bool {
	for _, id := range g.itemIDs {
		st, ok := g.stacks.Get(id)
		if !ok || !st.Kind.IsFood() || st.Qty <= 0 || st.Carrier != 0 || st.Forbidden {
			continue
		}
		if _, taken := g.reserve.EntityOwner(id); taken {
			continue
		}
		return true
	}
	return false
}

// foodRetry is how long a colonist whose food is out of reach waits before
// trying to eat again (it works or sleeps meanwhile instead of retrying
// every tick).
const foodRetry = 500

// seekFood reports whether pawn id should go looking for food now.
func (g *Game) seekFood(id entity.EntityID) bool {
	return uint64(g.clock.Now()) >= g.noFood[id] && g.hasEdible()
}

func (g *Game) assignEat(id entity.EntityID) {
	best, bx, by := g.bestFood(id)
	if best == 0 {
		return
	}
	if !g.startJob(id, job.Record{
		Kind: job.Eat, Target: best, TX: bx, TY: by, WorkLeft: needs.EatWorkTicks,
	}) {
		g.noFood[id] = uint64(g.clock.Now()) + foodRetry
	}
}

func (g *Game) assignSleepInPlace(id entity.EntityID) {
	pos, _ := g.positions.Get(id)
	g.sleptAt[id] = 0
	g.startJob(id, job.Record{Kind: job.Sleep, TX: pos.X, TY: pos.Y, WorkLeft: needs.SleepWorkMin})
}

func (g *Game) assignSleep(id entity.EntityID) {
	if !g.alive(id) {
		g.assignSleepInPlace(id)
		return
	}
	if bid, slot, ok := g.partnerBed(id); ok && g.sleepInPartnerBed(id, bid, slot) { // beds.go
		return
	}
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || !b.Kind.Sleeps() || !b.CanClaim(uint64(id)) || b.ForPrisoners { // prison beds: prisoners only
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, b.X, b.Y)
		if b.SlotOf(uint64(id)) >= 0 {
			d = -1 // its own bed first
		}
		if d < bestD || (d == bestD && bid < best) {
			best, bestD = bid, d
		}
	}
	if best != 0 {
		b, _ := g.bldgs.Get(best)
		if b.SlotOf(uint64(id)) < 0 {
			g.releaseBed(id) // moving to a new bed gives up the old one
		}
		b = b.Claim(uint64(id))
		g.bldgs.Set(best, b)
		g.sleptAt[id] = best
		sx, sy := g.sleepCellFor(id, b)
		if !g.startJob(id, job.Record{Kind: job.Sleep, Target: best, TX: sx, TY: sy, WorkLeft: needs.SleepWorkMin}) {
			g.bldgs.Set(best, b.Release(uint64(id)))
			g.assignSleepInPlace(id) // the bed is out of reach: the ground will do
		}
		return
	}
	g.sleptAt[id] = 0
	g.startJob(id, job.Record{Kind: job.Sleep, TX: pos.X, TY: pos.Y, WorkLeft: needs.SleepWorkMin})
}

func (g *Game) assignHaul(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	for range workTries {
		rec, shares, ok := g.planDelivery(id, pos)
		if !ok {
			break
		}
		if g.startDelivery(id, rec, shares) {
			return true
		}
		if !g.postponedFor(id, job.Haul, rec.Target, 0, 0) {
			break // not out of reach: planning again picks the same load
		}
	}
	return g.haulToStockpile(id, pos) // nothing to supply (or it fell through)
}

// haulToStockpile carries the nearest loose stack to a stockpile, trying
// the next nearest when one can't be started (postponed or out of reach:
// SIM-027), at most workTries stacks; kinds with no stockpile room are
// skipped once found full.
func (g *Game) haulToStockpile(id entity.EntityID, pos Position) bool {
	var tried []entity.EntityID
	var full []item.Kind
	var takes [256]int8 // filterMatches per kind (0 unknown, 1 yes, -1 no): zones don't change here
	var postBuf [8]entity.EntityID
	for range workTries {
		var best item.Stack
		var bestID entity.EntityID
		bestD := 1 << 30
		found := false
		// For a stack not put off, postponedFor is false and prunes
		// nothing: its pure filters then run cheapest-first (most loose
		// stacks already lie on a stockpile that takes them). A put-off
		// one keeps the original order, so pruning is unchanged.
		post := g.postponedTargets(id, job.Haul, postBuf[:0])
		for _, iid := range g.itemIDs {
			st := g.stacks.Ref(iid) // read only: nothing below changes the store
			if st == nil || st.Qty <= 0 || st.Carrier != 0 || st.Forbidden {
				continue
			}
			off := len(post) > 0 && slices.Contains(post, iid)
			if !off && g.inMatchingStockpileAt(st.Kind, st.X, st.Y) {
				continue
			}
			if _, taken := g.reserve.EntityOwner(iid); taken {
				continue
			}
			if slices.Contains(tried, iid) || slices.Contains(full, st.Kind) || off && g.postponedFor(id, job.Haul, iid, 0, 0) {
				continue
			}
			if off && g.inMatchingStockpileAt(st.Kind, st.X, st.Y) {
				continue
			}
			if takes[st.Kind] == 0 {
				takes[st.Kind] = -1
				if g.filterMatches(st.Kind) {
					takes[st.Kind] = 1
				}
			}
			if takes[st.Kind] < 0 {
				continue
			}
			d := pathing.Dist(pos.X, pos.Y, st.X, st.Y)
			if !found || d < bestD || (d == bestD && iid < bestID) {
				found, bestD, bestID, best = true, d, iid, *st
			}
		}
		if !found {
			return false
		}
		dx, dy, ok := g.stockpileCell(best.Kind, best.X, best.Y)
		if !ok {
			full = append(full, best.Kind)
			continue
		}
		if g.startJob(id, job.Record{
			Kind: job.Haul, Target: bestID, TX: best.X, TY: best.Y, DestX: dx, DestY: dy,
		}) {
			return true
		}
		tried = append(tried, bestID)
	}
	return false
}

// assignConstruct builds the nearest ready frame or takes down the nearest
// marked building, trying the next nearest when one can't be started
// (postponed or out of reach: SIM-027), at most workTries.
func (g *Game) assignConstruct(id entity.EntityID) bool {
	var tried []entity.EntityID
	for range workTries {
		t, ok := g.constructTry(id, tried)
		if ok || t == 0 {
			return ok
		}
		tried = append(tried, t)
	}
	return false
}

// constructTry starts the nearest construct or deconstruct job whose
// target is not in tried; it returns the target tried (0: none left).
func (g *Game) constructTry(id entity.EntityID, tried []entity.EntityID) (entity.EntityID, bool) {
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	var bx, by int
	for _, fid := range g.frameIDs {
		fr := g.frames.Ref(fid) // read only: nothing below changes the store
		if fr == nil || !fr.Ready() || g.replacedStanding(*fr) != 0 {
			continue // a door waits for the wall it replaces to come down
		}
		if _, taken := g.reserve.EntityOwner(fid); taken {
			continue
		}
		if slices.Contains(tried, fid) || g.postponedFor(id, job.Construct, fid, 0, 0) {
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, fr.X, fr.Y)
		if fr.Rebuild {
			d -= rebuildFirst
		}
		if d < bestD || (d == bestD && fid < best) {
			best, bestD, bx, by = fid, d, fr.X, fr.Y
		}
	}
	for _, bid := range g.bldgIDs {
		b := g.bldgs.Ref(bid)
		if b == nil || !b.Deconstruct {
			continue
		}
		if _, taken := g.reserve.EntityOwner(bid); taken {
			continue
		}
		if slices.Contains(tried, bid) || g.postponedFor(id, job.Deconstruct, bid, 0, 0) {
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, b.X, b.Y)
		if d < bestD || (d == bestD && bid < best) {
			best, bestD, bx, by = bid, d, b.X, b.Y
		}
	}
	if best == 0 {
		return 0, false
	}
	if b, ok := g.bldgs.Get(best); ok {
		tx, ty := g.workSpot(id, bx, by)
		return best, g.startJob(id, job.Record{
			Kind: job.Deconstruct, Target: best, TX: tx, TY: ty, WorkLeft: b.WorkLeft,
		})
	}
	fr, _ := g.frames.Get(best)
	return best, g.startJob(id, job.Record{
		Kind: job.Construct, Target: best, TX: bx, TY: by, WorkLeft: fr.WorkLeft,
	})
}

func (g *Game) assignHarvest(id entity.EntityID) bool {
	best := g.nearestPlant(id, job.Harvest, wantsHarvest)
	if best == 0 {
		return false
	}
	bp, _ := g.plants.Get(best)
	return g.startJob(id, job.Record{
		Kind: job.Harvest, Target: best, TX: bp.X, TY: bp.Y, WorkLeft: plant.DefOf(bp.Species).HarvestWork,
	})
}

func (g *Game) phaseJobs() {
	now := uint64(g.clock.Now())
	for _, id := range g.pawnIDs {
		v, _ := g.vital.Get(id)
		if v.Status == health.Dead {
			continue
		}
		j, _ := g.jobs.Get(id)
		if j.Kind == job.Hunt || j.Kind == job.Attack {
			if v.Status == health.Alive && (j.State == job.Goto || j.State == job.Do) {
				g.doCombat(id, j, now)
			}
			continue
		}
		if j.State != job.Do {
			continue
		}
		if v.Status == health.Downed && j.Kind != job.Sleep {
			continue
		}
		g.dirtWork(id, j, now) // filth.go: field work soils the boots
		switch j.Kind {
		case job.Eat:
			g.doEat(id, j)
		case job.Sleep:
			g.doSleep(id, j, now)
		case job.Haul:
			g.doHaul(id, j)
		case job.Bury:
			g.doBury(id, j)
		case job.Roof, job.Unroof:
			g.doRoof(id, j)
		case job.Construct:
			g.doConstruct(id, j)
		case job.Harvest:
			g.doHarvest(id, j)
		case job.Deconstruct:
			g.doDeconstruct(id, j)
		case job.Cut:
			g.doCut(id, j)
		case job.Mine:
			g.doMine(id, j)
		case job.Smooth:
			g.doSmooth(id, j) // smooth.go
		case job.Sow:
			g.doSow(id, j)
		case job.Bill:
			g.doBill(id, j)
		case job.Equip:
			g.doEquip(id, j)
		case job.Rescue:
			g.doRescue(id, j)
		case job.Tend:
			g.doTend(id, j)
		case job.Refuel:
			g.doRefuel(id, j)
		case job.Warm:
			g.doWarm(id, j)
		case job.Repair:
			g.doRepair(id, j)
		case job.Research:
			g.doResearch(id, j)
		case job.Tame:
			g.doTame(id, j)
		case job.Milk, job.Shear:
			g.doCollect(id, j) // animal_products.go
		case job.Clean:
			g.doClean(id, j) // clean.go
		case job.Wander, job.Chat, job.Relax, job.FireSide, job.Horseshoes, job.Hide:
			g.doIdle(id, j)
		// ENG-003: social and host-system jobs, each in its owner's file.
		case job.Surgery:
			g.doSurgery(id, j) // surgery.go
		case job.Ingest:
			g.doIngest(id, j) // drugs.go
		case job.Lovin:
			g.doLovin(id, j) // beds.go
		case job.Gather:
			g.doGather(id, j) // gathering.go
		case job.Brawl:
			g.doBrawl(id, j) // social_fight.go
		case job.Capture:
			g.doCapture(id, j) // prisoners.go
		case job.Warden:
			g.doWarden(id, j) // prisoners.go
		case job.DeliverFood:
			g.doDeliverFood(id, j) // prisoners.go
		case job.SocialRelax:
			g.doSocialRelax(id, j) // social.go
		case job.Train:
			g.doTrain(id, j) // bonds.go
		case job.Arrest:
			g.doArrest(id, j) // prisoners.go
		case job.Kidnap:
			g.doKidnap(id, j) // world_state.go
		case job.ExitMap:
			g.doExitMap(id, j) // world_state.go
		case job.TendAnimal:
			g.doTendAnimal(id, j) // bonds.go
		case job.Visit:
			g.doVisit(id, j) // world_state.go
		default:
			j.State = job.Complete
			g.jobs.Set(id, j)
		}
	}
}

func (g *Game) doEat(id entity.EntityID, j job.Record) {
	if j.Count == 0 && j.Carry == 0 {
		if g.carryToTable(id, j) {
			return // walking to a table with the food
		}
		j.Count = 2 // no table in reach: eat right here
	}
	j.WorkLeft--
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	st, ok := g.stacks.Get(j.Target)
	if !ok || st.Qty <= 0 {
		j.State = job.Fail
		j.Fail = job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	// eat portions until satisfied (several lettuces make a meal; one
	// ration does), never past full by more than half a portion
	n, _ := g.need.Get(id)
	nut := st.Kind.Nutrition()
	for eaten := 0; st.Qty > 0 && eaten < 12; eaten++ {
		st.Qty--
		n.Hunger += nut
		if j.Count == bingeEat && eaten < 2 {
			continue // binge: stuffs itself well past full
		}
		if n.Hunger+nut/2 > needs.Full {
			break
		}
	}
	if st.Qty <= 0 {
		g.deleteItem(j.Target)
		j.Carry = 0
	} else {
		g.stacks.Set(j.Target, st) // leftovers: dropped where it ate
	}
	g.lastMeal[id] = st.Kind
	g.afterEat(id, st.Kind)
	if n.Hunger > needs.Full {
		n.Hunger = needs.Full
	}
	g.need.Set(id, n)
	g.ateAt[id] = true
	g.logEvent(id, logEntry{Kind: logAte, Name: st.Kind.String()})
	j.State = job.Complete
	g.jobs.Set(id, j)
}

func (g *Game) doSleep(id entity.EntityID, j job.Record, now uint64) {
	n, _ := g.need.Get(id)
	n = needs.SleepTick(n, now)
	g.need.Set(id, n)
	j.WorkLeft--
	if g.sleepDone(id, n, j) {
		j.State = job.Complete
		if g.sleptAt[id] != 0 {
			g.logEvent(id, logEntry{Kind: logSleptBed})
		} else {
			g.logEvent(id, logEntry{Kind: logSleptGround})
		}
		g.afterSleep(id)
	}
	g.jobs.Set(id, j)
}

// sleepDone reports whether a sleeper gets up now. A collapsed colonist
// sleeps to WakeRest whatever happens; otherwise it wakes to eat, at full
// rest in Sleep hours (the rest of the block is free time), with the start
// of Work hours once over the tired line, and at WakeRest in any other
// block. These gaps keep it from bouncing between bed and work.
func (g *Game) sleepDone(id entity.EntityID, n needs.Pair, j job.Record) bool {
	if v, ok := g.vital.Get(id); ok && v.Status == health.Downed {
		if v.Cause != health.CauseNoBed || n.Rest < needs.WakeRest {
			return false
		}
		v.Status, v.Cause = health.Alive, health.CauseNone
		g.vital.Set(id, v)
	}
	if n.Hungry() && g.seekFood(id) {
		return true
	}
	act := g.activityOf(id)
	var done bool
	switch {
	case needs.SleepScheduled(act):
		done = n.Rest >= needs.Full
		if done {
			g.restedFree[id] = true
		}
	case needs.SleepAllowed(act):
		done = n.Rest >= needs.WakeRest
	default: // Work hours
		done = n.Rest > needs.CriticalRest && j.WorkLeft <= 0
	}
	if done && g.patientRest(id) {
		if j.WorkLeft > -patientWait {
			done = false // stay put: a doctor is coming
		} else {
			g.surgeryGaveUp(id) // surgery.go: no surgeon came; don't lie straight back down
		}
	}
	return done
}

func (g *Game) doHaul(id entity.EntityID, j job.Record) {
	pos, _ := g.positions.Get(id)
	if j.Carry == 0 {
		st, ok := g.stacks.Get(j.Target)
		if !ok || st.Qty <= 0 {
			j.State = job.Fail
			j.Fail = job.TargetGone
			g.jobs.Set(id, j)
			return
		}
		take := st.Qty
		if take > item.CarryLimit {
			take = item.CarryLimit
		}
		if len(j.Route) > 0 {
			take = min(st.Qty, max(1, j.Count)) // the planned load (haul_route.go)
		} else if fr, ok := g.frames.Get(j.Dest); ok && j.Dest != 0 {
			if need := fr.Need - fr.Wood; need > 0 && take > need {
				take = need
			}
		}
		carry := j.Target
		if take < st.Qty { // split off one load, leave the rest
			st.Qty -= take
			g.stacks.Set(j.Target, st)
			carry = g.entities.Next()
			g.itemIDs = append(g.itemIDs, carry)
			st = splitLoad(st, take)
		}
		st.Carrier = id
		g.stacks.Set(carry, st)
		j.Carry = carry
		if len(j.Route) > 0 {
			g.trimDelivery(id, take)
			if !g.routeWalk(id, &j, st.Kind) {
				g.routeArrive(id, j)
			}
			return
		}
		path, ok := g.pathTo(id, j.DestX, j.DestY)
		if !ok {
			st.Carrier = 0
			st.X, st.Y = pos.X, pos.Y
			g.mergeOrSet(carry, st)
			j.Carry = 0
			j.State = job.Fail
			j.Fail = job.Unreachable
			g.jobs.Set(id, j)
			return
		}
		j.Path, j.PathI = path, 0
		if len(path) == 0 {
			g.jobs.Set(id, j)
			g.finishHaulDrop(id, j)
			return
		}
		j.State = job.Goto
		g.jobs.Set(id, j)
		return
	}
	if len(j.Route) > 0 {
		g.routeArrive(id, j)
		return
	}
	g.finishHaulDrop(id, j)
}

func (g *Game) finishHaulDrop(id entity.EntityID, j job.Record) {
	st, ok := g.stacks.Get(j.Carry)
	if !ok {
		j.State = job.Fail
		j.Fail = job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	st.Carrier = 0
	st.X, st.Y = j.DestX, j.DestY
	fid := j.Dest
	fr, isFrame := g.frames.Get(fid)
	if fid != 0 && isFrame && st.Kind == fr.Stuff {
		need := fr.Need - fr.Wood
		give := st.Qty
		if give > need {
			give = need
		}
		fr.Wood += give
		st.Qty -= give
		g.frames.Set(fid, fr)
		if st.Qty <= 0 {
			g.deleteItem(j.Carry)
		} else {
			pos, _ := g.positions.Get(id)
			st.X, st.Y = pos.X, pos.Y
			g.stacks.Set(j.Carry, st)
		}
	} else {
		g.mergeOrSet(j.Carry, st)
	}
	j.Carry = 0
	j.State = job.Complete
	g.jobs.Set(id, j)
}

func (g *Game) doConstruct(id entity.EntityID, j job.Record) {
	fr, ok := g.frames.Get(j.Target)
	if !ok {
		j.State = job.Fail
		j.Fail = job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	u := g.workProgress(id, skConstruction, true) // skills.go
	if u == 0 {
		g.jobs.Set(id, j)
		return
	}
	fr.WorkLeft -= u
	j.WorkLeft = fr.WorkLeft
	if fr.WorkLeft > 0 {
		g.frames.Set(j.Target, fr)
		g.jobs.Set(id, j)
		return
	}
	g.completeFrame(j.Target, fr)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

func (g *Game) completeFrame(id entity.EntityID, fr construction.Frame) {
	if g.replacedStanding(fr) != 0 {
		return // never built while the piece it replaces stands
	}
	if fr.Reinstall != 0 {
		g.completeMove(id, fr) // move.go: the same building moves here
		return
	}
	g.frames.Delete(id)
	kept := g.frameIDs[:0]
	for _, fid := range g.frameIDs {
		if fid != id {
			kept = append(kept, fid)
		}
	}
	g.frameIDs = kept
	g.bldgIDs = append(g.bldgIDs, id)
	g.bldgs.Set(id, construction.Building{Kind: fr.Kind, Stuff: fr.Stuff, Facing: fr.Facing, X: fr.X, Y: fr.Y})
	g.noteBuilding(id) // indexes.go
	g.markRoomsDirty()
	g.structVer++                              // SIM-029: link/autotile caches
	g.autoHome(fr.Kind, fr.Facing, fr.X, fr.Y) // home.go
	g.defaultBills(id, fr.Kind)
	// bushes and crops under the footprint are cleared below
	cells, _, _ := construction.Footprint(fr.Kind, fr.Facing, fr.X, fr.Y)
	for _, c := range cells {
		if pid := g.plantAt(c[0], c[1]); pid != 0 {
			g.removePlant(pid)
		}
		g.clearGrass(c[0], c[1]) // grass.go
	}
	if construction.DefOf(fr.Kind).BlocksMove {
		for _, c := range cells {
			g.grid.SetWalkable(c[0], c[1], false)
		}
		for _, c := range cells {
			g.ejectPawns(c[0], c[1])
		}
	}
}

func (g *Game) doDeconstruct(id entity.EntityID, j job.Record) {
	b, ok := g.bldgs.Get(j.Target)
	if !ok || !b.Deconstruct {
		j.State = job.Fail
		j.Fail = job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	u := g.workProgress(id, skConstruction, true) // skills.go
	if u == 0 {
		g.jobs.Set(id, j)
		return
	}
	b.WorkLeft -= u
	j.WorkLeft = b.WorkLeft
	if b.WorkLeft > 0 {
		g.bldgs.Set(j.Target, b)
		g.jobs.Set(id, j)
		return
	}
	g.reserve.ReleaseOwner(id)
	g.removeBuilding(j.Target)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

func (g *Game) doHarvest(id entity.EntityID, j job.Record) {
	p, ok := g.plants.Get(j.Target)
	if !ok {
		j.State = job.Fail
		j.Fail = job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	u := g.workProgress(id, skPlants, true) // skills.go
	if u == 0 {
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft -= u
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	d := plant.DefOf(p.Species)
	if g.fumbled(id) { // low Plants skill: nothing comes of it (skills.go)
		pos, _ := g.positions.Get(id)
		g.message(pos.X, pos.Y, "%s fumbled the harvest.", g.name(id))
	} else {
		g.dropStack(d.Yields, g.skillYield(id, skPlants, g.diffYield(DiffCrop, d.Yield)), p.X, p.Y)
	}
	if d.ExtraQty > 0 {
		g.dropStack(d.Extra, d.ExtraQty, p.X, p.Y) // a by-product: straw with wheat
	}
	if d.Regrows {
		p.Growth = d.GrowTicks * d.RegrowTo / 100
		p.Designated = false
		g.plants.Set(j.Target, p)
	} else {
		g.removePlant(j.Target)
	}
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// phaseMovement steps every due creature (colonists, raiders, animals)
// in ascending ID, applying each move at once so the occupancy index
// sees earlier moves of the same tick (SIM-014).
func (g *Game) phaseMovement() {
	now := uint64(g.clock.Now())
	movers := g.movers()
	g.pruneMotion(movers)
	g.buildOccupancy()
	var raiders []entity.EntityID
	for _, mv := range movers {
		m := g.motionOf(mv.id)
		if now < m.NextMoveAt {
			continue // mid-step
		}
		if mv.animal {
			g.moveAnimal(mv.id, m, now)
		} else if g.alive(mv.id) && g.movePawn(mv.id, m, now) && g.isRaider(mv.id) {
			raiders = append(raiders, mv.id)
		}
	}
	g.followCarriers()
	g.trapsUnderfoot(raiders)
}

// movePawn takes a colonist's or raider's next path step if it is due
// (m, its record, says it is due) and the way is clear; it reports
// whether the pawn changed cell.
func (g *Game) movePawn(id entity.EntityID, m motionRec, now uint64) bool {
	j, _ := g.jobs.Get(id)
	if j.State != job.Goto || j.PathI >= len(j.Path) {
		if m.BlockedTicks != 0 || m.BlockedBy != 0 {
			m.BlockedTicks, m.BlockedBy = 0, 0
			g.motion.Set(id, m)
		}
		return false
	}
	next := j.Path[j.PathI]
	tx, ty := walkTarget(j)
	if !g.grid.Walkable(next.X, next.Y) || g.isRaider(id) && g.doorAt(next.X, next.Y) {
		path, ok := g.pathTo(id, tx, ty)
		if !ok {
			j.State = job.Fail
			j.Fail = job.Unreachable
			g.jobs.Set(id, j)
			return false
		}
		j.Path, j.PathI = path, 0
		g.jobs.Set(id, j)
		if len(path) == 0 {
			g.arrive(id, j)
		}
		return false
	}
	from, _ := g.positions.Get(id)
	to := Position{X: next.X, Y: next.Y}
	v, alt := g.decide(id, from, to)
	switch v {
	case stepGo:
		j.PathI++
		if j.PathI >= len(j.Path) {
			if j.Kind == job.Move {
				j.State = job.Complete
			} else {
				j.State = job.Do
			}
		}
		g.jobs.Set(id, j)
		g.stepPawn(id, &m, j.Carry, from, to, now)
		return true
	case stepSidestep:
		g.stepPawn(id, &m, j.Carry, from, alt, now)
		path, ok := g.pathTo(id, tx, ty)
		if !ok {
			j.State, j.Fail = job.Fail, job.Unreachable
			g.jobs.Set(id, j)
			return true
		}
		j.Path, j.PathI = path, 0
		g.jobs.Set(id, j)
		if len(path) == 0 {
			g.arrive(id, j)
		}
		return true
	case stepArriveNear:
		moved := alt != from
		if moved {
			g.stepPawn(id, &m, j.Carry, from, alt, now)
		}
		g.arrive(id, j)
		return moved
	}
	// stepWait or stepRepath: stay put and try again next tick
	m.BlockedTicks++
	m.BlockedBy, _ = g.occupantAt(to.X, to.Y)
	if v == stepRepath {
		if path, ok := g.pathAround(id, tx, ty); ok {
			j.Path, j.PathI = path, 0
			g.jobs.Set(id, j)
			if len(path) == 0 {
				g.arrive(id, j)
			}
		}
	}
	if m.BlockedTicks > g.blockedLimit(id, m.BlockedBy, v) {
		if j, _ := g.jobs.Get(id); j.State == job.Goto {
			j.State, j.Fail = job.Fail, job.Blocked
			g.jobs.Set(id, j)
		}
		m.BlockedTicks, m.BlockedBy = 0, 0
	}
	g.motion.Set(id, m)
	return false
}

// stepPawn moves a pawn (and what it carries) from -> to at tick now.
func (g *Game) stepPawn(id entity.EntityID, m *motionRec, carry entity.EntityID, from, to Position, now uint64) {
	g.positions.Set(id, to)
	g.stainStep(id, from) // stains.go: a bleeding pawn drips
	g.dirtStep(id, to)    // filth.go: boots pick dirt up or leave it
	// skills.go: step time ÷ MoveSpeed
	m.stepped(now, g.paceStep(id, g.weatherSlow(to.X, to.Y, pawnStepCost(from, to))))
	g.motion.Set(id, *m)
	if carry != 0 {
		if st, ok := g.stacks.Get(carry); ok {
			st.X, st.Y = to.X, to.Y
			g.stacks.Set(carry, st)
		}
	}
}

func (g *Game) arrive(id entity.EntityID, j job.Record) {
	if j.Kind == job.Move {
		j.State = job.Complete
	} else {
		j.State = job.Do
	}
	g.jobs.Set(id, j)
}

func (g *Game) itemAt(x, y int) entity.EntityID {
	for _, id := range g.itemIDs {
		if st := g.stacks.Ref(id); st != nil && st.Carrier == 0 && st.X == x && st.Y == y {
			return id
		}
	}
	return 0
}

func (g *Game) deleteItem(id entity.EntityID) {
	g.stacks.Delete(id)
	g.itemIDs = dropIDs(g.itemIDs, func(iid entity.EntityID) bool { return iid == id })
}

// mergeOrSet puts stack id down at st's tile, merging into a same-kind
// stack there up to StackLimit; any overflow spills to a nearby tile.
func (g *Game) mergeOrSet(id entity.EntityID, st item.Stack) { g.mergeOrSetFrom(id, st, nil) }

// spillStep is one tile a spilling stack was tried on, with its quantity.
type spillStep struct{ x, y, qty int }

// mergeOrSetFrom is mergeOrSet along a spill chain. Arriving again on a
// tile with the same quantity means nothing merged since the last visit,
// so the chain would repeat forever (spillTile accepts a same-kind stack
// that CanMerge refuses, e.g. two corpses of different species side by
// side): the stack is set down where it is, as when no spill tile is
// found. Chains that end on their own never revisit a tile that way, so
// they are unchanged.
func (g *Game) mergeOrSetFrom(id entity.EntityID, st item.Stack, seen []spillStep) {
	here := spillStep{st.X, st.Y, st.Qty}
	if slices.Contains(seen, here) {
		g.stacks.Set(id, st)
		return
	}
	seen = append(seen, here)
	if g.isWaterAt(st.X, st.Y) && st.Carrier == 0 {
		// a load set down in water floats to the nearest dry ground
		if x, y, ok := g.spillTile(st.Kind, st.X, st.Y); ok {
			st.X, st.Y = x, y
			g.mergeOrSetFrom(id, st, seen)
			return
		}
	}
	for _, oid := range g.itemIDs {
		if oid == id {
			continue
		}
		op := g.stacks.Ref(oid)
		if op == nil || !item.CanMerge(st, *op) {
			continue
		}
		other := *op // copied before the store changes below
		room := item.StackLimit - other.Qty
		if room <= 0 {
			break
		}
		if st.Qty <= room {
			other.Rot = item.MergeRot(other.Qty, other.Rot, st.Qty, st.Rot)
			other.Qty += st.Qty
			g.stacks.Set(oid, other)
			g.deleteItem(id)
			return
		}
		other.Rot = item.MergeRot(other.Qty, other.Rot, room, st.Rot)
		other.Qty = item.StackLimit
		g.stacks.Set(oid, other)
		st.Qty -= room
		break
	}
	if g.otherStackAt(id, st.X, st.Y) {
		if x, y, ok := g.spillTile(st.Kind, st.X, st.Y); ok {
			st.X, st.Y = x, y
			g.mergeOrSetFrom(id, st, seen)
			return
		}
	}
	g.stacks.Set(id, st)
}

func (g *Game) otherStackAt(id entity.EntityID, x, y int) bool {
	for _, oid := range g.itemIDs {
		if oid == id {
			continue
		}
		if o := g.stacks.Ref(oid); o != nil && o.Carrier == 0 && o.X == x && o.Y == y {
			return true
		}
	}
	return false
}

// spillTile is the nearest walkable tile around (x, y) with no stack or
// a same-kind stack that still has room.
func (g *Game) spillTile(k item.Kind, x, y int) (int, int, bool) {
	for r := 1; r <= 4; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(abs(dx), abs(dy)) != r {
					continue
				}
				nx, ny := x+dx, y+dy
				if !g.dryWalkable(nx, ny) {
					continue // nothing is dropped in the water
				}
				oid := g.itemAt(nx, ny)
				if oid == 0 {
					return nx, ny, true
				}
				if o, _ := g.stacks.Get(oid); o.Kind == k && o.Qty < item.StackLimit {
					return nx, ny, true
				}
			}
		}
	}
	return 0, 0, false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Ate reports whether the pawn completed an Eat job (fixture helper).
func (g *Game) Ate(id entity.EntityID) bool { return g.ateAt[id] }

// SleptInBed reports a completed or in-progress sleep on a claimed bed.
func (g *Game) SleptInBed(id entity.EntityID) bool { return g.sleptAt[id] != 0 }

// VitalOf returns life status for fixtures.
func (g *Game) VitalOf(id entity.EntityID) health.Record {
	v, _ := g.vital.Get(id)
	return v
}

// splitLoad is the take-sized load split off stack st: same kind, place,
// age and drying, and the same body (species, who, when) for corpses.
func splitLoad(st item.Stack, take int) item.Stack {
	return item.Stack{Kind: st.Kind, Qty: take, X: st.X, Y: st.Y, Rot: st.Rot, Dried: st.Dried,
		Animal: st.Animal, Of: st.Of, DiedAt: st.DiedAt, Hatch: st.Hatch, Tame: st.Tame}
}
