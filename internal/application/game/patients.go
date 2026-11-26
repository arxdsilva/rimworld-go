// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"

	"rimworld-go/internal/application/game/diet"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/msgfmt"
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
	return body.NeedsCare(b)
}

// isPatient reports id, with body b, belongs in the index: it needs care
// and is a living (standing or downed) pawn.
func (g *Game) isPatient(id entity.EntityID, b *body.Body) bool {
	if !needsCare(b) {
		return false
	}
	v, ok := g.vital.Get(id)
	return ok && v.Status != health.Dead && (g.worldSt == nil || !g.worldSt.ax9.Asleep(id)) // AX-9: stasis suspends illness
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

// ownPatients is a copy of the patients index (a death may touch the
// index) holding the ids the current map's pass steps (MAP-0, owns): each
// patient once a tick, on its own map's pass, or the first map's when it is
// on none (a caravan member).
func ownPatients(g *Game) []entity.EntityID {
	ids := slices.Clone(g.med.patients)
	ids = slices.DeleteFunc(ids, func(id entity.EntityID) bool {
		return worldPawnCandidate(g, id) || len(g.maps) > 1 && !owns(g, id)
	})
	return ids
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

// agingHost is aging's view of the colony.
type agingHost struct{ g *Game }

func (h agingHost) Pawns() []entity.EntityID    { return h.g.pawnIDs }
func (h agingHost) Patients() []entity.EntityID { return ownPatients(h.g) }
func (h agingHost) Eligible(id entity.EntityID) bool {
	v, ok := h.g.vital.Get(id)
	_, aged := h.g.sk.bios[id]
	return ok && v.Status != health.Dead && aged && !h.g.prison.Is(id) && !h.g.isRaider(id) && !h.g.visitor(id)
}
func (h agingHost) Bio(id entity.EntityID) pawngen.Pawn { return h.g.sk.bios[id] }
func (h agingHost) Vital(id entity.EntityID) (health.Record, bool) {
	return h.g.vital.Get(id)
}
func (h agingHost) Body(id entity.EntityID) *body.Body { return h.g.bodies[id] }
func (h agingHost) EnsureBody(id entity.EntityID, v *health.Record) *body.Body {
	return h.g.ensureBody(id, v)
}
func (h agingHost) Salt(id entity.EntityID) uint64 { return h.g.agingSalt(id) }
func (h agingHost) Roll(n int) int                 { return h.g.stream(streamMedical).IntN(n) }
func (h agingHost) Pos(id entity.EntityID) (int, int) {
	p, _ := h.g.positions.Get(id)
	return p.X, p.Y
}
func (h agingHost) Name(id entity.EntityID) string       { return h.g.name(id) }
func (h agingHost) PersonName(id entity.EntityID) string { return h.g.personName(id) }
func (h agingHost) Message(x, y int, format string, args ...any) {
	h.g.message(x, y, format, args...)
}
func (h agingHost) Letter(now uint64, x, y int, id entity.EntityID, title string, threat bool, msg msgfmt.Text) {
	kind := letters.Caution
	if threat {
		kind = letters.Threat
	}
	h.g.post(MessageView{Tick: now, X: x, Y: y, Focus: id, Letter: true, Title: title, Kind: kind, Msg: msg})
}
func (h agingHost) DieMedical(id entity.EntityID, cause health.Cause) { h.g.dieMedical(id, cause) }
func (h agingHost) ClearPatient(id entity.EntityID)                   { h.g.clearPatient(id) }
func (h agingHost) BodyChanged(id entity.EntityID, b *body.Body, v *health.Record) {
	h.g.bodyChanged(id, b, v)
}
func (h agingHost) UpdateDowned(id entity.EntityID, b *body.Body, v *health.Record) {
	h.g.updateDowned(id, b, v)
}
func (h agingHost) Memories(id entity.EntityID) mood.Memories { return h.g.memories[id] }
func (h agingHost) SetMemories(id entity.EntityID, m mood.Memories) {
	h.g.memories[id] = m
}
func (h agingHost) Drafted(id entity.EntityID) bool { return h.g.draft[id] }
func (h agingHost) Wander(id entity.EntityID, kind mentalbreak.Kind, until uint64) bool {
	g := h.g
	if _, breaking := g.breaks[id]; breaking {
		return false
	}
	g.interrupt(id, job.Cancelled)
	delete(g.orderQueue, id)
	g.breaks[id] = breakState{kind: kind, until: until}
	return true
}

// MalnutritionCandidates walks native needs in store order. The world-pawn
// owner and off-map caravan ownership are independent of this map pass.
func (h agingHost) MalnutritionCandidates() []entity.EntityID {
	return diet.MalnutritionCandidates(h.g.need.All, h.g.bodies, func(id entity.EntityID) bool {
		return owns(h.g, id) && !worldPawnCandidate(h.g, id)
	})
}
func (h agingHost) Starving(id entity.EntityID) bool {
	return h.g.need.Ref(id).Starving()
}
