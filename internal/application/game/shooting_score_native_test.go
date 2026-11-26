// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
)

func TestManipulationAffectsShootingAccuracy(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	p, _ := g.SpawnNamedPawn(4, 4, "Ada")
	aim := combatSkills{g}
	before, _ := aim.ShotAim(p)
	v := g.VitalOf(p)
	b := g.ensureBody(p, &v)
	b.Missing |= 1 << body.HandR
	g.bodyChanged(p, b, &v)
	after, offset := aim.ShotAim(p)
	want := attack.ShooterPerTile(attack.ShootingScore(int(g.skillOf(p, skill.Shooting).Level),
		int(g.statOf(p, stat.ShootAcc))))
	if after != want || after >= before || offset != int(g.skillOf(p, skill.Shooting).Level) {
		t.Fatalf("missing hand must lower shooting: before %d after %d offset %d want %d",
			before, after, offset, want)
	}
}

func TestSightAffectsShootingAccuracy(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	p, _ := g.SpawnNamedPawn(4, 4, "Ada")
	aim := combatSkills{g}
	before, _ := aim.ShotAim(p)
	baseAcc := g.statOf(p, stat.ShootAcc)
	v := g.VitalOf(p)
	b := g.ensureBody(p, &v)
	b.Missing |= 1<<body.EyeL | 1<<body.EyeR
	g.bodyChanged(p, b, &v)
	after, offset := aim.ShotAim(p)
	want := attack.ShooterPerTile(attack.ShootingScore(int(g.skillOf(p, skill.Shooting).Level),
		int(g.statOf(p, stat.ShootAcc))))
	if after != want || after >= before || offset != int(g.skillOf(p, skill.Shooting).Level) {
		t.Fatalf("missing eyes must lower shooting: before %d after %d offset %d want %d",
			before, after, offset, want)
	}
	if acc := g.statOf(p, stat.ShootAcc); acc != baseAcc-12 {
		t.Fatalf("body Sight must change effective shooting, and ShootAcc: %d -> %d", baseAcc, acc)
	}
}
