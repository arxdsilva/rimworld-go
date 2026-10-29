package game

import (
	"slices"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/room"
)

// Wounds, healing and infection (MED-02). Every fresh wound that can
// fester gets an infection roll 6–18 hours later (Wound.InfectAt): its
// chance falls with the tend quality and halves in a clean room
// (infection.WoundChance), scaled by the difficulty. An infection is a
// condition on the wound's part racing immunity (infection.StepRace); a
// tend slows it for 12 hours. Once an hour the medical phase walks the
// patients index: infection rolls, the races, then every open wound heals
// at the wiki rates (faster tended and lying down) and may scar. Rolls use
// the medical stream (scars the body stream), never g.rng. Colonists,
// prisoners and raiders are all treated alike. Sick, hurt or untended
// colonists rest in bed until they are well again (care/rest.go); with no
// one else to help, a colonist tends itself.

// legacyWound is the per-pawn wound record saves up to version 5 wrote
// (Wounds, read only): an open wound since At and an infection's severity
// in percent. migrateWoundInfections turns it into conditions.
type legacyWound struct {
	At        uint64
	Infection int
	Care      uint8
}

const (
	// patientWait caps how long a patient lies in bed waiting for a
	// surgeon (rest.Sleep.done, surgeryGaveUp).
	patientWait = 6 * clock.TicksPerHour
	// painInjury: injuries this bad hurt (mood) and send the colonist to
	// bed (care/rest.go).
	painInjury = care.PainInjury
	// cleanRoomRoofed is the roofed share (permille) a room needs to
	// count as indoors for the infection room factor (original tuning).
	cleanRoomRoofed = 750
	// noMedicine is the kind a tend without medicine passes on: anything
	// that is not medicine tends at body.Potency's "none".
	noMedicine = item.Food
)

// woundOpened schedules the infection roll of each wound humanlike id
// took this tick that can fester: 6–18 hours from now (medical stream).
func (g *Game) woundOpened(id entity.EntityID) {
	b := g.bodies[id]
	if b == nil {
		return
	}
	now := uint64(g.clock.Now())
	for i := range b.Wounds {
		w := &b.Wounds[i]
		if w.At != now || w.InfectAt != 0 || w.Permanent || infection.WoundChance(w.Kind, -1, infection.RoomOther) == 0 {
			continue
		}
		h := infection.DelayMin + g.stream(streamMedical).IntN(infection.DelaySpan+1)
		w.InfectAt = now + uint64(h)*clock.TicksPerHour
	}
}

// needsTending reports living humanlike id with something a tend would
// help now (body.TendDue: an untended wound or stump, or a condition due
// its next tend), or Injury set past its body from outside. A pawn never
// hurt since an older save, with no body yet, needs one while it is hurt
// and not tended.
func (g *Game) needsTending(id entity.EntityID) bool {
	v, ok := g.vital.Get(id)
	if !ok || v.Status == health.Dead || !g.isPawn(id) {
		return false
	}
	if b := g.bodies[id]; b != nil {
		return b.TendDue(uint64(g.clock.Now())) || v.Injury > b.Damage()
	}
	return v.Injury > 0 && !g.tended(id)
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

// tendMedicine is the medicine a finished tend uses: the dose the job
// carries, or none.
func (g *Game) tendMedicine(j job.Record) item.Kind {
	if j.Carry != 0 {
		if st, ok := g.stacks.Get(j.Carry); ok && isMedicine(st.Kind) {
			return st.Kind
		}
	}
	return noMedicine
}

// tendQualityFor rolls the quality (permille) of doctor's tend on
// patient with medicine med (body.TendQuality): its Medicine level,
// manipulation and sight, the hospital bed offset and self-tending, on
// the medical stream.
func (g *Game) tendQualityFor(doctor, patient entity.EntityID, med item.Kind) int32 {
	in := body.TendInput{Skill: int(g.skillOf(doctor, skill.Medicine).Level), Manip: 1000, Sight: 1000, Medicine: med,
		Self: doctor == patient, Roll: g.stream(streamMedical).IntN}
	if b := g.bodies[doctor]; b != nil {
		in.Manip, in.Sight = b.Capacity(body.Manipulation), b.Capacity(body.Sight)
	}
	if g.inHospitalBed(patient) {
		in.BedOffset = hospitalTendOffset // items_ng1.go
	}
	in.BedOffset += g.monitorTend(patient) // surgery.go: a powered vitals monitor (MED-06)
	return body.TendQuality(in)
}

// roomInfectionFactor is the infection room factor (permille) where id
// lies: a roofed, fully floored room without filth halves the chance;
// outdoors or anywhere else it stays whole. wiki: Infection.
func (g *Game) roomInfectionFactor(id entity.EntityID) int32 {
	pos, _ := g.positions.Get(id)
	r := g.roomAt(pos.X, pos.Y)
	if r == room.Outdoors || r < 0 || r >= len(g.rooms) {
		return infection.RoomOther
	}
	c := g.rooms[r].contents
	if sum, _ := g.roomFilth(r); sum > 0 || c.Floored < c.Cells || g.roomRoofed(r) < cleanRoomRoofed {
		return infection.RoomOther
	}
	return infection.RoomClean
}

// stepCare runs an hour of care on every patient in id order (from
// phaseMedical, after the blood): due infection rolls, the condition
// races, then an hour of healing on every open wound; the downing rule
// follows the body.
func (g *Game) stepCare(now uint64) {
	if len(g.med.patients) == 0 {
		return
	}
	for _, id := range slices.Clone(g.med.patients) { // a death touches the index
		v, ok := g.vital.Get(id)
		if g.bodies[id] == nil || !ok || v.Status == health.Dead {
			continue
		}
		b := g.ensureBody(id, &v) // body.go: an Injury set from outside is reconciled, not overwritten
		g.rollInfections(id, b, now)
		if g.stepConds(id, b, now) {
			continue // died
		}
		before := len(b.Wounds)
		scarred := b.HealHour(g.stream(streamBody).IntN, g.lyingHeal(id)) // body/tend.go
		for _, p := range scarred {
			g.logEvent(id, logEntry{Kind: logScarred, Name: "A wound on the " + p.Label() + " healed into a scar"})
		}
		if len(scarred) > 0 || len(b.Wounds) != before || b.Damage() != v.Injury {
			g.bodyChanged(id, b, &v) // body.go: Injury follows the wounds
		}
		g.updateDowned(id, b, &v) // bleed.go
	}
}

// lyingHeal is the healing a patient gets from how it lies (milli-HP per
// day per wound): a hospital bed, another bed, the ground, or none while
// up. wiki: Injury.
func (g *Game) lyingHeal(id entity.EntityID) int32 {
	if !g.care.Doctor.Lying(id) {
		return 0
	}
	switch {
	case g.inHospitalBed(id):
		return body.LieHospital
	case g.inBed(id):
		return body.LieBed
	}
	return body.LieGround
}

// rollInfections rolls every wound of id whose infection roll is due:
// the wound's chance (its tend quality, the room) scaled by the
// difficulty, on the medical stream. A festering wound gives its part an
// infection (one per part).
func (g *Game) rollInfections(id entity.EntityID, b *body.Body, now uint64) {
	for i := range b.Wounds {
		w := &b.Wounds[i]
		if w.InfectAt == 0 || now < w.InfectAt {
			continue
		}
		w.InfectAt = 0
		if w.Permanent || w.Dmg <= 0 {
			continue
		}
		tq := int32(-1)
		if w.Tended {
			tq = int32(w.TendQ)
		}
		p := g.infectionChance(infection.WoundChance(w.Kind, tq, g.roomInfectionFactor(id))) // story.go: difficulty
		if p <= 0 || int32(g.stream(streamMedical).IntN(1000)) >= p {
			continue
		}
		if b.AddCond(body.Condition{Kind: body.CondInfection, Part: w.Part, Sev: infection.Onset, Since: now}) {
			pos, _ := g.positions.Get(id)
			g.logEvent(id, logEntry{Kind: logInfected})
			g.message(pos.X, pos.Y, "%s's wound on the %s is infected.", g.name(id), w.Part.Label())
		}
	}
}

// immunityGain is id's immunity gain (permille; vitals monitors and
// organs change it from MED-05/06).
func (g *Game) immunityGain(id entity.EntityID) int32 {
	return g.organImmunityGain(id) + g.monitorImmunityGain(id) // organs.go (MED-05), surgery.go (MED-06)
}

// stepConds advances an hour of every condition race on id; it reports
// id died of one. A beaten condition is removed.
func (g *Game) stepConds(id entity.EntityID, b *body.Body, now uint64) bool {
	if len(b.Conds) == 0 {
		return false
	}
	pos, _ := g.positions.Get(id)
	beaten := false
	for i := range b.Conds {
		c := &b.Conds[i]
		if body.IsDisease(c.Kind) {
			continue // disease.go steps the diseases (MED-04)
		}
		d := body.DefOfCond(c.Kind)
		dead, cured := infection.StepRace(c, d, now < c.TendUntil, g.immunityGain(id), 1)
		switch {
		case dead:
			g.message(pos.X, pos.Y, "%s died of %s.", g.name(id), d.Label)
			g.dieMedical(id, health.CauseInfection) // caravan_medical.go
			g.clearPatient(id)
			return true
		case cured:
			c.Sev, beaten = 0, true
			g.logEvent(id, logEntry{Kind: logInfectionCleared})
			g.message(pos.X, pos.Y, "%s fought off the %s.", g.name(id), d.Label)
		}
	}
	if beaten {
		b.Conds = slices.DeleteFunc(b.Conds, func(c body.Condition) bool { return c.Sev <= 0 })
		if len(b.Conds) == 0 {
			b.Conds = nil
		}
	}
	return false
}

// worstInfection is id's most severe infection (false: none).
func (g *Game) worstInfection(id entity.EntityID) (body.Condition, bool) {
	b := g.bodies[id]
	if b == nil {
		return body.Condition{}, false
	}
	var worst body.Condition
	found := false
	for _, c := range b.Conds {
		if c.Kind == body.CondInfection && (!found || c.Sev > worst.Sev) {
			worst, found = c, true
		}
	}
	return worst, found
}

// sick reports id fighting a condition it is not yet immune to (mood,
// bed rest): it starts with the condition and ends once immunity wins,
// which only ever grows, so it never flaps.
func (g *Game) sick(id entity.EntityID) bool {
	b := g.bodies[id]
	if b == nil {
		return false
	}
	for _, c := range b.Conds {
		if d := body.DefOfCond(c.Kind); !d.NoImmunity && c.Immunity < infection.Full {
			return true
		}
	}
	return false
}

// infectionLosing reports one of id's conditions on course to kill
// before immunity wins (infection.Losing at today's tend).
func (g *Game) infectionLosing(id entity.EntityID) bool {
	b := g.bodies[id]
	if b == nil {
		return false
	}
	now := uint64(g.clock.Now())
	for _, c := range b.Conds {
		if infection.Losing(c, body.DefOfCond(c.Kind), now < c.TendUntil, g.immunityGain(id)) {
			return true
		}
	}
	return false
}

// condDeathHours is the hours before id's soonest losing condition kills
// at today's rates (-1: none losing).
func (g *Game) condDeathHours(id entity.EntityID, b *body.Body) int {
	best := -1
	now := uint64(g.clock.Now())
	for _, c := range b.Conds {
		if h := infection.HoursToDeath(c, body.DefOfCond(c.Kind), now < c.TendUntil, g.immunityGain(id)); h >= 0 && (best < 0 || h < best) {
			best = h
		}
	}
	return best
}

// untendedHours is how long id's oldest untended open wound has waited
// (whole hours; 0: none, or under an hour).
func (g *Game) untendedHours(id entity.EntityID) int {
	b := g.bodies[id]
	if b == nil {
		return 0
	}
	now := uint64(g.clock.Now())
	oldest := now
	for _, w := range b.Wounds {
		if !w.Permanent && !w.Tended && w.At < oldest {
			oldest = w.At
		}
	}
	return int((now - oldest) / clock.TicksPerHour)
}

// woundView is id's infection (percent of the worst; 0: none) and
// whether it has an open wound left untended (PawnView.Infection and
// Wounded).
func (g *Game) woundView(id entity.EntityID) (infPct int, wounded bool) {
	if c, ok := g.worstInfection(id); ok {
		infPct = max(int(c.Sev)/10, 1)
	}
	b := g.bodies[id]
	if b == nil {
		return infPct, false
	}
	for _, w := range b.Wounds {
		if !w.Permanent && !w.Tended {
			return infPct, true
		}
	}
	return infPct, b.Fresh != 0
}

// InfectionOf is id's worst infection severity in percent (0: none).
func (g *Game) InfectionOf(id entity.EntityID) int {
	inf, _ := g.woundView(id)
	return inf
}

// Infect gives humanlike id an infection of severity percent on its
// worst open wound's part (the torso when it has none), or sets the
// severity of the one already there (dev scenarios and fixtures).
func (g *Game) Infect(id entity.EntityID, severity int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	v, ok := g.vital.Get(id)
	if !ok {
		return
	}
	b := g.ensureBody(id, &v)
	if b == nil {
		return
	}
	g.infectBody(b, severity*10, uint64(g.clock.Now()))
	g.bodyChanged(id, b, &v)
}

// infectBody puts an infection of severity sev (permille) on b's worst
// open wound's part (the torso when none), or sets the one there.
func (g *Game) infectBody(b *body.Body, sev int, now uint64) {
	part := body.Torso
	worst := 0
	for _, w := range b.Wounds {
		if !w.Permanent && w.Dmg > worst {
			part, worst = w.Part, w.Dmg
		}
	}
	sev = min(max(sev, 1), infection.Full-1)
	for i := range b.Conds {
		if b.Conds[i].Kind == body.CondInfection && b.Conds[i].Part == part {
			b.Conds[i].Sev = int32(sev)
			return
		}
	}
	d := body.DefOfCond(body.CondInfection)
	b.AddCond(body.Condition{Kind: body.CondInfection, Part: part, Sev: int32(sev), Since: now, Stage: d.StageAt(int32(sev))})
}

// migrateWoundInfections (save v6, MED-02) turns each legacy per-pawn
// infection of an older save (Wounds) into an infection condition on the
// pawn's worst open wound, with no immunity yet; the legacy open-wound
// window has no counterpart (its wounds keep no infection roll).
func (g *Game) migrateWoundInfections() {
	legacy := g.med.legacyWounds
	g.med.legacyWounds = nil
	if len(legacy) == 0 {
		return
	}
	ids := make([]entity.EntityID, 0, len(legacy))
	for id := range legacy {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	now := uint64(g.clock.Now())
	for _, id := range ids {
		w := legacy[id]
		v, ok := g.vital.Get(id)
		if w.Infection <= 0 || !ok || v.Status == health.Dead {
			continue
		}
		b := g.ensureBody(id, &v)
		if b == nil {
			continue
		}
		g.infectBody(b, w.Infection*10, now)
		g.bodyChanged(id, b, &v)
	}
}
