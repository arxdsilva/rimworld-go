package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Medical care (Doctor work): carry injured, downed colonists to a bed
// (they heal faster there) and tend their wounds (an immediate injury
// reduction, then faster healing for a day).

const (
	// TendWork is the ticks a doctor spends tending one patient.
	TendWork = 300
	// TendHeal is injury removed immediately by tending.
	TendHeal = 8
	// TendTicks is how long a tend keeps speeding up healing.
	TendTicks = clock.TicksPerDay
)

// injuredDowned: a colonist lying down from injury.
func (g *Game) injuredDowned(id entity.EntityID) bool {
	v, ok := g.vital.Get(id)
	return ok && v.Status == health.Downed && v.Cause == health.CauseInjury
}

// bedUnder is the bed covering a pawn's tile that it has claimed.
func (g *Game) bedUnder(id entity.EntityID) entity.EntityID {
	pos, _ := g.positions.Get(id)
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || !b.Kind.Sleeps() || entity.EntityID(b.Claimant) != id {
			continue
		}
		if construction.Covers(b.Kind, b.Facing, b.X, b.Y, pos.X, pos.Y) {
			return bid
		}
	}
	return 0
}

func (g *Game) inBed(id entity.EntityID) bool { return g.bedUnder(id) != 0 }

func (g *Game) tended(id entity.EntityID) bool {
	return uint64(g.clock.Now()) < g.tendedUntil[id]
}

// carriedBy is the rescuer carrying id, if any.
func (g *Game) carriedBy(id entity.EntityID) entity.EntityID {
	for c, p := range g.carrying {
		if p == id {
			return c
		}
	}
	return 0
}

// freeBedFor finds the nearest bed the patient may lie in.
func (g *Game) freeBedFor(patient entity.EntityID, from Position) (entity.EntityID, bool) {
	best, bestD := entity.EntityID(0), 1<<30
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || !b.Kind.Sleeps() {
			continue
		}
		if b.Claimant != 0 && entity.EntityID(b.Claimant) != patient {
			continue
		}
		if owner, taken := g.reserve.EntityOwner(bid); taken && owner != patient {
			continue
		}
		if d := manhattan(from.X, from.Y, b.X, b.Y); d < bestD || (d == bestD && bid < best) {
			best, bestD = bid, d
		}
	}
	return best, best != 0
}

// assignDoctor picks the nearest patient that needs a rescue or a tend.
func (g *Game) assignDoctor(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	type cand struct {
		rec job.Record
		d   int
	}
	var best *cand
	for _, pid := range g.pawnIDs {
		if pid == id || !g.injuredDowned(pid) || g.carriedBy(pid) != 0 {
			continue
		}
		if _, taken := g.reserve.EntityOwner(pid); taken {
			continue
		}
		pp, _ := g.positions.Get(pid)
		var rec job.Record
		switch {
		case !g.inBed(pid):
			if _, ok := g.freeBedFor(pid, pp); ok {
				rec = job.Record{Kind: job.Rescue, Target: pid}
			} else if !g.tended(pid) {
				rec = job.Record{Kind: job.Tend, Target: pid, WorkLeft: TendWork}
			}
		case !g.tended(pid):
			rec = job.Record{Kind: job.Tend, Target: pid, WorkLeft: TendWork}
		}
		if rec.Kind != job.None {
			sx, sy, ok := g.besideSpot(id, pp.X, pp.Y)
			if !ok {
				continue
			}
			rec.TX, rec.TY = sx, sy
		}
		if rec.Kind == job.None {
			continue
		}
		if d := manhattan(pos.X, pos.Y, pp.X, pp.Y); best == nil || d < best.d {
			best = &cand{rec, d}
		}
	}
	if best == nil {
		return false
	}
	return g.startJob(id, best.rec)
}

// doRescue: on reaching the patient pick them up and head for a bed; on
// reaching the bed lay them in it.
func (g *Game) doRescue(id entity.EntityID, j job.Record) {
	if !g.injuredDowned(j.Target) {
		g.dropPatient(id)
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	pos, _ := g.positions.Get(id)
	if j.Dest == 0 {
		bid, ok := g.freeBedFor(j.Target, pos)
		if !ok || !g.reserve.TryEntity(id, bid) {
			j.State, j.Fail = job.Fail, job.Reserved
			g.jobs.Set(id, j)
			return
		}
		b, _ := g.bldgs.Get(bid)
		sx, sy, ok := g.besideSpot(id, b.X, b.Y)
		var path []pathing.Point
		if ok {
			path, ok = g.pathTo(id, sx, sy)
		}
		if !ok {
			j.State, j.Fail = job.Fail, job.Unreachable
			g.jobs.Set(id, j)
			return
		}
		g.carrying[id] = j.Target
		j.Dest, j.TX, j.TY, j.Path, j.PathI = bid, sx, sy, path, 0
		j.State = job.Goto
		if len(path) == 0 {
			j.State = job.Do
		}
		g.jobs.Set(id, j)
		if j.State == job.Goto {
			return
		}
	}
	b, ok := g.bldgs.Get(j.Dest)
	if !ok {
		g.dropPatient(id)
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	b.Claimant = uint64(j.Target)
	g.bldgs.Set(j.Dest, b)
	delete(g.carrying, id)
	g.positions.Set(j.Target, Position{X: b.X, Y: b.Y})
	g.sleptAt[j.Target] = j.Dest
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// dropPatient puts a carried patient down where the rescuer stands.
func (g *Game) dropPatient(id entity.EntityID) {
	p, ok := g.carrying[id]
	if !ok {
		return
	}
	delete(g.carrying, id)
	pos, _ := g.positions.Get(id)
	g.positions.Set(p, pos)
}

// followCarriers keeps carried patients on their rescuer's tile.
func (g *Game) followCarriers() {
	for c, p := range g.carrying {
		if pos, ok := g.positions.Get(c); ok {
			g.positions.Set(p, pos)
		}
	}
}

func (g *Game) doTend(id entity.EntityID, j job.Record) {
	v, ok := g.vital.Get(j.Target)
	if !ok || v.Status == health.Dead || v.Injury <= 0 {
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	if !g.workTick(id) {
		return
	}
	j.WorkLeft--
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	v.Injury -= TendHeal
	if v.Injury < 0 {
		v.Injury = 0
	}
	if v.Status == health.Downed && v.Cause == health.CauseInjury && v.Injury <= health.RecoverInjury {
		v.Status, v.Cause = health.Alive, health.CauseNone
	}
	g.vital.Set(j.Target, v)
	g.tendedUntil[j.Target] = uint64(g.clock.Now()) + TendTicks
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// Injure deals damage to a colonist as if from an attack (dev scenarios
// and fixtures).
func (g *Game) Injure(id entity.EntityID, dmg int) { g.hurt(id, dmg, 0) }

// BuildNow places a finished building (dev scenarios and fixtures).
func (g *Game) BuildNow(kind, stuff, facing string, x, y int) bool {
	b, ok := buildableByName(kind)
	if !ok {
		return false
	}
	st := item.Wood
	if stuff == "stone" {
		st = item.Stone
	}
	f := parseFacingName(facing)
	if ok, _ := g.canPlaceAt(b, f, x, y); !ok {
		return false
	}
	g.placeFrameStuff(b, st, f, x, y)
	if b.Instant() {
		return g.layerBldgAt(x, y, construction.DefOf(b).Layer) != 0
	}
	fid := g.layerFrameAt(x, y, construction.DefOf(b).Layer)
	fr, ok := g.frames.Get(fid)
	if !ok {
		return false
	}
	g.completeFrame(fid, fr)
	return true
}

func buildableByName(name string) (construction.Kind, bool) {
	for _, b := range Buildables() {
		if b.Kind == name {
			return construction.Kind(b.ID), true
		}
	}
	return 0, false
}

// besideSpot is the reachable walkable tile next to (x, y), nearest to
// pawn id, that is not under a bed (doctors stand beside patients).
func (g *Game) besideSpot(id entity.EntityID, x, y int) (int, int, bool) {
	pos, _ := g.positions.Get(id)
	best, bx, by := 1<<30, 0, 0
	for _, d := range [][2]int{{0, 1}, {1, 0}, {-1, 0}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
		nx, ny := x+d[0], y+d[1]
		if !g.grid.Walkable(nx, ny) || g.bedCovering(nx, ny) {
			continue
		}
		dd := manhattan(pos.X, pos.Y, nx, ny)
		if dd < best && g.reachable(id, nx, ny) {
			best, bx, by = dd, nx, ny
		}
	}
	if best == 1<<30 {
		return x, y, g.reachable(id, x, y)
	}
	return bx, by, true
}

func (g *Game) bedCovering(x, y int) bool {
	for _, bid := range g.bldgIDs {
		if b, ok := g.bldgs.Get(bid); ok && b.Kind.Sleeps() && construction.Covers(b.Kind, b.Facing, b.X, b.Y, x, y) {
			return true
		}
	}
	return false
}
