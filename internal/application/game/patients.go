package game

import (
	"slices"

	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// The patients index (MED-00): every living pawn whose body needs care —
// an open wound, a condition or blood loss — as a sorted id list, so the
// hourly medical work walks the patients and never scans every body.
//
// bodyChanged (body.go) marks or clears a pawn after each change to its
// body, which covers hits (woundBody), healing, tending and surgery; the
// hourly sweep (phaseMedical, bleed.go) clears pawns who got well or died since.
// Right after a sweep the index is exactly what a scan of g.bodies would
// give (TestPatientsIndexMatchesScan). It is derived state: never saved,
// rebuilt on load from the bodies.

// medState is the medical system's runtime state. legacyWounds holds an
// older save's per-pawn wound records between loading and migrating them
// (migrateWoundInfections, infection.go); nil otherwise.
//
// animalPatients (MED-09, animal_health.go) is the sorted ids of animals
// with a body, derived state never saved: animalIdxOK false (a new game,
// or after a load) rebuilds it on first use.
type medState struct {
	patients     []entity.EntityID // sorted, unique
	legacyWounds map[entity.EntityID]legacyWound

	animalPatients []entity.EntityID
	animalIdxOK    bool
}

// indexedPatient reports id in the patients index.
func (g *Game) indexedPatient(id entity.EntityID) bool {
	_, found := slices.BinarySearch(g.med.patients, id)
	return found
}

// needsCare reports b has something to treat: an open wound, a condition
// or blood loss.
func needsCare(b *body.Body) bool {
	return b != nil && (hasOpenWound(b) || len(b.Conds) > 0 || b.BloodLoss > 0)
}

// isPatient reports id, with body b, belongs in the index: it needs care
// and is a living (standing or downed) pawn.
func (g *Game) isPatient(id entity.EntityID, b *body.Body) bool {
	if !needsCare(b) {
		return false
	}
	v, ok := g.vital.Get(id)
	return ok && v.Status != health.Dead
}

// markPatient adds id to the index (once).
func (g *Game) markPatient(id entity.EntityID) {
	i, found := slices.BinarySearch(g.med.patients, id)
	if !found {
		g.med.patients = slices.Insert(g.med.patients, i, id)
	}
}

// clearPatient removes id from the index.
func (g *Game) clearPatient(id entity.EntityID) {
	if i, found := slices.BinarySearch(g.med.patients, id); found {
		g.med.patients = slices.Delete(g.med.patients, i, i+1)
	}
}

// syncPatient marks or clears id after a change to its body b.
func (g *Game) syncPatient(id entity.EntityID, b *body.Body) {
	if g.isPatient(id, b) {
		g.markPatient(id)
		return
	}
	g.clearPatient(id)
}

// sweepPatients clears every indexed pawn that no longer needs care or
// has died (hourly, from phaseMedical).
func (g *Game) sweepPatients() {
	kept := g.med.patients[:0]
	for _, id := range g.med.patients {
		if g.isPatient(id, g.bodies[id]) {
			kept = append(kept, id)
		}
	}
	clear(g.med.patients[len(kept):])
	g.med.patients = kept
}

// rebuildPatients fills the index from the bodies (after a load).
func (g *Game) rebuildPatients() {
	g.med.patients = nil
	for id, b := range g.bodies {
		if g.isPatient(id, b) {
			g.med.patients = append(g.med.patients, id)
		}
	}
	slices.Sort(g.med.patients)
}
