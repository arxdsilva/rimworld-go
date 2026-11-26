// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/saveutil"
	"rimworld-go/internal/domain/building/construction"
	"slices"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/roster"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/entity"
)

// Captives live in game/prison and remain on the raid roster until released or recruited.
// Capture/arrest are owned by care; raid movements by raid.
const (
	// Faction goodwill on a released prisoner.
	releaseGoodwill = 15
	// Trying to arrest a visitor lowers its faction goodwill to at most -75.
	arrestGoodwill  = -75
	arrestBase      = prisoner.ArrestBase
	arrestPerSocial = prisoner.ArrestPerSocial
)

// exportPrisoners is the Prisoners save field in id order (nil: none).
func (g *Game) exportPrisoners() []entry[prisoner.State] {
	return saveutil.ExportMap[entry[prisoner.State]](g.prison.Export())
}

// importPrisoners restores exportPrisoners' state.
func (g *Game) importPrisoners(es []entry[prisoner.State]) { g.prison.Import(saveutil.ImportMap(es)) }

// prisonerView delegates to the captive owner with the live bed predicate.
func (g *Game) prisonerView(id entity.EntityID) (PrisonerView, bool) {
	_, ok := g.prisonerBed(id)
	v, held := g.prison.View(id, ok)
	if p := g.personOf(id); held && p != nil {
		v.Faction = faction.AffiliationLabel(g.factionOf(p.Faction), p.Origin)
	}
	return v, held
}

// cmdSetBedForPrisoners updates bed designation and evicts incompatible occupants.
func (g *Game) cmdSetBedForPrisoners(bed entity.EntityID, on bool) {
	b, ok := g.bldgs.Get(bed)
	if !ok || !b.Kind.Sleeps() || b.ForPrisoners == on {
		return
	}
	b.ForPrisoners = on
	for _, c := range [2]uint64{b.Claimant, b.Claimant2} {
		if c != 0 && g.prison.Is(entity.EntityID(c)) != on {
			b = construction.ReleaseBed(b, c)
		}
	}
	g.bldgs.Set(bed, b)
}

// prisonBedFor is the safe prison bed for target (care/beds.go ranks it;
// from: where the carrier stands).

// withID is ids with id added in order, in a new array (the caller may be
// ranging over the old one).
func withID(ids []entity.EntityID, id entity.EntityID) []entity.EntityID {
	return roster.WithID(ids, id)
}

// withoutID is ids without id, in a new array.
func withoutID(ids []entity.EntityID, id entity.EntityID) []entity.EntityID {
	return roster.WithoutID(ids, id)
}

// visitor reports id is a friendly visitor on the map (M9).
func (g *Game) visitor(id entity.EntityID) bool {
	p := g.personOf(id)
	return p != nil && p.Where == whereVisitor && g.onMap(id)
}

// arrestChance supplies the current pawn facts to the native prisoner rule.
func (g *Game) arrestChance(arrester, target entity.EntityID) int {
	v, _ := g.vital.Get(target)
	j, _ := g.jobs.Get(target)
	return prisoner.ArrestChance(v.Status != health.Alive, j.Kind == job.Sleep && j.State == job.Do,
		int(g.skillOf(arrester, skSocial).Level), g.statOf(arrester, stat.ArrestSuccess))
}

// makePrisoner moves an actor into custody, retaining its identity and bio.
func (g *Game) makePrisoner(target, captor entity.EntityID) {
	now := uint64(g.clock.Now())
	g.ensurePerson(target)
	senseOf(g).Gear.Spill(target) // game/sense (CSE-036): no prisoner keeps a pack
	if slices.Contains(g.pawnIDs, target) {
		g.pawnIDs = withoutID(g.pawnIDs, target)
		g.mind.End(target) // mind (SIM-066): an arrested breaker's break ends
		delete(g.draft, target)
		g.releaseBed(target)
		g.raiderIDs = withID(g.raiderIDs, target)
	}
	r := g.raiders[target]
	r.Leaving, r.Hungry, r.FoodRetry, r.Warden = false, false, 0, 0
	r.Mode, r.Idle, r.RetryAt = raid.Arriving, 0, 0
	if r.Arrived == 0 {
		r.Arrived = now
	}
	g.raiders[target] = r
	g.prison.Take(target, captor, now) // prison/prison.go: a fresh resistance, Recruit
	g.setWhere(target, wherePrisoner)
	if _, ok := g.joy[target]; !ok {
		g.joy[target] = recreation.Start
	}
	pos, _ := g.positions.Get(target)
	g.message(pos.X, pos.Y, "%s is now the colony's prisoner.", g.personName(target))
	g.addHistory("prisoner", g.personName(target)+" was taken prisoner.", target, captor)
}

// bedDeadlyMargin: air this far beyond everyday comfort is deadly
// (tenths of a degree; wiki: Rescue, 90 °C).
const bedDeadlyMargin = 900

// BedHazard is care.Reach's (SIM-063): fire on or beside bed's anchor
// (every bed's cells are within one tile of it), or deadly air there.
func (h careReach) BedHazard(bed entity.EntityID) bool { return h.BedWhy(bed, 0) != "" }
func (h careReach) BedWhy(bed, carrier entity.EntityID) string {
	g := h.g
	b, ok := g.bldgs.Get(bed)
	if !ok {
		return care.ReasonNoBed
	}
	if g.fireNear(b.X, b.Y, 1) {
		return care.ReasonBedBurning
	}
	bands := exposure.Everyday
	if carrier != 0 {
		bands = g.bandsOf(carrier)
	}
	t := int(g.temperatureAt(b.X, b.Y))
	if t < bands.Min-bedDeadlyMargin || t > bands.Max+bedDeadlyMargin {
		return care.ReasonBedTemperature
	}
	return ""
}
func (h careReach) PrisonRoom(bed entity.EntityID) bool {
	b, ok := h.g.bldgs.Get(bed)
	return ok && h.g.roomAt(b.X, b.Y) >= 0
}

// prisonerThoughtList includes the captive mood without duplicates.
func (g *Game) prisonerThoughtList(id entity.EntityID) []mood.Thought {
	return g.thoughts(id)
}

// prisonerMood is the mood a prisoner's resistance answers to.
func (g *Game) prisonerMood(id entity.EntityID) int { return g.moodOf(id) }

// prisonerBed is the bed prisoner id holds, if it still stands.
func (g *Game) prisonerBed(id entity.EntityID) (entity.EntityID, bool) {
	bid := g.bedOf(id) // ownership survives a collapse or sleeping on the floor
	b, ok := g.bldgs.Get(bid)
	return bid, ok && b.ForPrisoners
}

// departReleased: a released pawn reached the edge. Its faction is
// grateful, and it is off the map.
func (g *Game) departReleased(id entity.EntityID) {
	if p := g.personOf(id); p != nil {
		if f := g.factionOf(p.Faction); f != nil {
			faction.Adjust(f, releaseGoodwill, "prisoner released")
		}
	}
	g.setWhere(id, whereOffMap)
	g.addHistory("released", g.personName(id)+" was released and went home.", id)
	g.removeRaider(id) // raids.go: off the map
}

// prisonerPawnView is the read model of a prisoner (or released pawn):
// not hostile, Prisoner while held.
func (g *Game) prisonerPawnView(id entity.EntityID) PawnView {
	pos, _ := g.positions.Get(id)
	name, _ := g.names.Get(id)
	j, _ := g.jobs.Get(id)
	v, _ := g.vital.Get(id)
	n, _ := g.need.Get(id)
	pv := PawnView{ID: id, Name: name, X: pos.X, Y: pos.Y, Job: j.Line(), Status: v.Status.String(), Reason: v.Cause.String(),
		Hunger: needs.Percent(n.Hunger), Rest: needs.Percent(n.Rest), Health: v.HP() * 100 / health.MaxHP, Malnutrition: int(body.Malnutrition(g.bodies[id])) / 10, ToxicBuildup: int(body.ToxicBuildup(g.bodies[id])) / 10,
		Weapon: weaponDefOf(g, id).Name, Prisoner: g.prison.Is(id), Visitor: g.visitor(id), Ally: g.isAlly(id), SleptInBed: g.sleptAt[id] != 0,
		NoRecreation: true, FoodPolicy: g.sk.diet.Of(id, false).Name}
	ts := g.prisonerThoughtList(id)
	pv.Thoughts = thoughtViews(ts)
	g.fillSocialPawn(&pv, mood.Level(ts))
	pv.Lying = v.Status != health.Alive || j.Kind == job.Sleep && (j.State == job.Do || j.State == job.Complete)
	pv.InBed = g.inBed(id)
	cv := g.care.View(id, j)
	pv.CarriedBy, pv.CarryingPawn, pv.Tended, pv.OnMedicalBed = cv.CarriedBy, cv.Carrying, cv.Tended, cv.OnMedicalBed
	pv.GuestFaction = careGuests{g}.FactionName(id)                   // visitors.go (SIM-063)
	pv.Infection, pv.Wounded = g.woundView(id)                        // infection.go (MED-02)
	pv.Hypothermia, pv.Heatstroke, pv.Frostbite = exposureView(g, id) // heat.go (TEMP-3): prisoners and guests take exposure
	if person := g.personOf(id); person != nil {
		if f := g.factionOf(person.Faction); f != nil {
			pv.Faction = f.Name
		}
	}
	g.fillPawnAction(&pv, id, j)
	return pv
}
