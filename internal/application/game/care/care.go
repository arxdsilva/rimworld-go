package care

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/entity"
)

// Care is the care cluster's root: the game builds one with New and keeps
// one field. Its components are exported so the game reaches each by name
// (g.care.Doctor.DoTend); the root itself holds what several of them
// share: the care level and the carry flow.
type Care struct {
	Carry  *Carries
	Policy *Policy
	Doctor *Doctor
	Rest   *Rest
	Feed   *Feeding // feed.go (SIM-064)
	// PrisonBeds returns downed prisoners to bed (prisonbed.go, SIM-063);
	// Guest looks after rescued visitors and strangers (guest.go, SIM-063);
	// Visits and Strip are visiting the sick and stripping (SIM-065).
	PrisonBeds *PrisonBeds
	Guest      *Hospitality
	Visits     *Visits
	Strip      *Stripping
	h          Host
	pr         Prison
	gu         Guests
	so         Social
	ge         Gear
}

// Roles are the host roles beyond Host, each its own consumer-owned
// interface so Host stays at eight roles: Prison for capture and arrest
// (SIM-062), Guests for rescuing visitors and strangers (SIM-063), Social
// for visiting the sick and Gear for stripping (SIM-065).
type Roles struct {
	Prison Prison
	Guests Guests
	Social Social
	Gear   Gear
	Meals  Meals // feed.go (SIM-064)
}

// New builds the care cluster on host h with the extra roles r.
func New(h Host, r Roles) *Care {
	c := &Care{Carry: NewCarries(), Policy: &Policy{}, h: h, pr: r.Prison, gu: r.Guests, so: r.Social, ge: r.Gear}
	c.Doctor = &Doctor{c: c, h: h, tendedUntil: make(map[entity.EntityID]uint64)}
	c.Rest = &Rest{c: c, h: h}
	c.Feed = newFeeding(c, h, r.Meals)
	c.PrisonBeds, c.Guest = newPrisonBeds(c), newHospitality(c)
	c.Visits, c.Strip = newVisits(c, r.Social), newStripping(c, r.Gear)
	return c
}

// Level is patient id's care level: its own, else its group's default.
func (c *Care) Level(id entity.EntityID) medcare.Policy {
	if l, ok := c.Policy.Own(id); ok {
		return l
	}
	return c.Policy.Default(c.h.CareGroup(id))
}

// SelfTendAllowed: colonist id may tend itself (the player did not turn
// it off, and its care level allows a tend at all).
func (c *Care) SelfTendAllowed(id entity.EntityID) bool {
	return !c.Policy.SelfTendOff(id) && c.Level(id).Tends()
}

// SetMedicalCare sets patient's care level to mode (a medcare.Policy);
// anything else, or someone who is no patient at all, is ignored.
func (c *Care) SetMedicalCare(patient entity.EntityID, mode uint8) {
	if !medcare.Policy(mode).Valid() || !c.h.IsPatient(patient) {
		return
	}
	c.Policy.Set(patient, mode)
}

// SetSelfTend lets colonist id tend itself (on) or not.
func (c *Care) SetSelfTend(id entity.EntityID, on bool) {
	if c.h.IsColonist(id) {
		c.Policy.SetSelfTend(id, on)
	}
}

// DropPatient puts the patient carrier id holds down where it stands.
func (c *Care) DropPatient(id entity.EntityID) {
	p, ok := c.Carry.Drop(id)
	if !ok {
		return
	}
	x, y, _ := c.h.Pos(id)
	c.h.Place(p, x, y)
}

// FollowCarriers keeps carried patients on their carrier's tile.
func (c *Care) FollowCarriers() {
	for _, id := range c.Carry.order {
		if x, y, ok := c.h.Pos(id); ok {
			c.h.Place(c.Carry.byCarrier[id], x, y)
		}
	}
}

// CareView is a pawn's care in the read model.
type CareView struct {
	CarriedBy    entity.EntityID // the rescuer carrying it (0: none)
	Carrying     entity.EntityID // the pawn it carries (0: none; SIM-062)
	OnMedicalBed bool            // lying in a medical bed (SIM-063, UI 10.4)
	Tended       bool            // a tend still shows (TendTicks)
	BedRest      bool            // resting in bed to heal (PLAY-020)
	CareLevel    medcare.Policy  // its care level (MED-03)
	RestWhenHurt bool            // the colonist's setting (on unless turned off)
}

// View is pawn id's care for the read model; j is its current job.
func (c *Care) View(id entity.EntityID, j job.Record) CareView {
	return CareView{
		CarriedBy:    c.Carry.Carrier(id),
		Carrying:     c.Carry.Patient(id),
		OnMedicalBed: onMedicalBed(c.h, id),
		Tended:       c.h.Now() < c.Doctor.TendedUntil(id),
		BedRest:      c.Rest.On(id) && j.Kind == job.Sleep && j.Active(),
		CareLevel:    c.Level(id),
		RestWhenHurt: !c.Rest.Off(id),
	}
}

// onMedicalBed: id lies in a bed kept for the sick and injured.
func onMedicalBed(h Host, id entity.EntityID) bool {
	b, ok := h.Building(h.BedUnder(id))
	return ok && IsMedicalBed(b)
}
