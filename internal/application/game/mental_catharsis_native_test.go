// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/clock"
)

func TestInterruptedMentalBreakCatharsisInRealGame(t *testing.T) {
	g := New(Config{Seed: 81, Width: 12, Height: 10})
	id, _ := g.SpawnNamedPawn(4, 4, "Ada")
	hasCatharsis := func() bool {
		for _, th := range g.thoughts(id) {
			if th.Kind == mood.Catharsis {
				return true
			}
		}
		return false
	}
	g.ForceBreak(id, mentalbreak.SadWander)
	g.mind.End(id)
	if hasCatharsis() || g.Breaking(id) != "" {
		t.Fatal("arrested SadWander granted catharsis or stayed in a break")
	}
	g.ForceBreak(id, mentalbreak.Berserk)
	g.mind.End(id)
	if !hasCatharsis() || g.Breaking(id) != "" {
		t.Fatal("interrupted Berserk must clear its break and grant catharsis")
	}
	g.clock.Advance(int(mood.DefOf(mood.Catharsis).Duration))
	g.mind.Force(id, mentalbreak.Catatonic)
	g.clock.Advance(5 * clock.TicksPerDay)
	// Natural catatonic recovery follows the same catharsis decision,
	// with the body's standing state handled by the native pawn host.
	g.mind.Downed(id)
	if g.Breaking(id) != "" || !hasCatharsis() {
		t.Fatal("natural catatonic recovery must clear the break and grant catharsis")
	}
}
