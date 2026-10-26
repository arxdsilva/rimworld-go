package game

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Bed rest and medical beds (PLAY-020).
//
// A colonist in pain, with a wound or an infection no one has tended,
// sick, or waiting for an operation goes to bed (patientRest) and stays
// there until it is well again (restOver): at most restWellInjury of
// injury that can still heal, nothing left to tend, not sick, and never
// before restMin. The two lines are far apart (pain at painInjury, well at
// restWellInjury), so a patient never flaps between bed and work. Only
// injury that can still heal counts: scars and lost parts never keep
// anyone in bed.
//
// A resting colonist gets up only for what phaseAI puts first (fire,
// flight, mental breaks, social duty, warmth, drafting, the player's
// orders) and to eat, and lies down again after. With no other colonist
// who can doctor, it tends itself where it lies. It takes the nearest
// free medical bed, held only for the stay, else its own bed, else the
// nearest free bed it may claim; with none free it carries on (or, if a
// doctor or a surgeon is to come, lies down where it is) and looks again
// after restRetry. The player can turn "Rest when hurt" off per
// colonist (SetPawnBedRest).
//
// A medical bed (Building.Medical, SetBedMedical; hospital beds start
// medical) belongs to no one: healthy colonists never sleep in it, own it,
// or are ordered into it.

const (
	// restWellInjury: rest may end once healing injury is at most this.
	restWellInjury = 15
	// restMin is the shortest bed rest.
	restMin = 2 * clock.TicksPerHour
	// restRetry: a patient who found no free bed carries on and looks
	// for one again after this long.
	restRetry = 6 * clock.TicksPerHour
	// selfTendSettle is how long a patient with no one else to doctor
	// lies in bed before it tends itself (a failed try waits as long).
	selfTendSettle = 250
	// restingLabel is a resting colonist's job line.
	restingLabel = "Resting (injured)"
	// ReasonMedicalBed: a healthy colonist can't be ordered into a
	// medical bed.
	ReasonMedicalBed = "for the sick and injured"
)

// bedRestState is every colonist's bed rest: who turned it off, who rests
// since when, who found no bed until when, and the medical bed each
// patient holds only for its stay. Maps are made on first use.
type bedRestState struct {
	off   map[entity.EntityID]bool
	since map[entity.EntityID]uint64
	retry map[entity.EntityID]uint64
	bed   map[entity.EntityID]entity.EntityID
}

// bedRestSave is bedRestState in the save, in id order.
type bedRestSave struct {
	Off   []entity.EntityID
	Since []entry[uint64]
	Retry []entry[uint64]
	Bed   []entry[entity.EntityID]
}

func (s *bedRestState) ensure() {
	if s.off == nil {
		s.off = make(map[entity.EntityID]bool)
		s.since = make(map[entity.EntityID]uint64)
		s.retry = make(map[entity.EntityID]uint64)
		s.bed = make(map[entity.EntityID]entity.EntityID)
	}
}

// exportBedRest is the BedRest save field (nil: nothing to keep).
func (g *Game) exportBedRest() *bedRestSave {
	s := &g.rest
	if len(s.off) == 0 && len(s.since) == 0 && len(s.retry) == 0 && len(s.bed) == 0 {
		return nil
	}
	out := &bedRestSave{Since: exportValues(s.since), Retry: exportValues(s.retry), Bed: exportValues(s.bed)}
	for id := range s.off {
		out.Off = append(out.Off, id)
	}
	slices.Sort(out.Off)
	return out
}

// importBedRest restores exportBedRest's state (nil: an older save).
func (g *Game) importBedRest(s *bedRestSave) {
	g.rest = bedRestState{}
	if s == nil {
		return
	}
	g.rest = bedRestState{off: make(map[entity.EntityID]bool, len(s.Off)),
		since: importValues(s.Since), retry: importValues(s.Retry), bed: importValues(s.Bed)}
	for _, id := range s.Off {
		g.rest.off[id] = true
	}
}

// onBedRest: colonist id is on bed rest.
func (g *Game) onBedRest(id entity.EntityID) bool {
	_, ok := g.rest.since[id]
	return ok
}

// isMedicalBed: b is a sleeping place kept for the sick and injured.
func (g *Game) isMedicalBed(b construction.Building) bool {
	return b.Medical && b.Kind.Sleeps()
}

// healingInjury is the part of id's injury rest can still heal: its open
// wounds, plus any Injury set past the body from outside (all of it with
// no body yet). Scars and lost parts never heal.
func (g *Game) healingInjury(id entity.EntityID, v health.Record) int {
	b := g.bodies[id]
	if b == nil {
		return v.Injury
	}
	n := max(0, v.Injury-b.Damage())
	for _, w := range b.Wounds {
		if !w.Permanent {
			n += w.Dmg
		}
	}
	return n
}

// needsBedRest: colonist id is hurt enough to rest in bed, whatever its
// setting: in pain, a wound or an infection to tend, sick, or waiting for
// an operation.
func (g *Game) needsBedRest(id entity.EntityID) bool {
	v, _ := g.vital.Get(id)
	return g.healingInjury(id, v) >= painInjury || g.wounds[id].Infection >= infection.Sick ||
		g.needsTending(id) || g.awaitingSurgery(id)
}

// restOver: a resting colonist is well enough to get up.
func (g *Game) restOver(id entity.EntityID) bool {
	v, _ := g.vital.Get(id)
	return g.healingInjury(id, v) <= restWellInjury && g.wounds[id].Infection < infection.Sick &&
		!g.needsTending(id) && !g.awaitingSurgery(id)
}

// mayUseMedicalBed: colonist id is a patient (resting, or hurt enough to).
func (g *Game) mayUseMedicalBed(id entity.EntityID) bool {
	return g.onBedRest(id) || g.needsBedRest(id)
}

// updateBedRest starts and ends colonist id's bed rest (phaseAI, every
// tick, before anything else may send it somewhere), and gives back a
// medical bed it held once it is up.
func (g *Game) updateBedRest(id entity.EntityID) {
	now := uint64(g.clock.Now())
	if r, ok := g.rest.retry[id]; ok && now >= r {
		delete(g.rest.retry, id)
	}
	if since, on := g.rest.since[id]; on {
		if g.rest.off[id] || now-since >= restMin && g.restOver(id) {
			delete(g.rest.since, id)
		}
	} else if g.patientRest(id) { // infection.go
		g.rest.ensure()
		g.rest.since[id] = now
	}
	if len(g.rest.bed) > 0 {
		g.leaveRestBed(id)
	}
}

// clearBedRest forgets id's rest (dead, or off the map); its setting
// stays.
func (g *Game) clearBedRest(id entity.EntityID) {
	delete(g.rest.since, id)
	delete(g.rest.retry, id)
	delete(g.rest.bed, id)
}

// noRestBed: no bed could be had for colonist id; it looks for one again
// after restRetry.
func (g *Game) noRestBed(id entity.EntityID) {
	g.rest.ensure()
	g.rest.retry[id] = uint64(g.clock.Now()) + restRetry
}

// bedRestSlot is bed rest's turn in phaseAI (after warmth): a resting
// colonist stays in bed, tends itself there when no one else can, gets
// up to eat, and lies back down after anything else it did. A player's
// order, a meal or a warm-up underway goes on (rest resumes after). True
// when it gave id something to do; false leaves id to the rest of
// phaseAI (no bed to be had: rest ends until restRetry).
func (g *Game) bedRestSlot(id entity.EntityID, j job.Record) bool {
	if !g.onBedRest(id) || j.Forced {
		return false
	}
	switch {
	case j.Kind == job.Eat || j.Kind == job.Warm:
		return false
	case j.Kind == job.Tend && j.Target == id && j.Active():
		return true // tending itself
	case j.Kind == job.Sleep && j.Active() && (j.Target != 0 || uint64(g.clock.Now()) < g.rest.retry[id] && g.waitsForDoctor(id)):
		// in bed, or on the floor for a doctor with no bed to be had
		if j.State == job.Do && g.selfTendDue(id, j) {
			g.tendSelfInBed(id)
		}
		return true
	}
	if j.Kind != job.None && j.State != job.Idle {
		g.interrupt(id, job.Interrupted)
	}
	if n, _ := g.need.Get(id); n.Hungry() && g.seekFood(id) {
		g.assignEat(id)
		return true
	}
	if g.lieDownForCare(id) { // infection.go
		return true
	}
	g.noRestBed(id)
	delete(g.rest.since, id)
	return false
}

// selfTendDue: a patient lying down a while with a wound or an infection
// to tend, and no one else to doctor it, tends itself.
func (g *Game) selfTendDue(id entity.EntityID, j job.Record) bool {
	if j.WorkLeft > needs.SleepWorkMin-selfTendSettle || !g.needsTending(id) || g.prioOf(id)[work.Doctor] == 0 {
		return false
	}
	if _, taken := g.reserve.EntityOwner(id); taken {
		return false // a doctor or a rescuer holds it
	}
	return !g.otherDoctor(id)
}

// tendSelfInBed starts patient id's tend on itself where it lies (as a
// lone colonist does at work: assignDoctor), fetching medicine first
// when there is some.
func (g *Game) tendSelfInBed(id entity.EntityID) {
	pos, _ := g.positions.Get(id)
	rec := g.withMedicine(id, job.Record{Kind: job.Tend, Target: id, TX: pos.X, TY: pos.Y, WorkLeft: TendWork * 3 / 2})
	g.interrupt(id, job.Interrupted)
	if g.startJob(id, rec) && rec.Dest != 0 {
		g.reserve.TryEntity(id, rec.Dest)
	}
}

// medicalBedFor is the medical bed patient id would rest in: one it
// holds, else the nearest free one it may use (prisoners' beds for
// prisoners only), not held by a rescuer for someone else; 0 when none.
// Medical beds take patients from any area (areaExemptJob).
func (g *Game) medicalBedFor(id entity.EntityID, pos Position) entity.EntityID {
	var best entity.EntityID
	bestD := 1 << 30
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || !g.isMedicalBed(b) || b.ForPrisoners != g.isPrisoner(id) || !b.CanClaim(uint64(id)) {
			continue
		}
		if owner, taken := g.reserve.EntityOwner(bid); taken && owner != id {
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, b.X, b.Y)
		if b.SlotOf(uint64(id)) >= 0 {
			d = -1
		}
		if best == 0 || d < bestD || d == bestD && bid < best {
			best, bestD = bid, d
		}
	}
	return best
}

// restInMedicalBed sends patient id to its medical bed, held for the stay
// (its own bed stays its own); false, with nothing changed, when there is
// none free or it can't be reached.
func (g *Game) restInMedicalBed(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	bid := g.medicalBedFor(id, pos)
	if bid == 0 {
		return false
	}
	b, _ := g.bldgs.Get(bid)
	held := b.SlotOf(uint64(id)) >= 0
	claimed := b.Claim(uint64(id))
	g.bldgs.Set(bid, claimed)
	sx, sy := g.sleepCellFor(id, claimed)
	if !g.startJob(id, job.Record{Kind: job.Sleep, Target: bid, TX: sx, TY: sy, WorkLeft: needs.SleepWorkMin}) {
		if !held {
			g.bldgs.Set(bid, b)
		}
		return false
	}
	g.holdRestBed(id, bid)
	g.sleptAt[id] = bid
	return true
}

// holdRestBed records medical bed bid as patient id's stay, given back
// once it is up and well (leaveRestBed); a stay held elsewhere ends.
func (g *Game) holdRestBed(id, bid entity.EntityID) {
	g.rest.ensure()
	if old, ok := g.rest.bed[id]; ok && old != bid {
		g.releaseSlot(id, old)
	}
	g.rest.bed[id] = bid
}

// releaseSlot frees id's slot in bed bid, if it holds one.
func (g *Game) releaseSlot(id, bid entity.EntityID) {
	if b, ok := g.bldgs.Get(bid); ok && b.SlotOf(uint64(id)) >= 0 {
		g.bldgs.Set(bid, b.Release(uint64(id)))
	}
}

// leaveRestBed gives back the medical bed id held for its stay once it
// is no longer resting, down, or lying in it.
func (g *Game) leaveRestBed(id entity.EntityID) {
	bid, ok := g.rest.bed[id]
	if !ok || g.onBedRest(id) {
		return
	}
	if v, _ := g.vital.Get(id); v.Status == health.Downed {
		return
	}
	if j, _ := g.jobs.Get(id); j.Kind == job.Sleep && j.Target == bid && j.Active() {
		return // still lying in it: sleepDone gets it up first
	}
	delete(g.rest.bed, id)
	g.releaseSlot(id, bid)
}

// lyingInBed: id lies in bed bid (b) now: asleep in it, or down on its
// sleeping cell (a check that needs no index, for loading).
func (g *Game) lyingInBed(id, bid entity.EntityID, b construction.Building) bool {
	if j, _ := g.jobs.Get(id); j.Kind == job.Sleep && j.Target == bid && j.Active() {
		return true
	}
	v, _ := g.vital.Get(id)
	slot := b.SlotOf(uint64(id))
	if v.Status != health.Downed || slot < 0 {
		return false
	}
	pos, _ := g.positions.Get(id)
	x, y := construction.SleepCell(b.Kind, b.Facing, b.X, b.Y, slot)
	return pos.X == x && pos.Y == y
}

// fillBedRestView fills a colonist's bed rest in the read model: its
// setting, and BedRest with the job line while it rests in bed.
func (g *Game) fillBedRestView(pv *PawnView, id entity.EntityID, j job.Record) {
	pv.RestWhenHurt = !g.rest.off[id]
	if g.onBedRest(id) && j.Kind == job.Sleep && j.Active() {
		pv.BedRest, pv.Job = true, restingLabel
	}
}

// migrateMedicalBeds: a save from before medical beds (v3 and older)
// gets every hospital bed medical. A colonist lying in one keeps it for
// its stay; any other colonist owner gives it up (a prisoner keeps its
// prison bed).
func (g *Game) migrateMedicalBeds() {
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || b.Kind != construction.HospitalBed {
			continue
		}
		b.Medical = true
		for _, c := range [2]uint64{b.Claimant, b.Claimant2} {
			cid := entity.EntityID(c)
			switch {
			case c == 0 || g.isPrisoner(cid):
			case g.lyingInBed(cid, bid, b):
				g.holdRestBed(cid, bid)
			default:
				b = b.Release(c)
			}
		}
		g.bldgs.Set(bid, b)
	}
}
