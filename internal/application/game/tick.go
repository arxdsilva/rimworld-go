package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

func (g *Game) phaseCommands() {
	cmds := g.pending
	g.pending = nil
	for _, c := range cmds {
		g.applyOne(c)
	}
}

func (g *Game) applyOne(c command.Command) {
	switch c.Kind {
	case command.MovePawn:
		g.cmdMove(entity.EntityID(c.Pawn), c.X, c.Y)
	case command.DesignateStockpile:
		minX, minY, maxX, maxY := zone.Normalize(c.X, c.Y, c.X2, c.Y2)
		g.stockpiles = append(g.stockpiles, zone.Stockpile{
			MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY,
			AllowFood: true, AllowWood: true, AllowStone: true,
		})
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
			}
		}
	case command.DesignateCancel:
		g.cancelRect(c.X, c.Y, c.X2, c.Y2)
	case command.DesignateBuild:
		stuff := item.Kind(c.Stuff)
		if !stuff.IsStuff() {
			stuff = item.Wood // zero value: default material
		}
		g.placeFrameStuff(construction.Kind(c.Build), stuff, construction.ParseFacing(c.Facing), c.X, c.Y)
	case command.DesignateCut:
		g.markCut(c.X, c.Y, c.X2, c.Y2, true)
	case command.DesignateMine:
		g.markMine(c.X, c.Y, c.X2, c.Y2, true)
	case command.DesignateDeconstruct:
		g.markDeconstruct(c.X, c.Y, c.X2, c.Y2, true)
	case command.DraftPawn:
		g.setDraft(entity.EntityID(c.Pawn), c.Enabled)
	case command.Prioritize:
		g.prioritize(entity.EntityID(c.Pawn), c.Order, c.X, c.Y, construction.Layer(c.Layer))
	case command.SetWorkEnabled:
		id := entity.EntityID(c.Pawn)
		m, ok := g.labor.Get(id)
		if !ok {
			return
		}
		var t work.Type
		switch c.Work {
		case command.WorkHaul:
			t = work.Haul
		case command.WorkConstruct:
			t = work.Construct
		case command.WorkGrow:
			t = work.Grow
		case command.WorkMine:
			t = work.Mine
		case command.WorkCook:
			t = work.Cook
		case command.WorkHunt:
			t = work.Hunt
		default:
			return
		}
		g.labor.Set(id, m.Set(t, c.Enabled))
	}
}

func (g *Game) cmdMove(id entity.EntityID, x, y int) {
	if !g.alive(id) {
		return
	}
	g.interrupt(id, job.Cancelled)
	rec := job.Record{Kind: job.Move, TX: x, TY: y}
	g.startJob(id, rec)
}

func (g *Game) cancelRect(x1, y1, x2, y2 int) {
	minX, minY, maxX, maxY := zone.Normalize(x1, y1, x2, y2)
	keptFrames := g.frameIDs[:0]
	for _, id := range g.frameIDs {
		fr, ok := g.frames.Get(id)
		if !ok {
			continue
		}
		if fr.X >= minX && fr.X <= maxX && fr.Y >= minY && fr.Y <= maxY {
			g.frames.Delete(id)
			continue
		}
		keptFrames = append(keptFrames, id)
	}
	g.frameIDs = keptFrames
	g.markDeconstruct(x1, y1, x2, y2, false)
	g.markCut(x1, y1, x2, y2, false)
	g.markMine(x1, y1, x2, y2, false)

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

	keptZ := g.stockpiles[:0]
	for _, s := range g.stockpiles {
		if s.MaxX < minX || s.MinX > maxX || s.MaxY < minY || s.MinY > maxY {
			keptZ = append(keptZ, s)
		}
	}
	g.stockpiles = keptZ
}

func (g *Game) phaseNeeds() {
	now := uint64(g.clock.Now())
	for _, id := range g.pawnIDs {
		v, _ := g.vital.Get(id)
		if v.Status == health.Dead {
			continue
		}
		if v.Status == health.Downed {
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
			n = needs.Decay(n, now)
			g.need.Set(id, n)
		}
		if n.Starving() {
			g.down(id, health.CauseNoFood)
			continue
		}
		if n.Exhausted() {
			g.down(id, health.CauseNoBed)
		}
	}
}

func (g *Game) down(id entity.EntityID, cause health.Cause) {
	g.interrupt(id, job.Cancelled)
	g.vital.Set(id, health.Record{
		Status:     health.Downed,
		Cause:      cause,
		DownedTick: uint64(g.clock.Now()),
	})
}

func (g *Game) releaseBed(id entity.EntityID) {
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || entity.EntityID(b.Claimant) != id {
			continue
		}
		b.Claimant = 0
		g.bldgs.Set(bid, b)
	}
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
	g.reserve.ReleaseOwner(id)
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
	act := g.Activity()
	for _, id := range g.pawnIDs {
		v, _ := g.vital.Get(id)
		if v.Status == health.Dead {
			g.reserve.ReleaseOwner(id)
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
			g.dropCarry(id)
			g.reserve.ReleaseOwner(id)
			g.jobs.Set(id, job.Record{})
			j = job.Record{}
		}
		if g.drafted(id) {
			continue // drafted pawns only follow direct orders
		}
		n, _ := g.need.Get(id)

		busy := j.Kind != job.None && j.State != job.Idle
		if busy && j.Forced && !n.Exhausted() {
			continue // player-prioritized: finish it first
		}
		if busy {
			if n.Hungry() && j.Kind != job.Eat && g.hasEdible() {
				g.interrupt(id, job.Interrupted)
				g.assignEat(id)
				continue
			}
			if (n.Exhausted() || n.Tired()) && j.Kind != job.Sleep && j.Kind != job.Eat {
				g.interrupt(id, job.Interrupted)
				g.assignSleep(id)
				continue
			}
			continue
		}

		if n.Hungry() && g.hasEdible() {
			g.assignEat(id)
			continue
		}
		if n.Exhausted() || n.Tired() || needs.SleepScheduled(act) {
			g.assignSleep(id)
			continue
		}
		if !needs.WorkScheduled(act) {
			continue
		}
		m, _ := g.labor.Get(id)
		if m.Enabled(work.Haul) && g.assignHaul(id) {
			continue
		}
		if m.Enabled(work.Construct) && g.assignConstruct(id) {
			continue
		}
		if m.Enabled(work.Grow) && (g.assignHarvest(id) || g.assignCut(id)) {
			continue
		}
		if m.Enabled(work.Mine) && g.assignMine(id) {
			continue
		}
	}
}

func (g *Game) startJob(id entity.EntityID, rec job.Record) bool {
	g.reserve.ReleaseOwner(id)
	if rec.Target != 0 && !g.reserve.TryEntity(id, rec.Target) {
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
	if rec.Kind == job.Haul && !g.reserve.TryTile(id, rec.DestX, rec.DestY) {
		g.reserve.ReleaseOwner(id)
		rec.State = job.Fail
		rec.Fail = job.Reserved
		g.jobs.Set(id, rec)
		return false
	}
	path, ok := g.pathTo(id, rec.TX, rec.TY)
	if !ok {
		g.reserve.ReleaseOwner(id)
		rec.State = job.Fail
		rec.Fail = job.Unreachable
		g.jobs.Set(id, rec)
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
	pos, _ := g.positions.Get(id)
	if pos.X == tx && pos.Y == ty {
		return nil, true
	}
	if g.grid.Walkable(tx, ty) {
		return pathing.Find(g.grid, pos.X, pos.Y, tx, ty)
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
			if !g.grid.Walkable(ax, ay) {
				continue
			}
			p, ok := pathing.Find(g.grid, pos.X, pos.Y, ax, ay)
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
		if !ok || st.Kind != item.Food || st.Qty <= 0 || st.Carrier != 0 {
			continue
		}
		if _, taken := g.reserve.EntityOwner(id); taken {
			continue
		}
		return true
	}
	return false
}

func (g *Game) assignEat(id entity.EntityID) {
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	var bx, by int
	for _, iid := range g.itemIDs {
		st, ok := g.stacks.Get(iid)
		if !ok || st.Kind != item.Food || st.Qty <= 0 || st.Carrier != 0 {
			continue
		}
		if _, taken := g.reserve.EntityOwner(iid); taken {
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, st.X, st.Y)
		if d < bestD || (d == bestD && iid < best) {
			best, bestD, bx, by = iid, d, st.X, st.Y
		}
	}
	if best == 0 {
		return
	}
	g.startJob(id, job.Record{
		Kind: job.Eat, Target: best, TX: bx, TY: by, WorkLeft: needs.EatWorkTicks,
	})
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
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	var bx, by int
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || b.Kind != construction.Bed {
			continue
		}
		if b.Claimant != 0 && entity.EntityID(b.Claimant) != id {
			continue
		}
		if owner, taken := g.reserve.EntityOwner(bid); taken && owner != id {
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, b.X, b.Y)
		if d < bestD || (d == bestD && bid < best) {
			best, bestD, bx, by = bid, d, b.X, b.Y
		}
	}
	if best != 0 {
		b, _ := g.bldgs.Get(best)
		b.Claimant = uint64(id)
		g.bldgs.Set(best, b)
		g.sleptAt[id] = best
		if !g.startJob(id, job.Record{Kind: job.Sleep, Target: best, TX: bx, TY: by, WorkLeft: needs.SleepWorkMin}) {
			b.Claimant = 0
			g.bldgs.Set(best, b)
			g.sleptAt[id] = 0
		}
		return
	}
	g.sleptAt[id] = 0
	g.startJob(id, job.Record{Kind: job.Sleep, TX: pos.X, TY: pos.Y, WorkLeft: needs.SleepWorkMin})
}

func (g *Game) assignHaul(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	if rec, ok := g.haulToFrame(id, pos); ok {
		return g.startJob(id, rec)
	}
	return g.haulToStockpile(id, pos)
}

func (g *Game) haulToFrame(id entity.EntityID, pos Position) (job.Record, bool) {
	var bestWood, bestFrame entity.EntityID
	bestD := 1 << 30
	var wx, wy, fx, fy int
	for _, fid := range g.frameIDs {
		fr, ok := g.frames.Get(fid)
		if !ok || fr.Ready() {
			continue
		}
		if _, taken := g.reserve.EntityOwner(fid); taken {
			continue
		}
		for _, iid := range g.itemIDs {
			st, ok := g.stacks.Get(iid)
			if !ok || st.Kind != fr.Stuff || st.Qty <= 0 || st.Carrier != 0 {
				continue
			}
			if _, taken := g.reserve.EntityOwner(iid); taken {
				continue
			}
			d := pathing.Dist(pos.X, pos.Y, st.X, st.Y)
			if d < bestD || (d == bestD && iid < bestWood) {
				bestD, bestWood, bestFrame = d, iid, fid
				wx, wy, fx, fy = st.X, st.Y, fr.X, fr.Y
			}
		}
	}
	if bestWood == 0 {
		return job.Record{}, false
	}
	return job.Record{
		Kind: job.Haul, Target: bestWood, TX: wx, TY: wy, DestX: fx, DestY: fy, Dest: bestFrame,
	}, true
}

func (g *Game) haulToStockpile(id entity.EntityID, pos Position) bool {
	var best item.Stack
	var bestID entity.EntityID
	bestD := 1 << 30
	found := false
	for _, iid := range g.itemIDs {
		st, ok := g.stacks.Get(iid)
		if !ok || st.Qty <= 0 || st.Carrier != 0 {
			continue
		}
		if _, taken := g.reserve.EntityOwner(iid); taken {
			continue
		}
		if g.inMatchingStockpile(st) {
			continue
		}
		if !g.filterMatches(st.Kind) {
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, st.X, st.Y)
		if !found || d < bestD || (d == bestD && iid < bestID) {
			found, bestD, bestID, best = true, d, iid, st
		}
	}
	if !found {
		return false
	}
	dx, dy, ok := g.emptyStockpileCell(best.Kind)
	if !ok {
		return false
	}
	return g.startJob(id, job.Record{
		Kind: job.Haul, Target: bestID, TX: best.X, TY: best.Y, DestX: dx, DestY: dy,
	})
}

func (g *Game) filterMatches(k item.Kind) bool {
	for _, s := range g.stockpiles {
		if allows(s, k) {
			return true
		}
	}
	return false
}

func allows(s zone.Stockpile, k item.Kind) bool {
	switch {
	case k.IsFood():
		return s.AllowFood
	case k == item.Wood:
		return s.AllowWood
	case k == item.Stone:
		return s.AllowStone
	}
	return false
}

func (g *Game) inMatchingStockpile(st item.Stack) bool {
	for _, s := range g.stockpiles {
		if !s.Contains(st.X, st.Y) {
			continue
		}
		if allows(s, st.Kind) {
			return true
		}
	}
	return false
}

func (g *Game) emptyStockpileCell(k item.Kind) (int, int, bool) {
	for _, s := range g.stockpiles {
		if !allows(s, k) {
			continue
		}
		for y := s.MinY; y <= s.MaxY; y++ {
			for x := s.MinX; x <= s.MaxX; x++ {
				if !g.grid.Walkable(x, y) {
					continue
				}
				if g.itemAt(x, y) != 0 || g.layerFrameAt(x, y, construction.LayerObject) != 0 ||
					g.layerBldgAt(x, y, construction.LayerObject) != 0 {
					continue
				}
				if _, taken := g.reserve.TileOwner(x, y); taken {
					continue
				}
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

func (g *Game) assignConstruct(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	var bx, by int
	for _, fid := range g.frameIDs {
		fr, ok := g.frames.Get(fid)
		if !ok || !fr.Ready() {
			continue
		}
		if _, taken := g.reserve.EntityOwner(fid); taken {
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, fr.X, fr.Y)
		if d < bestD || (d == bestD && fid < best) {
			best, bestD, bx, by = fid, d, fr.X, fr.Y
		}
	}
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || !b.Deconstruct {
			continue
		}
		if _, taken := g.reserve.EntityOwner(bid); taken {
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, b.X, b.Y)
		if d < bestD || (d == bestD && bid < best) {
			best, bestD, bx, by = bid, d, b.X, b.Y
		}
	}
	if best == 0 {
		return false
	}
	if b, ok := g.bldgs.Get(best); ok {
		tx, ty := g.workSpot(id, bx, by)
		return g.startJob(id, job.Record{
			Kind: job.Deconstruct, Target: best, TX: tx, TY: ty, WorkLeft: b.WorkLeft,
		})
	}
	fr, _ := g.frames.Get(best)
	return g.startJob(id, job.Record{
		Kind: job.Construct, Target: best, TX: bx, TY: by, WorkLeft: fr.WorkLeft,
	})
}

func (g *Game) assignHarvest(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	var bx, by int
	for _, pid := range g.plantIDs {
		p, ok := g.plants.Get(pid)
		if !ok || !p.Harvestable() || !p.Designated {
			continue
		}
		if _, taken := g.reserve.EntityOwner(pid); taken {
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, p.X, p.Y)
		if d < bestD || (d == bestD && pid < best) {
			best, bestD, bx, by = pid, d, p.X, p.Y
		}
	}
	if best == 0 {
		return false
	}
	bp, _ := g.plants.Get(best)
	return g.startJob(id, job.Record{
		Kind: job.Harvest, Target: best, TX: bx, TY: by, WorkLeft: plant.DefOf(bp.Species).HarvestWork,
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
		if j.State != job.Do {
			continue
		}
		if v.Status == health.Downed && j.Kind != job.Sleep {
			continue
		}
		switch j.Kind {
		case job.Eat:
			g.doEat(id, j)
		case job.Sleep:
			g.doSleep(id, j, now)
		case job.Haul:
			g.doHaul(id, j)
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
		default:
			j.State = job.Complete
			g.jobs.Set(id, j)
		}
	}
}

func (g *Game) doEat(id entity.EntityID, j job.Record) {
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
	st.Qty--
	if st.Qty <= 0 {
		g.deleteItem(j.Target)
	} else {
		g.stacks.Set(j.Target, st)
	}
	n, _ := g.need.Get(id)
	n.Hunger += needs.EatRestore
	if n.Hunger > needs.Full {
		n.Hunger = needs.Full
	}
	g.need.Set(id, n)
	g.ateAt[id] = true
	j.State = job.Complete
	g.jobs.Set(id, j)
}

func (g *Game) doSleep(id entity.EntityID, j job.Record, now uint64) {
	n, _ := g.need.Get(id)
	n = needs.SleepTick(n, now)
	g.need.Set(id, n)
	if v, ok := g.vital.Get(id); ok && v.Status == health.Downed && v.Cause == health.CauseNoBed && n.Rest >= needs.CriticalRest {
		v.Status = health.Alive
		v.Cause = health.CauseNone
		g.vital.Set(id, v)
	}
	j.WorkLeft--
	act := g.Activity()
	done := n.Rest >= needs.Full
	if !n.Exhausted() && needs.WorkScheduled(act) && n.Rest > needs.CriticalRest && j.WorkLeft <= 0 {
		done = true
	}
	if n.Hungry() && g.hasEdible() {
		done = true
	}
	if done {
		j.State = job.Complete
	}
	g.jobs.Set(id, j)
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
		st.Carrier = id
		g.stacks.Set(j.Target, st)
		j.Carry = j.Target
		path, ok := g.pathTo(id, j.DestX, j.DestY)
		if !ok {
			st.Carrier = 0
			st.X, st.Y = pos.X, pos.Y
			g.stacks.Set(j.Target, st)
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
	fr.WorkLeft--
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
	if pid := g.plantAt(fr.X, fr.Y); pid != 0 {
		g.removePlant(pid) // bushes and crops are cleared by building over them
	}
	if construction.DefOf(fr.Kind).BlocksMove {
		g.grid.SetWalkable(fr.X, fr.Y, false)
		g.ejectPawns(fr.X, fr.Y)
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
	b.WorkLeft--
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
	j.WorkLeft--
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	d := plant.DefOf(p.Species)
	g.dropStack(item.Food, d.Yield, p.X, p.Y)
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

func (g *Game) phaseMovement() {
	type move struct {
		id  entity.EntityID
		pos Position
	}
	var moves []move
	for _, id := range g.pawnIDs {
		if !g.alive(id) || !g.dueToMove(id) {
			continue
		}
		j, _ := g.jobs.Get(id)
		if j.State != job.Goto || j.PathI >= len(j.Path) {
			continue
		}
		next := j.Path[j.PathI]
		if !g.grid.Walkable(next.X, next.Y) {
			path, ok := g.pathTo(id, j.TX, j.TY)
			if !ok {
				j.State = job.Fail
				j.Fail = job.Unreachable
				g.jobs.Set(id, j)
				continue
			}
			j.Path, j.PathI = path, 0
			g.jobs.Set(id, j)
			if len(path) == 0 {
				g.arrive(id, j)
			}
			continue
		}
		moves = append(moves, move{id: id, pos: Position{X: next.X, Y: next.Y}})
		j.PathI++
		if j.PathI >= len(j.Path) {
			if j.Kind == job.Move {
				j.State = job.Complete
			} else {
				j.State = job.Do
			}
		}
		g.jobs.Set(id, j)
	}
	for _, m := range moves {
		g.positions.Set(m.id, m.pos)
		if j, ok := g.jobs.Get(m.id); ok && j.Carry != 0 {
			if st, ok := g.stacks.Get(j.Carry); ok {
				st.X, st.Y = m.pos.X, m.pos.Y
				g.stacks.Set(j.Carry, st)
			}
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
		st, ok := g.stacks.Get(id)
		if ok && st.Carrier == 0 && st.X == x && st.Y == y {
			return id
		}
	}
	return 0
}

func (g *Game) deleteItem(id entity.EntityID) {
	g.stacks.Delete(id)
	kept := g.itemIDs[:0]
	for _, iid := range g.itemIDs {
		if iid != id {
			kept = append(kept, iid)
		}
	}
	g.itemIDs = kept
}

func (g *Game) mergeOrSet(id entity.EntityID, st item.Stack) {
	for _, oid := range g.itemIDs {
		if oid == id {
			continue
		}
		other, ok := g.stacks.Get(oid)
		if !ok {
			continue
		}
		if item.CanMerge(st, other) {
			other.Qty = item.Add(other.Qty, st.Qty)
			g.stacks.Set(oid, other)
			g.deleteItem(id)
			return
		}
	}
	g.stacks.Set(id, st)
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
