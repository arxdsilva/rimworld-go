package game

import (
	"slices"

	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/sim/entity"
)

// Prisoners (ENG-003, M8). g.prisoners holds each prisoner's state (nil
// until the first capture; saved as Prisoners). A prisoner stays on
// raiderIDs, out of the raid AI (thinkRaider hands it to thinkPrisoner):
// it paths like a raider, so a closed door keeps it in its room, and no
// colonist, guard or turret treats it as hostile (standingRaider). Its
// person record says Where=Prisoner. Capture and arrest are
// prisoner_capture.go, its life prisoner_life.go, the warden
// prisoner_warden.go.

// Values follow the published RimWorld wiki by explicit user decision (ENG-003 balance override).
const (
	// releaseGoodwill is what a released prisoner's faction gives once it
	// is home.
	releaseGoodwill = 15
	// arrestGoodwill is what arresting a visitor costs with its faction.
	arrestGoodwill = 25
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
