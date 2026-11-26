// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package diet

import (
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

type MalnutritionPeople interface {
	MalnutritionCandidates() []entity.EntityID
	Starving(id entity.EntityID) bool
	Vital(id entity.EntityID) (health.Record, bool)
	EnsureBody(id entity.EntityID, v *health.Record) *body.Body
	Roll(n int) int
}

type MalnutritionCare interface {
	Pos(id entity.EntityID) (int, int)
	Name(id entity.EntityID) string
	Message(x, y int, format string, args ...any)
	DieMedical(id entity.EntityID, cause health.Cause)
	ClearPatient(id entity.EntityID)
	BodyChanged(id entity.EntityID, b *body.Body, v *health.Record)
	UpdateDowned(id entity.EntityID, b *body.Body, v *health.Record)
}

// MalnutritionCandidates preserves the native needs-store order and keeps
// the selection predicate in diet while ownership remains with the caller.
func MalnutritionCandidates(all func(func(entity.EntityID, needs.Pair)), bodies map[entity.EntityID]*body.Body, owned func(entity.EntityID) bool) []entity.EntityID {
	var ids []entity.EntityID
	all(func(id entity.EntityID, n needs.Pair) {
		if (n.Starving() || body.Malnutrition(bodies[id]) > 0) && owned(id) {
			ids = append(ids, id)
		}
	})
	return ids
}

// StepMalnutrition advances ordinary map and caravan patients once per hour.
// Off-map WorldPawns are owned by the separate world pass.
func StepMalnutrition(h interface {
	MalnutritionPeople
	MalnutritionCare
}, now uint64) {
	for _, id := range h.MalnutritionCandidates() {
		v, ok := h.Vital(id)
		b := h.EnsureBody(id, &v)
		if !ok || v.Status == health.Dead || b == nil {
			continue
		}
		changed, dead := body.StepMalnutrition(b, h.Starving(id), now/clock.TicksPerHour, h.Roll)
		if dead {
			x, y := h.Pos(id)
			h.Message(x, y, "%s starved to death.", h.Name(id))
			h.DieMedical(id, health.CauseNoFood)
			h.ClearPatient(id)
		} else if changed {
			h.BodyChanged(id, b, &v)
			h.UpdateDowned(id, b, &v)
		}
	}
}
