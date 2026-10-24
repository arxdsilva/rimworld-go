package game

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
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
	return ok && v.Status == health.Downed &&
		(v.Cause == health.CauseInjury || v.Cause == health.CauseHypothermia || v.Cause == health.CauseHeatstroke)
}

// bedUnder is the bed covering a pawn's tile that it has claimed.
func (g *Game) bedUnder(id entity.EntityID) entity.EntityID {
	pos, _ := g.positions.Get(id)
	bid := g.bedUnderIndexed(id, pos) // jobindex.go
	if selfCheck() {
		if s := g.bedUnderScan(id, pos); s != bid {
			panic(fmt.Sprintf("bedUnder(%d) index %d, scan %d", id, bid, s))
		}
	}
	return bid
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

// freeBedFor finds the bed the patient may lie in: a hospital bed if one
// is free, else the nearest bed (NG-1 §5.6).
func (g *Game) freeBedFor(patient entity.EntityID, from Position) (entity.EntityID, bool) {
	best, bestD, bestHosp := entity.EntityID(0), 1<<30, false
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || !b.Kind.Sleeps() {
			continue
		}
		if !b.CanClaim(uint64(patient)) || b.ForPrisoners != g.isPrisoner(patient) {
			continue
		}
		if owner, taken := g.reserve.EntityOwner(bid); taken && owner != patient {
			continue
		}
		hosp := b.Kind == construction.HospitalBed
		d := manhattan(from.X, from.Y, b.X, b.Y)
		if best == 0 || hosp && !bestHosp || hosp == bestHosp && (d < bestD || d == bestD && bid < best) {
			best, bestD, bestHosp = bid, d, hosp
		}
	}
	return best, best != 0
}

// assignDoctor picks the nearest patient that needs a rescue or a tend:
// downed colonists first get carried to bed; anyone lying down (downed,
// in bed or asleep) with an open wound or an infection gets tended, with
// herbal medicine when there is some. With no other doctor around, a
// colonist tends its own wounds.
func (g *Game) assignDoctor(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	type cand struct {
		rec job.Record
		d   int
	}
	var best *cand
	consider := func(pid entity.EntityID) {
		if pid == id || g.carriedBy(pid) != 0 {
			return
		}
		// prisoners are tended where they lie, never rescued to a colonist bed
		downed := g.injuredDowned(pid) && !g.isPrisoner(pid)
		if !downed && !(g.lyingForCare(pid) && g.needsTending(pid)) {
			return
		}
		if _, taken := g.reserve.EntityOwner(pid); taken {
			return
		}
		pp, _ := g.positions.Get(pid)
		var rec job.Record
		switch {
		case downed && !g.inBed(pid):
			if _, ok := g.freeBedFor(pid, pp); ok {
				rec = job.Record{Kind: job.Rescue, Target: pid}
			} else if !g.tended(pid) {
				rec = job.Record{Kind: job.Tend, Target: pid, WorkLeft: TendWork}
			}
		case !g.tended(pid):
			rec = job.Record{Kind: job.Tend, Target: pid, WorkLeft: TendWork}
		}
		if pv, _ := g.vital.Get(pid); rec.Kind == job.Tend && !g.openInjury(pid, pv) && g.wounds[pid].Infection == 0 {
			return // exposure needs a bed, not a bandage
		}
		if rec.Kind == job.None {
			return
		}
		sx, sy, ok := g.besideSpot(id, pp.X, pp.Y)
		if !ok {
			return
		}
		rec.TX, rec.TY = sx, sy
		if d := manhattan(pos.X, pos.Y, pp.X, pp.Y); best == nil || d < best.d {
			best = &cand{rec, d}
		}
	}
	for _, pid := range g.pawnIDs {
		consider(pid)
	}
	for _, pid := range g.raiderIDs {
		if g.isPrisoner(pid) {
			consider(pid)
		}
	}
	var rec job.Record
	switch {
	case best != nil:
		rec = best.rec
	case g.needsTending(id) && !g.otherDoctor(id):
		rec = job.Record{Kind: job.Tend, Target: id, TX: pos.X, TY: pos.Y, WorkLeft: TendWork * 3 / 2}
	default:
		return false
	}
	if rec.Kind == job.Tend {
		rec = g.withMedicine(id, rec)
	}
	if !g.startJob(id, rec) {
		return false
	}
	if rec.Kind == job.Tend && rec.Dest != 0 {
		g.reserve.TryEntity(id, rec.Dest)
	}
	return true
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
	if _, ok := g.bldgs.Get(j.Dest); !ok {
		g.dropPatient(id)
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	g.releaseBed(j.Target)
	b, _ := g.bldgs.Get(j.Dest)
	b = b.Claim(uint64(j.Target))
	g.bldgs.Set(j.Dest, b)
	delete(g.carrying, id)
	cx, cy := g.sleepCellFor(j.Target, b)
	g.positions.Set(j.Target, Position{X: cx, Y: cy})
	g.sleptAt[j.Target] = j.Dest
	g.logPair(j.Target, id, logRescuedBy, logRescued, 0, 0)
	g.onRescued(id, j.Target) // social.go: the patient's gratitude
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
	if !ok || v.Status == health.Dead || !g.openInjury(j.Target, v) && g.wounds[j.Target].Infection == 0 {
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	if j.Dest != 0 && j.Carry == 0 && !g.pickUpMedicine(id, &j) {
		return // walking to the patient (or the job ended)
	}
	if j.Target != id {
		pos, _ := g.positions.Get(id)
		if pp, _ := g.positions.Get(j.Target); abs(pp.X-pos.X) > 1 || abs(pp.Y-pos.Y) > 1 {
			j.State, j.Fail = job.Fail, job.TargetGone // the patient got up and left
			g.jobs.Set(id, j)
			return
		}
	}
	u := g.workProgress(id, skMedicine, true) // skills.go
	if u == 0 {
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft -= u
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	care := g.tendCare(id, j.Target, j)
	heal := g.tendHealBy(id, care) // skills.go: tier, hospital bed, Medicine skill
	if j.Carry != 0 {
		g.deleteItem(j.Carry)
		j.Carry = 0
	}
	if !g.healBody(j.Target, &v, heal, true) { // body.go: tends every open wound, then heals
		v.Injury = max(0, v.Injury-heal)
	}
	if v.Status == health.Downed && v.Cause == health.CauseInjury && v.Injury <= health.RecoverInjury && g.consciousEnough(j.Target) {
		v.Status, v.Cause = health.Alive, health.CauseNone
	}
	g.vital.Set(j.Target, v)
	g.tendedUntil[j.Target] = uint64(g.clock.Now()) + TendTicks
	g.logPair(j.Target, id, logTendedBy, logTended, 0, 0)
	g.closeWound(j.Target, care, id)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// Injure deals damage to a colonist as if from an attack (dev scenarios
// and fixtures).
func (g *Game) Injure(id entity.EntityID, dmg int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.hurt(id, dmg, 0)
}

// BuildNow places a finished building (dev scenarios and fixtures).
func (g *Game) BuildNow(kind, stuff, facing string, x, y int) bool {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	b, ok := buildableByName(kind)
	if !ok {
		return false
	}
	st, ok := stuffByName(stuff)
	if !ok {
		return false
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
	on := g.bedCoveringIndexed(x, y) // jobindex.go
	if selfCheck() {
		if s := g.bedCoveringScan(x, y); s != on {
			panic(fmt.Sprintf("bedCovering(%d,%d) index %v, scan %v", x, y, on, s))
		}
	}
	return on
}

func (g *Game) bedCoveringScan(x, y int) bool {
	for _, bid := range g.bldgIDs {
		if b, ok := g.bldgs.Get(bid); ok && b.Kind.Sleeps() && construction.Covers(b.Kind, b.Facing, b.X, b.Y, x, y) {
			return true
		}
	}
	return false
}
