// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/game/combat"
)

// Vanilla hostility response Attack: a colonist set to Attack meets a hostile
// with its hands. Left at the default an unarmed colonist still flees.
func TestUnarmedColonistSetToAttackFightsBack(t *testing.T) {
	g := New(Config{Seed: 29, Width: 20, Height: 12})
	id, err := g.SpawnNamedPawn(5, 5, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if g.fightsBack(id) {
		t.Fatal("an unarmed colonist at the default response fights")
	}
	g.combat.Drafts.SetResponse(id, combat.RespondAttack)
	if !g.fightsBack(id) {
		t.Fatal("an unarmed colonist set to Attack does not fight")
	}
	g.combat.Drafts.SetResponse(id, combat.RespondFlee)
	if g.fightsBack(id) {
		t.Fatal("an unarmed colonist set to Flee fights")
	}
}
