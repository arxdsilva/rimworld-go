// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"

	"rimworld-go/internal/application/game/animalview"
	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Animal medicine (MED-09/MED-13). Animals use species-specific body
// anatomy and sizes. Hits create wounds and can remove parts, slow movement,
// bleed or infect. Injury remains the authoritative scalar for downing and
// fleeing; the body holds the treatable wounds, scars and infection state.
// Hourly patient steps heal and treat a small index of wounded animals.
// Old saves migrate virtual body parts to the species' anatomy.

// A tame animal can be rescued to a species bed; wild animals remain
// outside the colony's doctor roster. Recovered animals release temporary
// bed stays, and dying animals clear carrier and reservation records.

// woundAnimal gives animal id (state a, Injury already raised by dmg) a
// blow stopped at the scalar HP it had before the hit (SpreadBlowWithin).
// Only newly applied damage raises Injury; historical part loss is not
// fresh damage. It makes the body, schedules infection and marks the patient.
// A base blow that kills leaves no wound; a lethal split draws no infection delay.
func (g *Game) woundAnimal(id entity.EntityID, a *animal.State, dmg int, kind body.WoundKind, split body.Split, by entity.EntityID, now uint64) {
	if dmg <= 0 || a.Dead() || animal.IsMech(a.Species) { // DEF-4: a machine takes damage, not wounds
		return
	}
	if a.Body == nil {
		a.Body = animal.NewBody(a.Species)
	}
	b := a.Body
	a.Injury += body.SpreadBlowWithin(b, dmg, kind, split, a.HP()+dmg, g.stream(streamMedical).IntN, now) - dmg // the medical stream; never a fall
	if infection.WoundChance(kind, -1, infection.RoomOther) > 0 && !a.Dead() {
		h := infection.DelayMin + g.stream(streamMedical).IntN(infection.DelaySpan+1)
		for i := range b.Wounds { // the wounds this hit opened
			if w := &b.Wounds[i]; w.At == now && w.InfectAt == 0 && !w.Permanent && !w.Tended {
				w.InfectAt = now + uint64(h)*clock.TicksPerHour
			}
		}
	}
	animal.ReconcileBody(a)
	if a.Body != nil && !a.Dead() {
		g.markAnimalPatient(id)
	}
}

// animalPatient reports an animal the hourly medical step must walk: it
// has a body that needs care (patients.go). A body kept only for its
// lost parts needs none.
func animalPatient(a *animal.Live) bool { return a.Body != nil && needsCare(a.Body) }

// migrateAnimalBodies gives every animal body saved before MED-13 (the
// humanlike model with no Scale: every wound on one virtual part) its
// species' anatomy, moving its wounds and conditions to the root part
// (the torso, a snake's body, an insect's thorax). Run once on load.
func (g *Game) migrateAnimalBodies() {
	for _, id := range g.animalIDs {
		a := g.animals.Ref(id)
		if a == nil || a.Body == nil {
			continue
		}
		body.MigrateReplacements(a.Body)
		if a.Body.Scale != 0 || a.Body.Anatomy != body.Humanlike {
			continue
		}
		animal.MigrateBody(a)
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
	animal.ReconcileBody(a)
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
	animal.StepBlood(b, a.Species)
	if b.BloodLoss >= body.BloodLossDeath {
		g.animalMedicalDeath(id, a)
		return true
	}
	g.rollAnimalInfections(id, a, b, now)
	if g.stepAnimalConds(id, a, b, now) {
		return true
	}
	if b.HasAging() {
		b.StepAging(now, g.agingSalt(id), g.stream(streamMedical).IntN)
	}
	lying := int32(0)
	if lyingDown(*a) {
		lying = body.LieGround
		if a.Tame && g.rest.Animals.MedicalLying(id) {
			lying = body.LieBed
		}
	}
	before := body.OpenWoundDamage(b)
	b.HealHour(noScarRoll, lying)
	a.Injury = max(0, a.Injury-(before-body.OpenWoundDamage(b)))
	animal.ReconcileBody(a)
	g.checkAnimalDowned(id, a, now)
	return false
}

// Downing uses the domain's injury and blood-loss hysteresis. A downed
// animal lies still while bleeding, healing and starving; predators can
// hunt it and doctors can tend tame animals. StandsUp restores movement.

// lyingDown reports an animal lying on the ground: asleep, or downed. It
// heals as a sleeper and its rest refills.
func lyingDown(a animal.State) bool { return a.Downed || sleeping(a) } // animal_needs.go

// checkAnimalDowned downs animal id (state a) when it is now too hurt or
// bled to stand, or stands a downed one well enough again. It reports a
// change.
func (g *Game) checkAnimalDowned(id entity.EntityID, a *animal.State, now uint64) bool {
	switch {
	case a.DownsNow():
		g.downAnimal(id, a, now)
		return true
	case a.StandsUp():
		a.Downed, a.DownedTick = false, 0
		g.care.Rest.RecoveredAnimal(id)
		a.Mode, a.Path, a.PathI = animal.Wander, nil, 0
		a.NextDecide = now // it thinks again at once
		return true
	}
	return false
}

// downAnimal puts animal id (state a) down at tick now: it drops whatever
// it was doing (a hunt, a flight, a meal, a charge, a manhunter's chase)
// and lies where it is.
func (g *Game) downAnimal(id entity.EntityID, a *animal.State, now uint64) {
	a.Downed, a.DownedTick = true, now
	a.Mode, a.Path, a.PathI = animal.Wander, nil, 0
	a.Target, a.HuntSince, a.FleeUntil, a.ChargeSince = 0, 0, 0, 0
	a.Feed, a.FeedLeft = animal.FeedTarget{}, 0
	g.logEvent(id, logEntry{Kind: evlog.Downed})
	if a.Tame && a.Name != "" {
		pos, _ := g.positions.Get(id)
		g.message(pos.X, pos.Y, "%s the %s is down.", a.Name, a.Species.Label())
	}
}

// downedThink is a downed animal's tick in thinkAnimal: it lies still,
// and on its decision cycle a tame one heals naturally and it is checked
// for standing up. A standing animal is checked for going down on its
// decision cycle too (starvation hurts outside combat). It reports the
// animal is (still) down: thinkAnimal does nothing else.
func (g *Game) downedThink(id entity.EntityID, a *animal.State, now uint64) bool {
	if now >= a.NextDecide {
		if !a.Downed {
			if !a.DownsNow() {
				return false
			}
			g.downAnimal(id, a, now)
		}
		a.NextDecide = animal.NextDecision(uint64(id), now) // animal_needs.go
		if a.Tame && (now+uint64(id))%animal.DecideEvery == 0 {
			g.naturalHeal(id, a, now) // vet_animals.go
		}
		if g.checkAnimalDowned(id, a, now) && !a.Downed {
			return false // up again: it decides at once
		}
	}
	if !a.Downed {
		return false
	}
	// whatever was set on it from outside (a pack's call, a rally), it
	// lies still
	a.Mode, a.Target, a.Path, a.PathI = animal.Wander, 0, nil, 0
	return true
}

// noScarRoll is the scar roll animal healing passes: never a scar
// (animals heal clean; it keeps healing free of random draws).
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
		if b.AddCond(body.Condition{Kind: body.CondInfection, Part: w.Part, Sev: infection.Onset, Since: now}) {
			g.logEvent(id, logEntry{Kind: evlog.Infected})
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
		if body.IsDisease(c.Kind) || body.IsAging(c.Kind) {
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
			g.logEvent(id, logEntry{Kind: evlog.InfectionCleared})
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

// animalPatientIDs is the sorted ids of animals with a body that needs
// care (animalPatient), built from every map's animals on first use (a new
// game, or after a load reset the care state). The index is colony-wide
// (MAP-0): each map's pass steps the ones standing on it.
func (g *Game) animalPatientIDs() []entity.EntityID {
	c := &g.med
	if !c.animalIdxOK {
		c.animalPatients = c.animalPatients[:0]
		for _, m := range g.maps {
			for _, id := range m.animalIDs {
				if a := m.animals.Ref(id); a != nil && animalPatient(a) {
					c.animalPatients = append(c.animalPatients, id)
				}
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
		g.med.animalPatients = slices.Insert(ids, i, id)
	}
}

// sweepAnimalPatients drops every indexed animal that is gone or has no
// body needing care left; another map's animals wait for its pass (owns).
func (g *Game) sweepAnimalPatients() {
	ids := g.animalPatientIDs()
	kept := ids[:0]
	for _, id := range ids {
		if a := g.animals.Ref(id); a != nil && animalPatient(a) || !owns(g, id) {
			kept = append(kept, id)
		}
	}
	clear(ids[len(kept):])
	g.med.animalPatients = kept
}

// ---- the read model ----

// AnimalPartView is one hurt or lost part of an animal (MED-13,
// game/animalview).
type AnimalPartView = animalview.Part

// fillAnimalHealth provides the colony-owned care facts to the view owner.
func (g *Game) fillAnimalHealth(av *AnimalView, id entity.EntityID, a animal.State, now uint64) {
	animalview.FillHealth(av, a, now, uint8(g.care.Level(id)), a.Body == nil && a.Tame && a.Injury > 0 && g.tended(id))
}
