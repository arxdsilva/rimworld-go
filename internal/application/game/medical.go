package game

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Medical care (Doctor work): carry injured, downed colonists to a bed
// (they heal faster there) and tend their wounds. A tend has a quality
// (body.TendQuality, MED-02): it stops the bleeding, speeds the healing of
// every wound it tends and slows an infection for its window
// (infection.go). Animals have no wounds yet: a tend mends some of their
// injury at once (animalTendHeal).

const (
	// TendWork is the ticks a doctor spends tending one patient.
	TendWork = 300
	// TendHeal is the injury a plain tend mends on an animal at once.
	TendHeal = 8
	// TendTicks is how long a tend shows as tended (PawnView.Tended) and
	// keeps an animal from being tended again.
	TendTicks = clock.TicksPerDay
)

// animalTendHeal is the injury a tend of quality q (permille) mends on an
// animal at once: TendHeal × (250 + q) / 500, so about 9 for a plain tend
// by a level-8 doctor, 14 with herbal and 20 with industrial medicine
// (original tuning near the old care tiers, until animals get wounds in
// MED-09).
func animalTendHeal(q int32) int { return TendHeal * int(250+q) / 500 }

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

// freeBedFor finds the bed the patient may lie in: a free medical bed
// first (PLAY-020), then a hospital bed (NG-1 §5.6), each the nearest.
func (g *Game) freeBedFor(patient entity.EntityID, from Position) (entity.EntityID, bool) {
	best, bestD, bestRank := entity.EntityID(0), 1<<30, 0
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
		rank := 0
		if g.isMedicalBed(b) {
			rank += 2
		}
		if b.Kind == construction.HospitalBed {
			rank++
		}
		d := manhattan(from.X, from.Y, b.X, b.Y)
		if best == 0 || rank > bestRank || rank == bestRank && (d < bestD || d == bestD && bid < best) {
			best, bestD, bestRank = bid, d, rank
		}
	}
	return best, best != 0
}

// assignDoctor is the humanlike half of Doctor work (MED-03), each step
// its own giver: rescue a downed patient to bed, then tend the bleeding,
// then everyone else (or itself, with no one else to help). Surgery and
// animals follow in assignWorkType.
func (g *Game) assignDoctor(id entity.EntityID) bool {
	return g.assignRescue(id) || g.assignTendBleeding(id) || g.assignTendOther(id)
}

// patientRank is a doctor's candidate: the job it would start, how fast
// the patient bleeds, how hurt it is and how far away.
type patientRank struct {
	rec       job.Record
	pid       entity.EntityID
	bleed     int32
	injury, d int
}

// outranks: c comes before best: bleeding fastest, then worst hurt,
// then nearest, then lowest id (a deterministic tie-break).
func (c *patientRank) outranks(best *patientRank) bool {
	switch {
	case best == nil:
		return true
	case c.bleed != best.bleed:
		return c.bleed > best.bleed
	case c.injury != best.injury:
		return c.injury > best.injury
	case c.d != best.d:
		return c.d < best.d
	}
	return c.pid < best.pid
}

// bestPatient ranks every humanlike patient doctor id could look after
// (colonists, then prisoners; never one being carried or held by someone
// else) for which want gives a job, and returns the first by outranks,
// with the job's spot beside the patient filled in.
func (g *Game) bestPatient(id entity.EntityID, want func(pid entity.EntityID) (job.Record, bool)) *patientRank {
	pos, _ := g.positions.Get(id)
	var best *patientRank
	consider := func(pid entity.EntityID) {
		if pid == id || g.carriedBy(pid) != 0 {
			return
		}
		if _, taken := g.reserve.EntityOwner(pid); taken {
			return
		}
		rec, ok := want(pid)
		if !ok {
			return
		}
		pp, _ := g.positions.Get(pid)
		sx, sy, ok := g.besideSpot(id, pp.X, pp.Y)
		if !ok {
			return
		}
		rec.TX, rec.TY = sx, sy
		pv, _ := g.vital.Get(pid)
		c := &patientRank{rec: rec, pid: pid, injury: pv.Injury, d: manhattan(pos.X, pos.Y, pp.X, pp.Y)}
		if b := g.bodies[pid]; b != nil {
			c.bleed = b.BleedPerDay() // body/bleed.go
		}
		if c.outranks(best) {
			best = c
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
	return best
}

// rescuable: pid is a downed colonist (prisoners are tended where they
// lie) out of bed with a free bed to be carried to. Its care level does
// not matter: even "No care" is rescued.
func (g *Game) rescuable(pid entity.EntityID) bool {
	if !g.injuredDowned(pid) || g.isPrisoner(pid) || g.inBed(pid) {
		return false
	}
	pp, _ := g.positions.Get(pid)
	_, ok := g.freeBedFor(pid, pp)
	return ok
}

// tendable: a doctor would tend pid where it lies (down, in bed or
// asleep) with something to tend, its care level allowing care, and not
// waiting to be rescued to a bed first. bleeding picks the patients
// losing blood (true) or the rest (false).
func (g *Game) tendable(pid entity.EntityID, bleeding bool) bool {
	if !g.lyingForCare(pid) || !g.needsTending(pid) || !g.careOf(pid).Tends() || g.rescuable(pid) {
		return false
	}
	b := g.bodies[pid]
	return (b != nil && b.BleedPerDay() > 0) == bleeding
}

// assignRescue carries the most urgent downed colonist to a bed.
func (g *Game) assignRescue(id entity.EntityID) bool {
	best := g.bestPatient(id, func(pid entity.EntityID) (job.Record, bool) {
		return job.Record{Kind: job.Rescue, Target: pid}, g.rescuable(pid)
	})
	return best != nil && g.startJob(id, best.rec)
}

// assignTendBleeding tends the patient bleeding fastest (MED-01).
func (g *Game) assignTendBleeding(id entity.EntityID) bool {
	return g.assignTendWhere(id, true)
}

// assignTendOther tends the worst hurt patient that is not bleeding;
// with no one else to tend and no other doctor around, a colonist tends
// its own wounds (unless its self-tend is off).
func (g *Game) assignTendOther(id entity.EntityID) bool {
	if g.assignTendWhere(id, false) {
		return true
	}
	if !g.needsTending(id) || g.otherDoctor(id) || !g.selfTendAllowed(id) {
		return false
	}
	pos, _ := g.positions.Get(id)
	return g.startTend(id, job.Record{Kind: job.Tend, Target: id, TX: pos.X, TY: pos.Y, WorkLeft: TendWork * 3 / 2})
}

// assignTendWhere tends the best ranked tendable patient (bleeding or
// not). The patient stays reserved until the tend ends, so a doctor never
// leaves one for another halfway (no flapping).
func (g *Game) assignTendWhere(id entity.EntityID, bleeding bool) bool {
	best := g.bestPatient(id, func(pid entity.EntityID) (job.Record, bool) {
		return job.Record{Kind: job.Tend, Target: pid, WorkLeft: TendWork}, g.tendable(pid, bleeding)
	})
	return best != nil && g.startTend(id, best.rec)
}

// startTend starts tend rec with a fetch of the medicine the patient's
// care level allows (withMedicine), holding the dose for the trip.
func (g *Game) startTend(id entity.EntityID, rec job.Record) bool {
	rec = g.withMedicine(id, rec)
	if !g.startJob(id, rec) {
		return false
	}
	if rec.Dest != 0 {
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
	if dest, _ := g.bldgs.Get(j.Dest); g.isMedicalBed(dest) {
		g.holdRestBed(j.Target, j.Dest) // bedrest.go: held for the stay; its own bed stays its own
	} else {
		g.releaseBed(j.Target)
	}
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

// doTend: fetch the dose if the job carries one, work beside the patient,
// then tend it (body.TendAll) at a quality rolled from the doctor, the
// medicine and the bed (tendQualityFor, infection.go). A patient with
// nothing left to tend (another doctor got there first) ends the job.
func (g *Game) doTend(id entity.EntityID, j job.Record) {
	v, ok := g.vital.Get(j.Target)
	if !ok || v.Status == health.Dead || !g.needsTending(j.Target) {
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
	now := uint64(g.clock.Now())
	q := g.tendQualityFor(id, j.Target, g.tendMedicine(j)) // infection.go (MED-02)
	if j.Carry != 0 {
		g.deleteItem(j.Carry)
		j.Carry = 0
	}
	if b := g.ensureBody(j.Target, &v); b != nil { // body.go: an older save's Injury becomes wounds first
		b.TendAll(q, now)
		g.bodyChanged(j.Target, b, &v)
	}
	if v.Status == health.Downed && v.Cause == health.CauseInjury && g.standsUp(j.Target, v) { // bleed.go
		v.Status, v.Cause = health.Alive, health.CauseNone
	}
	g.vital.Set(j.Target, v)
	g.tendedUntil[j.Target] = now + TendTicks
	g.logPair(j.Target, id, logTendedBy, logTended, 0, 0)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// Injure deals damage to a colonist as if from an attack (dev scenarios
// and fixtures).
func (g *Game) Injure(id entity.EntityID, dmg int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.hurt(id, dmg, 0)
}

// InjureCut deals dmg to id as a cut (dev scenarios and fixtures): unlike
// Injure's bruise it bleeds until tended (MED-01).
func (g *Game) InjureCut(id entity.EntityID, dmg int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.hurtAs(id, dmg, 0, body.Cut, health.CauseInjury)
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
