package care

import (
	"maps"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

const (
	// TendWork is the ticks a doctor spends tending one patient.
	TendWork = 300
	// TendTicks is how long a tend shows as tended (PawnView.Tended) and
	// keeps an animal from being tended again.
	TendTicks = clock.TicksPerDay
	// emergencyMedReach is how far (pathing.Dist tiles) a drafted
	// colonist looks for medicine on an emergency tend before tending
	// without (original tuning).
	emergencyMedReach = 15
)

// Order reasons (player-facing). The first three are the game's texts of
// the same names (orders.go); a game test keeps them equal.
const (
	ReasonIncapable     = "not able to act"
	ReasonBusyTarget    = "in use by "
	ReasonUnreachable   = "cannot reach"
	ReasonNothingToTend = "nothing to tend"
	ReasonNoSelfTend    = "self-tend is off"
)

// TendedUntil is the tick id's last tend stops showing (0: never tended).
func (d *Doctor) TendedUntil(id entity.EntityID) uint64 { return d.tendedUntil[id] }

// MarkTended records a tend of id that shows until tick until.
func (d *Doctor) MarkTended(id entity.EntityID, until uint64) { d.tendedUntil[id] = until }

// Forget drops id's tend record (it left the map).
func (d *Doctor) Forget(id entity.EntityID) { delete(d.tendedUntil, id) }

// ExportTended is the TendedUntil save field (a copy).
func (d *Doctor) ExportTended() map[entity.EntityID]uint64 { return maps.Clone(d.tendedUntil) }

// ImportTended restores ExportTended's map; nil (a save without the
// field) keeps the records as they are.
func (d *Doctor) ImportTended(m map[entity.EntityID]uint64) {
	if m != nil {
		d.tendedUntil = m
	}
}

// OtherDoctor reports a standing colonist besides id who does Doctor
// work and is not on bed rest itself (a resting doctor tends no one else).
func (d *Doctor) OtherDoctor(id entity.EntityID) bool {
	for _, pid := range d.h.PawnIDs() {
		if pid != id && d.h.Standing(pid) && d.h.DoesDoctor(pid) && !d.c.Rest.On(pid) {
			return true
		}
	}
	return false
}

// Lying: the patient is down, in bed, or asleep (a doctor can work on
// it where it lies).
func (d *Doctor) Lying(id entity.EntityID) bool {
	v, _ := d.h.Vital(id)
	if v.Status == health.Downed || d.h.BedUnder(id) != 0 {
		return true
	}
	j, _ := d.h.Job(id)
	return j.Kind == job.Sleep && j.State == job.Do
}

// tendable: a doctor would tend pid where it lies (down, in bed or
// asleep) with something to tend, its care level allowing care, and not
// waiting to be rescued to a bed first. bleeding picks the patients
// losing blood (true) or the rest (false).
func (d *Doctor) tendable(pid entity.EntityID, bleeding bool) bool {
	if !d.Lying(pid) || !d.h.NeedsTending(pid) || !d.c.Level(pid).Tends() || d.Rescuable(pid) {
		return false
	}
	return (d.h.BleedPerDay(pid) > 0) == bleeding
}

// assignTendOther tends the worst hurt patient that is not bleeding;
// with no one else to tend and no other doctor around, a colonist tends
// its own wounds (unless its self-tend is off).
func (d *Doctor) assignTendOther(id entity.EntityID) bool {
	if d.assignTendWhere(id, false) {
		return true
	}
	if !d.h.NeedsTending(id) || d.OtherDoctor(id) || !d.c.SelfTendAllowed(id) {
		return false
	}
	x, y, _ := d.h.Pos(id)
	return d.startTend(id, job.Record{Kind: job.Tend, Target: id, TX: x, TY: y, WorkLeft: TendWork * 3 / 2})
}

// assignTendWhere tends the best ranked tendable patient (bleeding or
// not). The patient stays reserved until the tend ends, so a doctor never
// leaves one for another halfway (no flapping).
func (d *Doctor) assignTendWhere(id entity.EntityID, bleeding bool) bool {
	best := d.bestPatient(id, func(pid entity.EntityID) (job.Record, bool) {
		return job.Record{Kind: job.Tend, Target: pid, WorkLeft: TendWork}, d.tendable(pid, bleeding)
	})
	return best != nil && d.startTend(id, best.rec)
}

// startTend starts tend rec with a fetch of the medicine the patient's
// care level allows (WithMedicine), holding the dose for the trip.
func (d *Doctor) startTend(id entity.EntityID, rec job.Record) bool {
	rec = d.WithMedicine(id, rec)
	if !d.h.StartJob(id, rec) {
		return false
	}
	if rec.Dest != 0 {
		d.h.Claim(id, rec.Dest)
	}
	return true
}

// WithMedicine adds a fetch of medicine to a tend job (rec.Target is the
// patient), if there is some to be had. The tier is chosen here, once,
// from the patient's state (Treatment.MedicineTiers), and only what the
// patient's care level allows (none for "No care" and "Care, no
// medicine"). The job carries the chosen dose, so the tier cannot flap
// while the doctor works (NG-1 §4.8).
func (d *Doctor) WithMedicine(id entity.EntityID, rec job.Record) job.Record {
	pol := d.c.Level(rec.Target)
	for _, k := range d.h.MedicineTiers(rec.Target) {
		if !medcare.Allows(pol, k) {
			continue
		}
		sid, st := d.h.NearestStack(id, k)
		if sid == 0 {
			continue
		}
		if _, taken := d.h.Owner(sid); taken {
			continue
		}
		rec.Dest, rec.DestX, rec.DestY = sid, rec.TX, rec.TY
		rec.TX, rec.TY = st.X, st.Y
		return rec
	}
	return rec
}

// DoTend runs tend job j of doctor id: fetch the dose if the job carries
// one, work beside the patient, then tend it at a quality the host rolls
// from the doctor, the medicine and the bed. A patient with nothing left
// to tend (another doctor got there first) ends the job.
func (d *Doctor) DoTend(id entity.EntityID, j job.Record) {
	h := d.h
	v, ok := h.Vital(j.Target)
	if !ok || v.Status == health.Dead || !h.NeedsTending(j.Target) {
		j.State = job.Complete
		h.SetJob(id, j)
		return
	}
	if j.Dest != 0 && j.Carry == 0 {
		var arrived bool
		if j, arrived = h.PickUpDose(id, j); !arrived {
			return // walking to the patient (or the job ended)
		}
	}
	if j.Target != id {
		x, y, _ := h.Pos(id)
		if px, py, _ := h.Pos(j.Target); abs(px-x) > 1 || abs(py-y) > 1 {
			j.State, j.Fail = job.Fail, job.TargetGone // the patient got up and left
			h.SetJob(id, j)
			return
		}
	}
	u := h.MedicineWork(id)
	if u == 0 {
		h.SetJob(id, j)
		return
	}
	j.WorkLeft -= u
	if j.WorkLeft > 0 {
		h.SetJob(id, j)
		return
	}
	now := h.Now()
	q := h.TendQuality(id, j.Target, j)
	if j.Carry != 0 {
		h.DeleteItem(j.Carry)
		j.Carry = 0
	}
	h.ApplyTend(j.Target, q)
	d.tendedUntil[j.Target] = now + TendTicks
	h.TendedBy(j.Target, id)
	j.State = job.Complete
	h.SetJob(id, j)
}

// CanEmergencyTend reports why drafted colonist doctor cannot tend
// patient now ("" when it can): tendWhy (forced.go, shared with
// "Prioritize tending"), drafted.
func (d *Doctor) CanEmergencyTend(doctor, patient entity.EntityID) string {
	if !d.h.Drafted(doctor) {
		return ReasonIncapable
	}
	return d.tendWhy(doctor, patient)
}

// EmergencyTend sends drafted doctor to tend patient now: it fetches the
// best medicine the patient's care level allows within emergencyMedReach
// (none when there is none) and tends beside it. It stays drafted;
// undrafting, or the patient healing, dying or leaving, ends the job
// (DoEmergencyTend).
func (d *Doctor) EmergencyTend(doctor, patient entity.EntityID) {
	if d.CanEmergencyTend(doctor, patient) != "" {
		return
	}
	h := d.h
	px, py, _ := h.Pos(patient)
	sx, sy, _ := h.BesideSpot(doctor, px, py)
	wl := TendWork
	if doctor == patient {
		wl = TendWork * 3 / 2
	}
	h.Interrupt(doctor, job.Cancelled) // a plain order already replaced the queue (tick.go)
	rec := d.emergencyMedicine(doctor, job.Record{Kind: job.EmergencyTend, Target: patient, TX: sx, TY: sy,
		WorkLeft: wl, Forced: true})
	if h.StartJob(doctor, rec) && rec.Dest != 0 {
		h.Claim(doctor, rec.Dest)
	}
}

// emergencyMedicine adds a fetch of the best medicine rec.Target's care
// level allows to rec, if there is some within emergencyMedReach of
// doctor; WithMedicine's route (medicine first, then the patient).
func (d *Doctor) emergencyMedicine(doctor entity.EntityID, rec job.Record) job.Record {
	pol := d.c.Level(rec.Target)
	x, y, _ := d.h.Pos(doctor)
	for _, k := range [...]item.Kind{item.IndustrialMedicine, item.HerbalMedicine} {
		if !medcare.Allows(pol, k) {
			continue
		}
		sid, st := d.h.NearestStack(doctor, k)
		if sid == 0 || pathing.Dist(x, y, st.X, st.Y) > emergencyMedReach {
			continue
		}
		if _, taken := d.h.Owner(sid); taken {
			continue
		}
		rec.Dest, rec.DestX, rec.DestY = sid, rec.TX, rec.TY
		rec.TX, rec.TY = st.X, st.Y
		return rec
	}
	return rec
}

// DoEmergencyTend runs an emergency tend like any tend (DoTend), while
// the doctor stays drafted; undrafted (the host interrupts it anyway) or
// downed, it ends.
func (d *Doctor) DoEmergencyTend(id entity.EntityID, j job.Record) {
	if v, _ := d.h.Vital(id); !d.h.Drafted(id) || v.Status != health.Alive {
		j.State = job.Complete
		d.h.SetJob(id, j)
		return
	}
	d.DoTend(id, j)
}
