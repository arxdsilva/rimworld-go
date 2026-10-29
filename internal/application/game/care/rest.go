package care

import (
	"maps"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Bed rest and medical beds (PLAY-020).
//
// A colonist in pain, with a wound or an infection no one has tended,
// sick, or waiting for an operation goes to bed (PatientRest) and stays
// there until it is well again (restOver): at most RestWellInjury of
// injury that can still heal, nothing left to tend, not sick, and never
// before RestMin. The two lines are far apart (pain at PainInjury, well at
// RestWellInjury), so a patient never flaps between bed and work. Only
// injury that can still heal counts: scars and lost parts never keep
// anyone in bed.
//
// A resting colonist gets up only for what the host's AI puts first
// (fire, flight, mental breaks, social duty, warmth, drafting, the
// player's orders) and to eat, and lies down again after. With no other
// colonist who can doctor, it tends itself where it lies. It takes the
// nearest free medical bed, held only for the stay, else its own bed,
// else the nearest free bed it may claim; with none free it carries on
// (or, if a doctor or a surgeon is to come, lies down where it is) and
// looks again after RestRetry. The player can turn "Rest when hurt" off
// per colonist (SetPawnBedRest).
//
// A medical bed (Building.Medical, SetBedMedical; hospital beds start
// medical) belongs to no one: healthy colonists never sleep in it, own it,
// or are ordered into it.

const (
	// PainInjury: injuries this bad hurt (mood) and send the colonist to
	// bed (counting only injury that can still heal).
	PainInjury = 40
	// RestWellInjury: rest may end once healing injury is at most this.
	RestWellInjury = 15
	// RestMin is the shortest bed rest.
	RestMin = 2 * clock.TicksPerHour
	// RestRetry: a patient who found no free bed carries on and looks for
	// one again after this long.
	RestRetry = 6 * clock.TicksPerHour
	// selfTendSettle is how long a patient with no one else to doctor
	// lies in bed before it tends itself (a failed try waits as long).
	selfTendSettle = 250
	// RestingLabel is a resting colonist's job line.
	RestingLabel = "Resting (injured)"
	// ReasonMedicalBed: a healthy colonist can't be ordered into a
	// medical bed.
	ReasonMedicalBed = "for the sick and injured"
)

// Rest is every colonist's bed rest: who turned it off, who rests since
// when, who found no bed until when, and the medical bed each patient
// holds only for its stay. Maps are made on first use.
type Rest struct {
	c     *Care
	h     Host
	off   map[entity.EntityID]bool
	since map[entity.EntityID]uint64
	retry map[entity.EntityID]uint64
	bed   map[entity.EntityID]entity.EntityID
}

// Entry is one id's value in a save list.
type Entry[T any] struct {
	ID entity.EntityID
	V  T
}

// RestSave is Rest in the save, in id order. Its field names are the
// game's pre-SIM-061 bedRestSave.
type RestSave struct {
	Off   []entity.EntityID
	Since []Entry[uint64]
	Retry []Entry[uint64]
	Bed   []Entry[entity.EntityID]
}

func (r *Rest) ensure() {
	if r.off == nil {
		r.off = make(map[entity.EntityID]bool)
		r.since = make(map[entity.EntityID]uint64)
		r.retry = make(map[entity.EntityID]uint64)
		r.bed = make(map[entity.EntityID]entity.EntityID)
	}
}

func exportEntries[T any](m map[entity.EntityID]T) []Entry[T] {
	if len(m) == 0 {
		return nil
	}
	out := make([]Entry[T], 0, len(m))
	for _, id := range slices.Sorted(maps.Keys(m)) {
		out = append(out, Entry[T]{id, m[id]})
	}
	return out
}

func importEntries[T any](es []Entry[T]) map[entity.EntityID]T {
	m := make(map[entity.EntityID]T, len(es))
	for _, e := range es {
		m[e.ID] = e.V
	}
	return m
}

// Export is the BedRest save field (nil: nothing to keep).
func (r *Rest) Export() *RestSave {
	if len(r.off) == 0 && len(r.since) == 0 && len(r.retry) == 0 && len(r.bed) == 0 {
		return nil
	}
	return &RestSave{Off: slices.Sorted(maps.Keys(r.off)), Since: exportEntries(r.since),
		Retry: exportEntries(r.retry), Bed: exportEntries(r.bed)}
}

// Import restores Export's state (nil: an older save).
func (r *Rest) Import(s *RestSave) {
	r.off, r.since, r.retry, r.bed = nil, nil, nil, nil
	if s == nil {
		return
	}
	r.off = make(map[entity.EntityID]bool, len(s.Off))
	r.since, r.retry, r.bed = importEntries(s.Since), importEntries(s.Retry), importEntries(s.Bed)
	for _, id := range s.Off {
		r.off[id] = true
	}
}

// On: colonist id is on bed rest.
func (r *Rest) On(id entity.EntityID) bool {
	_, ok := r.since[id]
	return ok
}

// Since is when id's bed rest began.
func (r *Rest) Since(id entity.EntityID) (uint64, bool) {
	t, ok := r.since[id]
	return t, ok
}

// Start puts id on bed rest as of tick since (fixtures and tests).
func (r *Rest) Start(id entity.EntityID, since uint64) {
	r.ensure()
	r.since[id] = since
}

// RetryAt is when id, having found no bed, looks for one again (0: not
// waiting).
func (r *Rest) RetryAt(id entity.EntityID) uint64 { return r.retry[id] }

// HeldBed is the medical bed id holds for its stay (0: none).
func (r *Rest) HeldBed(id entity.EntityID) entity.EntityID { return r.bed[id] }

// Off: the player turned id's "Rest when hurt" off.
func (r *Rest) Off(id entity.EntityID) bool { return r.off[id] }

// NeedsBedRest: colonist id is hurt enough to rest in bed, whatever its
// setting: in pain, sick (an infection, a disease at its major stage or
// worse, food poisoning at its major stage), a wound or an infection to
// tend, or waiting for an operation.
func (r *Rest) NeedsBedRest(id entity.EntityID) bool {
	return r.h.HealingInjury(id) >= PainInjury || r.h.SickRest(id) || r.h.NeedsTending(id) || r.h.AwaitingSurgery(id)
}

// restOver: a resting colonist is well enough to get up. A disease keeps
// it in bed until immune or cured, even once back below its major stage;
// food poisoning until it is recovering (Health.SickStill).
func (r *Rest) restOver(id entity.EntityID) bool {
	return r.h.HealingInjury(id) <= RestWellInjury && !r.h.SickStill(id) && !r.h.NeedsTending(id) && !r.h.AwaitingSurgery(id)
}

// MayUseMedicalBed: colonist id is a patient (resting, or hurt enough to).
func (r *Rest) MayUseMedicalBed(id entity.EntityID) bool {
	return r.On(id) || r.NeedsBedRest(id)
}

// PatientRest: colonist id should start bed rest — hurt enough
// (NeedsBedRest), with "Rest when hurt" on and no recent failure to find
// a bed. It needs no other doctor: a lone patient tends itself in bed.
func (r *Rest) PatientRest(id entity.EntityID) bool {
	if r.off[id] || r.h.Now() < r.retry[id] {
		return false
	}
	return r.NeedsBedRest(id)
}

// Update starts and ends colonist id's bed rest (the host's AI, every
// tick, before anything else may send it somewhere), and gives back a
// medical bed it held once it is up.
func (r *Rest) Update(id entity.EntityID) {
	now := r.h.Now()
	if t, ok := r.retry[id]; ok && now >= t {
		delete(r.retry, id)
	}
	if since, on := r.since[id]; on {
		if r.off[id] || now-since >= RestMin && r.restOver(id) {
			delete(r.since, id)
		}
	} else if r.PatientRest(id) {
		r.ensure()
		r.since[id] = now
	}
	if len(r.bed) > 0 {
		r.leaveBed(id)
	}
}

// Clear forgets id's rest (dead, or off the map); its setting stays.
func (r *Rest) Clear(id entity.EntityID) {
	delete(r.since, id)
	delete(r.retry, id)
	delete(r.bed, id)
}

// noBed: no bed could be had for colonist id; it looks for one again
// after RestRetry.
func (r *Rest) noBed(id entity.EntityID) {
	r.ensure()
	r.retry[id] = r.h.Now() + RestRetry
}

// Slot is bed rest's turn in the host's AI (after warmth): a resting
// colonist stays in bed, tends itself there when no one else can, gets
// up to eat, and lies back down after anything else it did. A player's
// order, a meal or a warm-up underway goes on (rest resumes after). True
// when it gave id something to do; false leaves id to the rest of the AI
// (not resting, or no bed to be had: rest ends until RestRetry).
func (r *Rest) Slot(id entity.EntityID, j job.Record) bool {
	if !r.On(id) || j.Forced {
		return false
	}
	switch {
	case j.Kind == job.Eat || j.Kind == job.Warm:
		return false
	case j.Kind == job.Tend && j.Target == id && j.Active():
		return true // tending itself
	case j.Kind == job.Sleep && j.Active() && (j.Target != 0 || r.h.Now() < r.retry[id] && r.WaitsForDoctor(id)):
		// in bed, or on the floor for a doctor with no bed to be had
		if j.State == job.Do && r.SelfTendDue(id, j) {
			r.tendSelfInBed(id)
		}
		return true
	}
	if j.Kind != job.None && j.State != job.Idle {
		r.h.Interrupt(id, job.Interrupted)
	}
	if r.h.GoEat(id) {
		return true
	}
	if r.LieDown(id) {
		return true
	}
	r.noBed(id)
	delete(r.since, id)
	return false
}

// SelfTendDue: a patient lying down a while with a wound or an infection
// to tend, and no one else to doctor it, tends itself.
func (r *Rest) SelfTendDue(id entity.EntityID, j job.Record) bool {
	if j.WorkLeft > needs.SleepWorkMin-selfTendSettle || !r.h.NeedsTending(id) || !r.h.DoesDoctor(id) ||
		!r.c.SelfTendAllowed(id) { // self-tend off, or no care at all
		return false
	}
	if _, taken := r.h.Owner(id); taken {
		return false // a doctor or a rescuer holds it
	}
	return !r.c.Doctor.OtherDoctor(id)
}

// tendSelfInBed starts patient id's tend on itself where it lies (as a
// lone colonist does at work: Doctor.Assign), fetching medicine first
// when there is some.
func (r *Rest) tendSelfInBed(id entity.EntityID) {
	x, y, _ := r.h.Pos(id)
	rec := r.c.Doctor.WithMedicine(id, job.Record{Kind: job.Tend, Target: id, TX: x, TY: y, WorkLeft: TendWork * 3 / 2})
	r.h.Interrupt(id, job.Interrupted)
	if r.h.StartJob(id, rec) && rec.Dest != 0 {
		r.h.Claim(id, rec.Dest)
	}
}

// LieDown puts a resting patient to bed: one waiting for surgery takes a
// free hospital bed when there is one (SIM-049); then the nearest free
// medical bed, held for the stay; then its own bed or the nearest free
// one it may claim. With no bed to be had, one waiting for a doctor or a
// surgeon lies down where it is (as before beds were required), and looks
// for a bed again after RestRetry; anyone else is left as it was (false).
func (r *Rest) LieDown(id entity.EntityID) bool {
	if _, held := r.bed[id]; !held && r.h.AwaitingSurgery(id) && r.h.LieInHospitalBed(id) {
		return true
	}
	if r.restInMedicalBed(id) || r.h.SleepInBed(id) {
		return true
	}
	if !r.WaitsForDoctor(id) {
		return false
	}
	r.noBed(id)
	r.h.SleepInPlace(id)
	return true
}

// WaitsForDoctor: patient id has an operation to wait for, or a wound or
// an infection another colonist will come and tend (it can lie down on
// the floor for them).
func (r *Rest) WaitsForDoctor(id entity.EntityID) bool {
	return r.h.AwaitingSurgery(id) || r.h.NeedsTending(id) && r.c.Doctor.OtherDoctor(id)
}

// restInMedicalBed sends patient id to its medical bed, held for the stay
// (its own bed stays its own); false, with nothing changed, when there is
// none free or it can't be reached.
func (r *Rest) restInMedicalBed(id entity.EntityID) bool {
	x, y, _ := r.h.Pos(id)
	bid := r.c.SafeBedFor(id, x, y, RestBed)
	if bid == 0 {
		return false
	}
	b, _ := r.h.Building(bid)
	held := b.SlotOf(uint64(id)) >= 0
	claimed := b.Claim(uint64(id))
	r.h.SetBuilding(bid, claimed)
	sx, sy := r.h.SleepCell(id, claimed)
	if !r.h.StartJob(id, job.Record{Kind: job.Sleep, Target: bid, TX: sx, TY: sy, WorkLeft: needs.SleepWorkMin}) {
		if !held {
			r.h.SetBuilding(bid, b)
		}
		return false
	}
	r.Hold(id, bid)
	r.h.SetSleptAt(id, bid)
	return true
}

// Hold records medical bed bid as patient id's stay, given back once it
// is up and well (leaveBed); a stay held elsewhere ends.
func (r *Rest) Hold(id, bid entity.EntityID) {
	r.ensure()
	if old, ok := r.bed[id]; ok && old != bid {
		r.releaseSlot(id, old)
	}
	r.bed[id] = bid
}

// releaseSlot frees id's slot in bed bid, if it holds one.
func (r *Rest) releaseSlot(id, bid entity.EntityID) {
	if b, ok := r.h.Building(bid); ok && b.SlotOf(uint64(id)) >= 0 {
		r.h.SetBuilding(bid, b.Release(uint64(id)))
	}
}

// leaveBed gives back the medical bed id held for its stay once it is no
// longer resting, down, or lying in it.
func (r *Rest) leaveBed(id entity.EntityID) {
	bid, ok := r.bed[id]
	if !ok || r.On(id) {
		return
	}
	if v, _ := r.h.Vital(id); v.Status == health.Downed {
		return
	}
	if j, _ := r.h.Job(id); j.Kind == job.Sleep && j.Target == bid && j.Active() {
		return // still lying in it: the host's sleepDone gets it up first
	}
	delete(r.bed, id)
	r.releaseSlot(id, bid)
}

// SetBedMedical keeps bed id for the sick and injured (on) or not; it
// changes that bed only. Turning it on frees the bed of its owners: a
// colonist who is a patient keeps its slot for its stay; any other gets
// up if it sleeps there. A prisoner keeps its prison bed.
func (r *Rest) SetBedMedical(id entity.EntityID, on bool) {
	b, ok := r.h.Building(id)
	if !ok || !b.Kind.Sleeps() || b.Medical == on {
		return
	}
	b.Medical = on
	var up []entity.EntityID
	if on {
		for _, c := range [2]uint64{b.Claimant, b.Claimant2} {
			cid := entity.EntityID(c)
			switch {
			case c == 0 || r.h.IsPrisoner(cid):
			case r.c.Doctor.InjuredDowned(cid) || r.MayUseMedicalBed(cid):
				r.Hold(cid, id)
			default:
				b = b.Release(c)
				if j, _ := r.h.Job(cid); j.Kind == job.Sleep && j.Target == id && j.Active() {
					up = append(up, cid)
				}
			}
		}
	}
	r.h.SetBuilding(id, b)
	for _, cid := range up {
		r.h.Interrupt(cid, job.Interrupted)
	}
}

// SetPawnBedRest turns colonist id's rest when hurt on or off; off ends a
// rest at once.
func (r *Rest) SetPawnBedRest(id entity.EntityID, on bool) {
	if !r.h.IsColonist(id) {
		return
	}
	if on {
		delete(r.off, id)
		return
	}
	r.ensure()
	r.off[id] = true
	delete(r.since, id)
}
