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
// runs its course. Sick or badly hurt colonists rest in bed so a doctor
// can see to them; with no one else to help, a colonist tends itself.

// wound is a colonist's open wound and infection.
type wound struct {
	At        uint64 // an untended wound opened at this tick (0: none)
	Infection int    // severity (0: none)
	Care      infection.Care
}

const (
	// patientWait caps how long a patient lies in bed waiting for a doctor.
	patientWait = 6 * clock.TicksPerHour
	// painInjury: injuries this bad hurt (mood) and send the colonist to bed.
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
	return w.At != 0 && v.Injury > 0 || w.Infection > 0
}

// otherDoctor reports a standing colonist besides id who does Doctor work.
func (g *Game) otherDoctor(id entity.EntityID) bool {
	for _, pid := range g.pawnIDs {
		if pid != id && g.standingPawn(pid) && g.prioOf(pid)[work.Doctor] > 0 {
			return true
		}
	}
	return false
}

// patientRest: a sick or badly hurt colonist should lie down for a doctor.
func (g *Game) patientRest(id entity.EntityID) bool {
	if !g.needsTending(id) || !g.otherDoctor(id) {
		return false
	}
	v, _ := g.vital.Get(id)
	return g.wounds[id].Infection >= infection.Sick || v.Injury >= painInjury
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

// withMedicine adds a fetch of herbal medicine to a tend job, if there is
// some to be had.
func (g *Game) withMedicine(id entity.EntityID, rec job.Record) job.Record {
	sid, st := g.ingredientFor(id, recipe.Input{Kind: item.HerbalMedicine, Qty: 1, Max: 1})
	if sid == 0 {
		return rec
	}
	if _, taken := g.reserve.EntityOwner(sid); taken {
		return rec
	}
	rec.Dest, rec.DestX, rec.DestY = sid, rec.TX, rec.TY
	rec.TX, rec.TY = st.X, st.Y
	return rec
}

// pickUpMedicine: the doctor reached the medicine; take one dose and head
// for the patient. Reports false if the job had to end.
func (g *Game) pickUpMedicine(id entity.EntityID, j *job.Record) bool {
	st, ok := g.stacks.Get(j.Dest)
	if !ok || st.Carrier != 0 || st.Kind != item.HerbalMedicine {
		j.Dest = 0 // gone: tend without it
	} else {
		carry := j.Dest
		if st.Qty > 1 {
			st.Qty--
			g.stacks.Set(j.Dest, st)
			carry = g.entities.Next()
			g.itemIDs = append(g.itemIDs, carry)
			g.stacks.Set(carry, item.Stack{Kind: item.HerbalMedicine, Qty: 1, X: st.X, Y: st.Y, Carrier: id})
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
		return infection.Medicine
	case doctor == patient:
		return infection.SelfTended
	}
	return infection.Tended
}

// tendHeal is the injury a tend mends at once.
func tendHeal(c infection.Care) int {
	switch c {
	case infection.Medicine:
		return TendHeal * 2
	case infection.SelfTended:
		return TendHeal / 2
	}
	return TendHeal
}

// closeWound: a tend closes an open wound (it may still fester).
func (g *Game) closeWound(id entity.EntityID, c infection.Care) {
	w := g.wounds[id]
	if w.At != 0 && w.Infection == 0 && g.rng.IntN(100) < infection.Chance(c) {
		w.Infection = infection.Onset
		pos, _ := g.positions.Get(id)
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
			if w.Infection == 0 && g.rng.IntN(100) < infection.Chance(infection.Untreated) {
				w.Infection = infection.Onset
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
	w := g.wounds[id]
	w.Infection = severity
	g.wounds[id] = w
}
