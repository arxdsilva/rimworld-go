package game

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Animal medicine (MED-09). A wounded animal gets a lightweight body
// (animal.State.Body): the humanlike body model with every wound on one
// virtual part (animalPart), no species anatomy. It is made on the first
// wound and dropped once nothing is left to treat, so an unhurt animal
// carries none and saves stay as they were.
//
// The body reuses the humanlike rules: open cuts, bites and scratches
// bleed (body.BleedRate) until tended, blood loss kills at 1000; each
// wound that can fester rolls an infection 6–18 hours later
// (infection.WoundChance), which races immunity (infection.StepRace) and
// kills at full severity; wounds heal at the wiki rates (body.HealHour,
// faster tended and asleep). Bleeding is scaled by the species' size:
// RimWorld divides blood loss by body size; ours have none, so hit points
// against a human's (health.MaxHP) stand in.
//
// Injury stays the hit point scalar everything else reads (death at
// MaxHP, fleeing, the Health bar). The wounds are the treatable part of
// it and never add up to more (reconcileAnimalBody): a wound adds to
// both, healing a wound takes it off Injury, and Injury healed another way
// (a tame animal's natural heal) trims the smallest open wounds. A vet's
// tend works on the wounds (tendAnimalBody). Animals have no downed state, so blood loss and infections
// kill but do not down.
//
// Once an hour the medical phase (phaseMedical, bleed.go) walks the
// animal patients (sorted ids of animals with a body; derived, rebuilt
// on first use after a load), never every animal. Rolls use the medical
// stream. Scars are left out (no anatomy to scar), so healing draws no
// random numbers.

// animalPart is the virtual part every animal wound sits on.
const animalPart = body.Torso

// Animal health scale bounds (permille of a human's hit points): the
// smallest species bleeds at most 10x a human's rate, the largest at
// least a tenth of it (original tuning).
const (
	animalScaleMin = 100
	animalScaleMax = 10000
)

// animalHealthScale is species s's body size against a human's, from its
// hit points (permille).
func animalHealthScale(s animal.Species) int32 {
	return min(max(int32(animal.DefOf(s).MaxHP)*1000/health.MaxHP, animalScaleMin), animalScaleMax)
}

// animalBleedPerDay is how fast an animal of species s with body b bleeds
// (permille of its blood per day): the body's bleeding scaled by its size.
func animalBleedPerDay(b *body.Body, s animal.Species) int32 {
	if b == nil {
		return 0
	}
	n := b.BleedPerDay()
	if n <= 0 {
		return 0
	}
	return max(n*1000/animalHealthScale(s), 1)
}

// hourShare is a daily permille rate's share of one hour, rounded to
// nearest, never below 1 for a positive rate (body.StepBlood's rounding).
func hourShare(perDay int32) int32 {
	if perDay <= 0 {
		return 0
	}
	return max((perDay+12)/24, 1)
}

// stepAnimalBlood advances an hour of blood loss on body b of species s:
// the scaled bleeding, or the regeneration while nothing bleeds; the
// stage follows with its hysteresis (body.BloodStage).
func stepAnimalBlood(b *body.Body, s animal.Species) {
	if bleed := animalBleedPerDay(b, s); bleed > 0 {
		b.BloodLoss = min(b.BloodLoss+hourShare(bleed), body.BloodLossDeath)
	} else if b.BloodLoss > 0 {
		b.BloodLoss = max(b.BloodLoss-hourShare(body.BloodRegenPerDay), 0)
	}
	b.BloodStage = body.BloodStage(b.BloodLoss, b.BloodStage)
}

// animalBleedDeathHours is how many hours an animal of species s with
// body b has before bleeding out at its current rate (-1: not bleeding).
func animalBleedDeathHours(b *body.Body, s animal.Species) int {
	bleed := animalBleedPerDay(b, s)
	if bleed <= 0 {
		return -1
	}
	left := max(body.BloodLossDeath-b.BloodLoss, 0)
	h := hourShare(bleed)
	return int((left + h - 1) / h)
}

// woundAnimal gives animal id (state a, Injury already raised by dmg) a
// wound of dmg: of kind, or the attacker by's weapon's kind. It makes the
// body on the first wound, schedules the wound's infection roll and adds
// the animal to the patients. A killing blow leaves no wound.
func (g *Game) woundAnimal(id entity.EntityID, a *animal.State, dmg int, kind body.WoundKind, by entity.EntityID, now uint64) {
	if dmg <= 0 || a.Dead() {
		return
	}
	if kind == woundByAttacker {
		kind = g.woundKindBy(by) // body.go
	}
	if a.Body == nil {
		a.Body = &body.Body{}
	}
	w := body.Wound{Part: animalPart, Dmg: dmg, Kind: kind, At: now, Size: int32(dmg)}
	if infection.WoundChance(kind, -1, infection.RoomOther) > 0 {
		h := infection.DelayMin + g.stream(streamMedical).IntN(infection.DelaySpan+1)
		w.InfectAt = now + uint64(h)*clock.TicksPerHour
	}
	a.Body.Wounds = append(a.Body.Wounds, w)
	reconcileAnimalBody(a)
	if a.Body != nil {
		g.markAnimalPatient(id)
	}
}

// reconcileAnimalBody keeps a's wounds within its Injury (trimming the
// smallest open wounds when Injury healed another way) and drops the body
// once nothing on it needs care.
func reconcileAnimalBody(a *animal.State) {
	b := a.Body
	if b == nil {
		return
	}
	if d := b.Damage(); d > a.Injury {
		trimOpen(b, d-max(a.Injury, 0)) // body.go
	}
	if !needsCare(b) { // patients.go
		a.Body = nil
	}
}

// animalTendDue reports animal id (state a) with something a tend would
// help at tick now: with a body, an untended wound or an infection due its
// next tend (body.TendDue: one tend per window); without one (an older
// save, starvation), any injury not tended within TendTicks.
func (g *Game) animalTendDue(id entity.EntityID, a *animal.State, now uint64) bool {
	if a.Body != nil {
		return a.Body.TendDue(now)
	}
	return a.Injury > 0 && !g.tended(id)
}

// tendAnimalBody applies a vet's tend of quality q to animal a at tick
// now. With a body, every open wound and due infection on it is tended:
// the bleeding stops, the wounds heal faster by the hour and fester less
// (as a colonist's). Without one (an older save, starvation) Injury mends
// at once by animalTendHeal (medical.go), the scalar model's tend.
func tendAnimalBody(a *animal.State, q int32, now uint64) {
	if a.Body == nil {
		a.Injury = max(0, a.Injury-animalTendHeal(q))
		return
	}
	a.Body.TendAll(q, now)
	reconcileAnimalBody(a)
}

// stepAnimalHealth runs an hour of animal medicine (from phaseMedical)
// on every animal patient in id order, then drops the healed and the gone.
func (g *Game) stepAnimalHealth(now uint64) {
	ids := g.animalPatientIDs()
	if len(ids) == 0 {
		return
	}
	for _, id := range slices.Clone(ids) { // a death touches the stores
		a, ok := g.animals.Get(id)
		if !ok || a.Body == nil || a.Dead() {
			continue
		}
		if g.stepAnimalBody(id, &a, now) {
			continue // died
		}
		g.animals.Set(id, a)
	}
	g.sweepAnimalPatients()
}

// stepAnimalBody is one hour on animal id's body: blood loss, due
// infection rolls, the infection races, then healing. It reports the
// animal died.
func (g *Game) stepAnimalBody(id entity.EntityID, a *animal.State, now uint64) bool {
	b := a.Body
	stepAnimalBlood(b, a.Species)
	if b.BloodLoss >= body.BloodLossDeath {
		g.animalMedicalDeath(id, a)
		return true
	}
	g.rollAnimalInfections(id, a, b, now)
	if g.stepAnimalConds(id, a, b, now) {
		return true
	}
	lying := int32(0)
	if sleeping(*a) { // animal_needs.go
		lying = body.LieGround
	}
	before := b.Damage()
	b.HealHour(noScarRoll, lying)
	a.Injury = max(0, a.Injury-(before-b.Damage()))
	reconcileAnimalBody(a)
	return false
}

// noScarRoll is the scar roll animal healing passes: never a scar (an
// animal's one virtual part has nothing to scar).
func noScarRoll(int) int { return 1 << 20 }

// animalMedicalDeath kills animal id (state a) of its wounds, blood loss
// or an infection, with no killer (killAnimal says a tame one has died).
func (g *Game) animalMedicalDeath(id entity.EntityID, a *animal.State) {
	g.animals.Set(id, *a)
	g.killAnimal(id, 0) // animals.go
}

// rollAnimalInfections rolls every wound of animal id whose infection
// roll is due: the wound's chance from its tend quality and the room,
// scaled by the difficulty (as rollInfections, infection.go).
func (g *Game) rollAnimalInfections(id entity.EntityID, a *animal.State, b *body.Body, now uint64) {
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
		p := g.infectionChance(infection.WoundChance(w.Kind, tq, g.roomInfectionFactor(id))) // story.go, infection.go
		if p <= 0 || int32(g.stream(streamMedical).IntN(1000)) >= p {
			continue
		}
		if b.AddCond(body.Condition{Kind: body.CondInfection, Part: animalPart, Sev: infection.Onset, Since: now}) {
			g.logEvent(id, logEntry{Kind: logInfected})
			if a.Tame {
				pos, _ := g.positions.Get(id)
				g.message(pos.X, pos.Y, "%s's wound is infected.", a.Name)
			}
		}
	}
}

// stepAnimalConds advances an hour of every condition race on animal id
// (normal immunity: no organs or monitors); it reports the animal died. A
// beaten condition is removed.
func (g *Game) stepAnimalConds(id entity.EntityID, a *animal.State, b *body.Body, now uint64) bool {
	if len(b.Conds) == 0 {
		return false
	}
	beaten := false
	for i := range b.Conds {
		c := &b.Conds[i]
		if body.IsDisease(c.Kind) {
			continue
		}
		d := body.DefOfCond(c.Kind)
		dead, cured := infection.StepRace(c, d, now < c.TendUntil, 1000, 1)
		switch {
		case dead:
			g.animalMedicalDeath(id, a)
			return true
		case cured:
			c.Sev, beaten = 0, true
			g.logEvent(id, logEntry{Kind: logInfectionCleared})
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

// ---- the animal patients index ----

// animalPatientIDs is the sorted ids of animals with a body, built from
// the animals on first use (a new game, or after a load reset the care
// state).
func (g *Game) animalPatientIDs() []entity.EntityID {
	c := &g.care
	if !c.animalIdxOK {
		c.animalPatients = c.animalPatients[:0]
		for _, id := range g.animalIDs {
			if a := g.animals.Ref(id); a != nil && a.Body != nil {
				c.animalPatients = append(c.animalPatients, id)
			}
		}
		slices.Sort(c.animalPatients)
		c.animalIdxOK = true
	}
	return c.animalPatients
}

// markAnimalPatient adds animal id to the index (once).
func (g *Game) markAnimalPatient(id entity.EntityID) {
	ids := g.animalPatientIDs()
	if i, found := slices.BinarySearch(ids, id); !found {
		g.care.animalPatients = slices.Insert(ids, i, id)
	}
}

// sweepAnimalPatients drops every indexed animal that is gone or has no
// body left.
func (g *Game) sweepAnimalPatients() {
	ids := g.animalPatientIDs()
	kept := ids[:0]
	for _, id := range ids {
		if a := g.animals.Ref(id); a != nil && a.Body != nil {
			kept = append(kept, id)
		}
	}
	clear(ids[len(kept):])
	g.care.animalPatients = kept
}

// ---- the read model ----

// fillAnimalHealth fills an animal view's health block (MED-09): the
// headline, bleeding, blood loss, the worst infection, whether a tend is
// due, and the care level.
func (g *Game) fillAnimalHealth(av *AnimalView, id entity.EntityID, a animal.State, now uint64) {
	av.CareLevel = uint8(g.careOf(id)) // care.go
	av.BleedDeathHours = -1
	av.HealthState = "Healthy"
	if a.Injury > 0 {
		av.HealthState = "Injured"
	}
	b := a.Body
	if b == nil {
		av.NeedsTend = a.Tame && a.Injury > 0 && !g.tended(id)
		return
	}
	av.NeedsTend = a.Tame && b.TendDue(now)
	av.BleedPerDay = int(animalBleedPerDay(b, a.Species))
	av.BleedDeathHours = animalBleedDeathHours(b, a.Species)
	av.BloodLossPct = int(b.BloodLoss) / 10
	av.BloodStage = body.BloodStageLabel(b.BloodStage)
	for _, c := range b.Conds {
		if c.Kind == body.CondInfection && (av.InfectionPct == 0 || int(c.Sev)/10 > av.InfectionPct) {
			av.InfectionPct = max(int(c.Sev)/10, 1)
			av.ImmunityPct = int(c.Immunity) / 10
			av.InfectionLosing = infection.Losing(c, body.DefOfCond(c.Kind), now < c.TendUntil, 1000)
		}
	}
	if av.BleedPerDay > 0 {
		av.HealthState = fmt.Sprintf("Bleeding %d%%/day · death in %dh", av.BleedPerDay/10, av.BleedDeathHours)
	}
}
