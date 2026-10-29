package game

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/entity"
)

// Prisoners (ENG-003, M8). g.prisoners holds each prisoner's state (nil
// until the first capture; saved as Prisoners). A prisoner stays on
// raiderIDs, out of the raid AI (thinkRaider hands it to thinkPrisoner):
// it paths like a raider, so a closed door keeps it in its room, and no
// colonist, guard or turret treats it as hostile (standingRaider). Its
// person record says Where=Prisoner. Capture and arrest are
// care/capture.go (through carePrison, care_host_prison.go), its life
// prisoner_life.go, the warden prisoner_warden.go.

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
	// prisonerVisitStagger spreads first visits over the interval by id.
	prisonerVisitStagger = 7919
)

// isPrisoner reports whether id is the colony's prisoner.
func (g *Game) isPrisoner(id entity.EntityID) bool { return g.prisoners[id] != nil }

// exportPrisoners is the Prisoners save field in id order (nil: none).
func (g *Game) exportPrisoners() []entry[prisoner.State] { return exportMap(g.prisoners) }

// importPrisoners restores exportPrisoners' state.
func (g *Game) importPrisoners(es []entry[prisoner.State]) { g.prisoners = importMap(es) }

// prisonerView builds prisoner id's tab: its mode, resistance in whole
// points (rounded up: 0 only when broken), the warden who last came and
// whether it lies in a prisoner bed.
func (g *Game) prisonerView(id entity.EntityID) (PrisonerView, bool) {
	st := g.prisoners[id]
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

// cmdSetPrisonerMode sets prisoner's interaction mode: what its warden
// does on the next visit (Release and Execute do not wait for the visit
// interval).
func (g *Game) cmdSetPrisonerMode(id entity.EntityID, mode uint8) {
	st := g.prisoners[id]
	if st == nil || prisoner.Mode(mode) > prisoner.Execute {
		return
	}
	st.Mode = prisoner.Mode(mode)
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
		if c != 0 && g.isPrisoner(entity.EntityID(c)) != on {
			b = b.Release(c)
		}
	}
	g.bldgs.Set(bed, b)
}

// cmdReleasePrisoner frees prisoner at once: it walks off the map.
func (g *Game) cmdReleasePrisoner(id entity.EntityID) {
	if g.isPrisoner(id) {
		g.releasePrisoner(id, 0)
	}
}

// prisonBedFor is the nearest free prisoner bed for target (from: where
// the carrier stands).
func (g *Game) prisonBedFor(target entity.EntityID, from Position) (entity.EntityID, bool) {
	best, bestD := entity.EntityID(0), 1<<30
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || !b.Kind.Sleeps() || !b.ForPrisoners || !b.CanClaim(uint64(target)) {
			continue
		}
		if owner, taken := g.reserve.EntityOwner(bid); taken && owner != target {
			continue
		}
		if d := manhattan(from.X, from.Y, b.X, b.Y); best == 0 || d < bestD {
			best, bestD = bid, d
		}
	}
	return best, best != 0
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
	if g.prisoners == nil {
		g.prisoners = map[entity.EntityID]*prisoner.State{}
	}
	roll := int32(g.stream(streamPrison).IntN(1001))
	g.prisoners[target] = &prisoner.State{
		Mode: prisoner.Recruit, Resistance: prisoner.InitialResistance(roll),
		NextVisit: now + uint64(target)*prisonerVisitStagger%prisoner.VisitInterval, Captor: uint64(captor),
	}
	g.setWhere(target, wherePrisoner)
	if _, ok := g.joy[target]; !ok {
		g.joy[target] = recreation.Start
	}
	pos, _ := g.positions.Get(target)
	g.message(pos.X, pos.Y, "%s is now the colony's prisoner.", g.personName(target))
	g.addHistory("prisoner", g.personName(target)+" was taken prisoner.", target, captor)
}
