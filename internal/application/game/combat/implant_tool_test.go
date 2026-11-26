// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package combat

import (
	"testing"

	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

func TestServoClawParticipatesInMeleeToolSelection(t *testing.T) {
	f := newFake(16, 16)
	const fighter entity.EntityID = 101
	f.pawn(fighter, 4, 4, weapon.Fists)
	f.implantTools = map[entity.EntityID][]weapon.Tool{
		fighter: {{Name: "servo claw", Damage: 22, Pen: 330, Cooldown: 120, Form: damage.FormScratch, Chance: 1000, NoSocialFight: true}},
	}
	s := f.combat().Strikes
	d := weapon.DefOf(weapon.Fists)
	chosen := s.tool(fighter, weapon.Fists, weapon.Fists, d, func(n int) int { return n - 1 })
	if chosen.Name != "servo claw" || chosen.Damage != 22 || chosen.Pen != 330 {
		t.Fatalf("claw unused by melee tool picker: %+v", chosen)
	}
	chosen = s.tool(fighter, weapon.Fists, weapon.Fists, d, func(n int) int { return 0 })
	if chosen.Name != "fist" {
		t.Fatalf("claw replaced regular fist: %+v", chosen)
	}
}

// A pawn with two artificial hands still attacks with its head and teeth.
// Its new prosthetic tools replace organic fists rather than duplicating them.
func TestCoreArtificialHandsDropOrganicFistAttack(t *testing.T) {
	f := newFake(16, 16)
	const fighter entity.EntityID = 101
	f.pawn(fighter, 4, 4, weapon.Fists)
	f.fistCounts = map[entity.EntityID]int{fighter: 0}
	s := f.combat().Strikes
	d := weapon.DefOf(weapon.Fists)
	chosen := s.tool(fighter, weapon.Fists, weapon.Fists, d, func(n int) int { return 0 })
	if chosen.Name != "head" {
		t.Fatalf("removed fists returned %+v", chosen)
	}
	f.implantTools = map[entity.EntityID][]weapon.Tool{
		fighter: {{Name: "prosthetic fist", Damage: 8, Pen: 120, Cooldown: 120, Form: damage.FormBlunt, Chance: 1000}},
	}
	chosen = s.tool(fighter, weapon.Fists, weapon.Fists, d, func(n int) int { return n - 1 })
	if chosen.Name != "prosthetic fist" || chosen.Damage != 8 {
		t.Fatalf("prosthetic strike not available: %+v", chosen)
	}
	f.fistCounts[fighter] = 1
	chosen = s.tool(fighter, weapon.Fists, weapon.Fists, d, func(n int) int { return 0 })
	if chosen.Name != "fist" {
		t.Fatalf("remaining organic fist removed: %+v", chosen)
	}
}

// Social brawls exclude the power claw by its tool property, independent
// of its localized attack label. Normal melee still selects that tool.
func TestBrawlRejectsPowerClaw(t *testing.T) {
	f := newFake(16, 16)
	const fighter entity.EntityID = 101
	f.pawn(fighter, 4, 4, weapon.Fists)
	f.implantTools = map[entity.EntityID][]weapon.Tool{
		fighter: {{Name: "localized power claw", Damage: 22, Pen: 330,
			Cooldown: 120, Form: damage.FormScratch, Chance: 1000, NoSocialFight: true}},
	}
	s := f.combat().Strikes
	d := weapon.DefOf(weapon.Fists)
	rollLast := func(n int) int { return n - 1 }
	if tool := s.tool(fighter, weapon.Fists, weapon.Fists, d, rollLast); tool.Name != "localized power claw" {
		t.Fatalf("combat claw disabled: %+v", tool)
	}
	if tool := s.toolFor(fighter, weapon.Fists, weapon.Fists, d, rollLast, true); tool.Name == "localized power claw" {
		t.Fatalf("social fight selected claw: %+v", tool)
	}
}
