package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Wounds and infection. A colonist's fresh wound stays open until a doctor
// tends it; left untended for a few hours it may fester. Tending (better
// with herbal medicine) closes it with a smaller chance of infection. An
// infection grows hour by hour unless it is being treated, and kills if it
// runs its course. Sick, hurt or untended colonists rest in bed until they
// are well again (bedrest.go); with no one else to help, a colonist tends
// itself.

// wound is a colonist's open wound and infection.
type wound struct {
	At        uint64 // an untended wound opened at this tick (0: none)
	Infection int    // severity (0: none)
	Care      infection.Care
}

const (
	// patientWait caps how long a patient lies in bed waiting for a
	// surgeon (sleepDone, surgeryGaveUp).
	patientWait = 6 * clock.TicksPerHour
	// painInjury: injuries this bad hurt (mood) and send the colonist to
	// bed (counting only injury that can still heal: bedrest.go).
	painInjury = 40
)

// woundOpened records a fresh wound on a colonist.
func (g *Game) woundOpened(id entity.EntityID) {
	if g.isRaider(id) || !g.isPawn(id) {
		return
	}
	w := g.wounds[id]
	if w.At == 0 {
		w.At = max(1, uint64(g.clock.Now())) // 0 means no open wound
	}
	g.wounds[id] = w
}

// needsTending: an open wound or an infection, not under treatment.
func (g *Game) needsTending(id entity.EntityID) bool {
	if g.tended(id) {
		return false
	}
	w := g.wounds[id]
	v, _ := g.vital.Get(id)
	return w.At != 0 && g.openInjury(id, v) || w.Infection > 0 // body.go
}

// otherDoctor reports a standing colonist besides id who does Doctor work
// and is not on bed rest itself (a resting doctor tends no one else).
func (g *Game) otherDoctor(id entity.EntityID) bool {
	for _, pid := range g.pawnIDs {
		if pid != id && g.standingPawn(pid) && g.prioOf(pid)[work.Doctor] > 0 && !g.onBedRest(pid) {
			return true
		}
	}
	return false
}

// patientRest: colonist id should start bed rest (bedrest.go) — hurt
// enough (needsBedRest), with "Rest when hurt" on and no recent failure
// to find a bed. It needs no other doctor: a lone patient tends itself in
// bed.
func (g *Game) patientRest(id entity.EntityID) bool {
	if g.rest.off[id] || uint64(g.clock.Now()) < g.rest.retry[id] {
		return false
	}
	return g.needsBedRest(id)
}

// lieDownForCare puts a resting patient to bed (bedrest.go): one waiting
// for surgery takes a free hospital bed when there is one (surgery.go,
// SIM-049); then the nearest free medical bed, held for the stay; then its
// own bed or the nearest free one it may claim (sleepInBed). With no bed
// to be had, one waiting for a doctor or a surgeon lies down where it is
// (as before beds were required), and looks for a bed again after
// restRetry; anyone else is left as it was (false).
func (g *Game) lieDownForCare(id entity.EntityID) bool {
	if _, held := g.rest.bed[id]; !held && g.awaitingSurgery(id) && g.lieInHospitalBed(id) {
		return true
	}
	if g.restInMedicalBed(id) || g.sleepInBed(id) {
		return true
	}
	if !g.waitsForDoctor(id) {
		return false
	}
	g.noRestBed(id)
	g.assignSleepInPlace(id)
	return true
}

// waitsForDoctor: patient id has an operation to wait for, or a wound or
// an infection another colonist will come and tend (it can lie down on
// the floor for them).
func (g *Game) waitsForDoctor(id entity.EntityID) bool {
	return g.awaitingSurgery(id) || g.needsTending(id) && g.otherDoctor(id)
}

// lyingForCare: the patient is down, in bed, or asleep (a doctor can work
// on them where they lie).
func (g *Game) lyingForCare(id entity.EntityID) bool {
	v, _ := g.vital.Get(id)
	if v.Status == health.Downed || g.inBed(id) {
		return true
	}
	j, _ := g.jobs.Get(id)
	return j.Kind == job.Sleep && j.State == job.Do
}

// withMedicine adds a fetch of medicine to a tend job (rec.Target is the
// patient), if there is some to be had. The tier is chosen here, once, from
// the patient's state: industrial for a severe injury or an infection,
// herbal otherwise, each falling back to the other. The job carries the
// chosen dose, so the tier cannot flap while the doctor works (NG-1 §4.8).
func (g *Game) withMedicine(id entity.EntityID, rec job.Record) job.Record {
	for _, k := range g.medicineTiers(rec.Target) {
		sid, st := g.ingredientFor(id, recipe.Input{Kind: k, Qty: 1, Max: 1})
		if sid == 0 {
			continue
		}
		if _, taken := g.reserve.EntityOwner(sid); taken {
			continue
		}
		rec.Dest, rec.DestX, rec.DestY = sid, rec.TX, rec.TY
		rec.TX, rec.TY = st.X, st.Y
		return rec
	}
	return rec
}

// pickUpMedicine: the doctor reached the medicine; take one dose and head
// for the patient. Reports false if the job had to end.
func (g *Game) pickUpMedicine(id entity.EntityID, j *job.Record) bool {
	return g.pickUpItem(id, j, isMedicine)
}

// pickUpItem: the doctor reached j.Dest; take one of it (if accept likes
// its kind) and head for the patient at DestX/DestY. Reports false while
// walking, or if the job had to end (surgery.go fetches implants with it).
func (g *Game) pickUpItem(id entity.EntityID, j *job.Record, accept func(item.Kind) bool) bool {
	st, ok := g.stacks.Get(j.Dest)
	if !ok || st.Carrier != 0 || !accept(st.Kind) {
		j.Dest = 0 // gone: tend without it
	} else {
		carry := j.Dest
		if st.Qty > 1 {
			st.Qty--
			g.stacks.Set(j.Dest, st)
			carry = g.entities.Next()
			g.itemIDs = append(g.itemIDs, carry)
			g.stacks.Set(carry, item.Stack{Kind: st.Kind, Qty: 1, X: st.X, Y: st.Y, Carrier: id})
		} else {
			st.Carrier = id
			g.stacks.Set(j.Dest, st)
		}
		j.Carry = carry
	}
	path, ok := g.pathTo(id, j.DestX, j.DestY)
	if !ok {
		j.State, j.Fail = job.Fail, job.Unreachable
		g.jobs.Set(id, *j)
		return false
	}
	j.TX, j.TY, j.Path, j.PathI = j.DestX, j.DestY, path, 0
	j.Dest = 0
	if len(path) > 0 {
		j.State = job.Goto
		g.jobs.Set(id, *j)
		return false
	}
	return true
}

// tendCare is the treatment a finished tend gives.
func (g *Game) tendCare(doctor, patient entity.EntityID, j job.Record) infection.Care {
	switch {
	case j.Carry != 0:
		if st, ok := g.stacks.Get(j.Carry); ok && st.Kind == item.IndustrialMedicine {
			return infection.IndustrialMedicine
		}
		return infection.Medicine
	case doctor == patient:
		return infection.SelfTended
	}
	return infection.Tended
}

// tendHeal is the injury a tend mends at once.
func tendHeal(c infection.Care) int {
	switch c {
	case infection.IndustrialMedicine:
		return TendHeal * infection.HealPct(c) / 100
	case infection.Medicine:
		return TendHeal * 2
	case infection.SelfTended:
		return TendHeal / 2
	}
	return TendHeal
}

// closeWound: a tend closes an open wound (it may still fester).
func (g *Game) closeWound(id entity.EntityID, c infection.Care, doctor entity.EntityID) {
	w := g.wounds[id]
	if w.At != 0 && w.Infection == 0 && g.rng.IntN(100) < g.infectionPct(g.infectionChance(id, c)*g.doctorInfectionPct(doctor)/100) { // story.go: difficulty (NG-1)
		w.Infection = infection.Onset
		pos, _ := g.positions.Get(id)
		g.logEvent(id, logEntry{Kind: logInfected})
		g.message(pos.X, pos.Y, "%s's wound is infected.", g.name(id))
	}
	w.At, w.Care = 0, c
	g.setWound(id, w)
}

func (g *Game) setWound(id entity.EntityID, w wound) {
	if w.At == 0 && w.Infection == 0 {
		delete(g.wounds, id)
		return
	}
	g.wounds[id] = w
}

// phaseInfection runs once an hour: open wounds may fester, infections
// grow or recede.
func (g *Game) phaseInfection() {
	now := uint64(g.clock.Now())
	if now%clock.TicksPerHour != 0 || len(g.wounds) == 0 {
		return
	}
	for _, id := range g.pawnIDs {
		w, ok := g.wounds[id]
		if !ok {
			continue
		}
		v, _ := g.vital.Get(id)
		if v.Status == health.Dead {
			delete(g.wounds, id)
			continue
		}
		pos, _ := g.positions.Get(id)
		if w.At != 0 && now-w.At >= infection.DelayHours*clock.TicksPerHour {
			if w.Infection == 0 && g.rng.IntN(100) < g.infectionPct(infection.Chance(infection.Untreated)) { // story.go: difficulty (NG-1)
				w.Infection = infection.Onset
				g.logEvent(id, logEntry{Kind: logInfected})
				g.message(pos.X, pos.Y, "%s's untended wound is infected.", g.name(id))
			}
			w.At = 0
		}
		if w.Infection > 0 {
			w.Infection = infection.Step(w.Infection, g.tended(id), w.Care)
			switch w.Infection {
			case infection.Fatal:
				g.setWound(id, wound{})
				g.message(pos.X, pos.Y, "%s died of infection.", g.name(id))
				g.die(id, health.CauseInfection)
				continue
			case 0:
				g.logEvent(id, logEntry{Kind: logInfectionCleared})
				g.message(pos.X, pos.Y, "%s fought off the infection.", g.name(id))
				w.Care = infection.Untreated
			}
		}
		g.setWound(id, w)
	}
}

// InfectionOf is a colonist's infection severity (0: none).
func (g *Game) InfectionOf(id entity.EntityID) int { return g.wounds[id].Infection }

// Infect gives a colonist an infection (dev scenarios and fixtures).
func (g *Game) Infect(id entity.EntityID, severity int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	w := g.wounds[id]
	w.Infection = severity
	g.wounds[id] = w
}
