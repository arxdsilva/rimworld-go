// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/aging"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
)

// stepAging advances the native medical and daily condition schedule.
func (g *Game) stepAging(now uint64) { aging.Step(agingHost{g}, now, g.clock.Calendar()) }

// agingSalt fixes chronic condition progression per world seed and pawn.
func (g *Game) agingSalt(id entity.EntityID) uint64 {
	return g.streams.seed ^ uint64(id)*0x9e3779b97f4a7c15
}

// stepAgingConds advances only patients under the current map pass.
func (g *Game) stepAgingConds(now uint64) {
	aging.StepIDs(agingHost{g}, now, ownPatients(g))
}

// rollAging retains the historical hourly test entrypoint for daily onset.
func (g *Game) rollAging(now uint64) {
	aging.RollDaily(agingHost{g}, now, g.clock.Calendar())
}

// developAging shares one disease owner across medical and world passes.
func (g *Game) developAging(id entity.EntityID, k body.CondKind, part int, now uint64) {
	aging.Develop(agingHost{g}, id, k, part, now)
}

func confusedWander(g *Game, id entity.EntityID, now uint64, reason string) {
	aging.ConfusedWander(agingHost{g}, id, now, reason)
}
