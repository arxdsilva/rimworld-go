package game

import (
	"slices"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/entity"
)

// Prisoners (ENG-003, M8). The prison package holds each prisoner's state
// (saved as Prisoners), its life and the warden's work (prison_host.go). A
// prisoner stays on raiderIDs, out of the raid AI (thinkRaider hands it to
// prison.Live): it paths like a raider, so a closed door keeps it in its
// room, and no colonist, guard or turret treats it as hostile
// (standingRaider). Its person record says Where=Prisoner. Capture and
// arrest are care/capture.go (through carePrison, care_host_prison.go).

// Values follow the published RimWorld wiki by explicit user decision (ENG-003 balance override).
const (
	// releaseGoodwill is what a released prisoner's faction gives once it
	// is home.
	releaseGoodwill = 15
	// arrestGoodwill is what arresting a visitor costs with its faction.
	arrestGoodwill = 25
	// arrestBase is the arrest chance, permille, before the arrester's
	// Social skill (arrestPerSocial a level) and ArrestSuccess stat.
	arrestBase      = 600
	arrestPerSocial = 20
	arrestMax       = 980
)

// exportPrisoners is the Prisoners save field in id order (nil: none).
func (g *Game) exportPrisoners() []entry[prisoner.State] { return exportMap(g.prison.Export()) }

// importPrisoners restores exportPrisoners' state.
func (g *Game) importPrisoners(es []entry[prisoner.State]) { g.prison.Import(importMap(es)) }

// prisonerView builds prisoner id's tab: its mode, resistance in whole
// points (rounded up: 0 only when broken), the warden who last came and
// whether it lies in a prisoner bed.
func (g *Game) prisonerView(id entity.EntityID) (PrisonerView, bool) {
	st := g.prison.State(id)
	if st == nil {
		return PrisonerView{}, false
	}
	v := PrisonerView{Mode: uint8(st.Mode), Resistance: int((st.Resistance + prisoner.ResistanceUnit - 1) / prisoner.ResistanceUnit)}
	if w := g.raiders[id].Warden; w != 0 {
		v.Warden = g.personName(w)
	}
	if bid := g.sleptAt[id]; bid != 0 {
		if b, ok := g.bldgs.Get(bid); ok && b.ForPrisoners && b.SlotOf(uint64(id)) >= 0 {
			v.ForPrisonersBed = true
		}
	}
	return v, true
}

// cmdSetBedForPrisoners marks bed for prisoners (or back for colonists);
// sleepers of the other kind give it up.
func (g *Game) cmdSetBedForPrisoners(bed entity.EntityID, on bool) {
	b, ok := g.bldgs.Get(bed)
	if !ok || !b.Kind.Sleeps() || b.ForPrisoners == on {
		return
	}
	b.ForPrisoners = on
	for _, c := range [2]uint64{b.Claimant, b.Claimant2} {
		if c != 0 && g.prison.Is(entity.EntityID(c)) != on {
			b = b.Release(c)
		}
	}
	g.bldgs.Set(bed, b)
}

// prisonBedFor is the safe prison bed for target (care/beds.go ranks it;
// from: where the carrier stands).
func (g *Game) prisonBedFor(target entity.EntityID, from Position) (entity.EntityID, bool) {
	bid := care.PrisonBedFor(g.care, target, from.X, from.Y)
	return bid, bid != 0
}

// withID is ids with id added in order, in a new array (the caller may be
// ranging over the old one).
func withID(ids []entity.EntityID, id entity.EntityID) []entity.EntityID {
	i, found := slices.BinarySearch(ids, id)
	if found {
		return ids
	}
	out := make([]entity.EntityID, 0, len(ids)+1)
	out = append(append(append(out, ids[:i]...), id), ids[i:]...)
	return out
}

// withoutID is ids without id, in a new array.
func withoutID(ids []entity.EntityID, id entity.EntityID) []entity.EntityID {
	i := slices.Index(ids, id)
	if i < 0 {
		return ids
	}
	out := make([]entity.EntityID, 0, len(ids)-1)
	return append(append(out, ids[:i]...), ids[i+1:]...)
}

// visitor reports id is a friendly visitor on the map (M9).
func (g *Game) visitor(id entity.EntityID) bool {
	p := g.personOf(id)
	return p != nil && p.Where == whereVisitor && g.onMap(id)
}

// adjacent reports a and b stand within one tile of each other.
func (g *Game) adjacent(a, b entity.EntityID) bool {
	pa, _ := g.positions.Get(a)
	pb, _ := g.positions.Get(b)
	return abs(pa.X-pb.X) <= 1 && abs(pa.Y-pb.Y) <= 1
}

// arrestChance is the permille chance arrester takes target in: certain
// when the target is down or asleep, else the base plus the arrester's
// Social skill, times its ArrestSuccess stat.
func (g *Game) arrestChance(arrester, target entity.EntityID) int {
	v, _ := g.vital.Get(target)
	tj, _ := g.jobs.Get(target)
	if v.Status != health.Alive || tj.Kind == job.Sleep && tj.State == job.Do {
		return 1000
	}
	c := int64(arrestBase + arrestPerSocial*int(g.skillOf(arrester, skSocial).Level))
	c = c * int64(g.statOf(arrester, stat.ArrestSuccess)) / 1000
	return int(min(max(c, 0), arrestMax))
}

// makePrisoner turns target into the colony's prisoner, taken by captor:
// a colonist leaves the roster (keeping its relations and bio), every
// prisoner joins raiderIDs out of the raid AI, with a fresh resistance
// and the Recruit mode.
func (g *Game) makePrisoner(target, captor entity.EntityID) {
	now := uint64(g.clock.Now())
	g.ensurePerson(target)
	if slices.Contains(g.pawnIDs, target) {
		g.pawnIDs = withoutID(g.pawnIDs, target)
		g.mind.End(target) // mind (SIM-066): an arrested breaker's break ends
		delete(g.draft, target)
		g.releaseBed(target)
		g.raiderIDs = withID(g.raiderIDs, target)
	}
	r := g.raiders[target]
	r.Leaving, r.Hungry, r.FoodRetry, r.Warden = false, false, 0, 0
	r.Mode, r.Idle, r.RetryAt = raidArrive, 0, 0
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
func (h careReach) BedHazard(bed entity.EntityID) bool {
	b, ok := h.g.bldgs.Get(bed)
	t := int(h.g.temperatureAt(b.X, b.Y))
	return !ok || h.g.fireNear(b.X, b.Y, 1) || t < exposure.Everyday.Min-bedDeadlyMargin || t > exposure.Everyday.Max+bedDeadlyMargin
}

// prisonerThoughts are a prisoner's own thoughts: being held (none for
// anyone else).
func (g *Game) prisonerThoughts(id entity.EntityID) []mood.Thought {
	if !g.prison.Is(id) {
		return nil
	}
	return mood.Active(nil, mood.Imprisoned)
}

// prisonerThoughtList is a prisoner's whole thought list: thoughts plus
// prisonerThoughts unless thoughts already carry them.
func (g *Game) prisonerThoughtList(id entity.EntityID) []mood.Thought {
	ts := g.thoughts(id)
	for _, t := range ts {
		if t.Kind == mood.Imprisoned {
			return ts
		}
	}
	return append(ts, g.prisonerThoughts(id)...)
}

// prisonerMood is the mood a prisoner's resistance answers to.
func (g *Game) prisonerMood(id entity.EntityID) int { return mood.Level(g.prisonerThoughtList(id)) }

// prisonerBed is the bed prisoner id holds, if it still stands.
func (g *Game) prisonerBed(id entity.EntityID) (entity.EntityID, bool) {
	bid := g.sleptAt[id]
	if bid == 0 {
		return 0, false
	}
	b, ok := g.bldgs.Get(bid)
	return bid, ok && b.SlotOf(uint64(id)) >= 0
}

// walkOut takes a released pawn to the map edge; there (or after a day
// of trying) it goes home.
func (g *Game) walkOut(id entity.EntityID, r raiderState, j job.Record, now uint64) {
	pos, _ := g.positions.Get(id)
	w, h := g.grid.Width(), g.grid.Height()
	if pos.X == 0 || pos.Y == 0 || pos.X == w-1 || pos.Y == h-1 || now-r.Idle >= raiderGiveUp {
		g.departReleased(id)
		return
	}
	if j.Active() && j.Kind == job.Move || now < r.RetryAt {
		return
	}
	ex, ey := nearestEdge(pos, w, h)
	if x, y, ok := g.combat.Tiles.WalkableNear(ex, ey, 3); ok && g.startJob(id, job.Record{Kind: job.Move, TX: x, TY: y}) {
		return
	}
	r.RetryAt = now + raiderRetry
	g.raiders[id] = r
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
		Hunger: needs.Percent(n.Hunger), Rest: needs.Percent(n.Rest), Health: v.HP() * 100 / health.MaxHP,
		Weapon: weaponDefOf(g, id).Name, Prisoner: g.prison.Is(id), Visitor: g.visitor(id), Ally: g.isAlly(id), SleptInBed: g.sleptAt[id] != 0,
		Recreation: g.joy[id] * 100 / recreation.Full}
	ts := g.prisonerThoughtList(id)
	pv.Mood = mood.Level(ts)
	pv.MoodTarget, pv.MoodLabel, pv.Thoughts = pv.Mood, mood.Label(pv.Mood), thoughtViews(ts)
	pv.Lying = v.Status != health.Alive || j.Kind == job.Sleep && (j.State == job.Do || j.State == job.Complete)
	pv.InBed = g.inBed(id)
	cv := g.care.View(id, j)
	pv.CarriedBy, pv.CarryingPawn, pv.Tended, pv.OnMedicalBed = cv.CarriedBy, cv.Carrying, cv.Tended, cv.OnMedicalBed
	pv.GuestFaction = careGuests{g}.FactionName(id) // visitors.go (SIM-063)
	pv.Infection, pv.Wounded = g.woundView(id)      // infection.go (MED-02)
	g.fillPawnAction(&pv, id, j)
	return pv
}
