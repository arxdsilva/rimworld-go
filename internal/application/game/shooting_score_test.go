// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/entity"
)

// TestShootingScoreInTheGame: Shooting accuracy points (traits, sight,
// conditions) join the Shooting level in one score before the per-tile
// curve (SHT-002); sight weighs 12 points capped at 200 % (SHT-003).
func TestShootingScoreInTheGame(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 30, Height: 8})
	shooter := func(x int, name string, edit func(p *pawngen.Pawn)) entity.EntityID {
		id := spawnGen(t, g, x, 4, genPawn(name, func(p *pawngen.Pawn) {
			p.Skills[skill.Shooting].Level = 10
			p.Traits, p.Conditions = nil, nil
			if edit != nil {
				edit(p)
			}
		}))
		g.Arm(id, item.Rifle)
		return id
	}
	careful := shooter(2, "Dee", func(p *pawngen.Pawn) { p.Traits = []trait.ID{trait.DeliberateAim} })
	plain := shooter(26, "Pim", nil)
	raider := spawnRaiderAt(t, g, 14, 4, 0)
	if g.statOf(careful, stat.ShootAcc) != 5 || g.statOf(plain, stat.ShootAcc) != 0 {
		t.Fatalf("shoot_acc %d / %d, want 5 / 0", g.statOf(careful, stat.ShootAcc), g.statOf(plain, stat.ShootAcc))
	}
	for id, score := range map[entity.EntityID]int{careful: 15, plain: 10} {
		r, ok := g.HitChance(id, raider)
		if !ok || r.Dist != 12 || r.Shooter != attack.ShooterFactor(attack.ShooterPerTile(score), 12) || r.Chance != r.Total() {
			t.Fatalf("score %d at 12 tiles: %+v", score, r)
		}
	}
	if a, _ := g.HitChance(careful, raider); a.Chance <= 0 {
		t.Fatalf("careful chance %+v", a)
	} else if b, _ := g.HitChance(plain, raider); a.Shooter <= b.Shooter {
		t.Fatalf("deliberate aim's points must raise the shooter factor: %d vs %d", a.Shooter, b.Shooter)
	}
	near := shooter(5, "Nia", func(p *pawngen.Pawn) { p.Conditions = []pawngen.Condition{pawngen.Nearsighted} })
	if acc := g.statOf(near, stat.ShootAcc); acc != -6 {
		t.Fatalf("nearsighted %d, want -6", acc)
	}
	v := g.VitalOf(plain)
	b := g.ensureBody(plain, &v)
	b.Missing |= 1 << body.EyeL
	g.bodyChanged(plain, b, &v)
	if acc := g.statOf(plain, stat.ShootAcc); acc != -6 {
		t.Fatalf("one eye %d, want -6", acc)
	}
	b.Missing |= 1 << body.EyeR
	g.bodyChanged(plain, b, &v)
	if acc := g.statOf(plain, stat.ShootAcc); acc != -12 {
		t.Fatalf("blind %d, want -12", acc)
	}
	for sight, want := range map[int32]int32{1500: 6, 2000: 12, 2500: 12, 500: -6, 0: -12} {
		ms := bodyMods(bodyKey{1000, 1000, sight})
		if len(ms) != 1 || ms[0].Stat != stat.ShootAcc || int32(ms[0].Add) != want {
			t.Errorf("sight %d: %+v, want shoot_acc %+d", sight, ms, want)
		}
	}
}

// TestMeleeCooldownIgnoresAimTime: aim time scales a ranged warmup only;
// a brawl or tantrum punch waits the fists' cooldown whatever the trait
// (SHT-103).
func TestMeleeCooldownIgnoresAimTime(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	for i, c := range []struct {
		tr  trait.ID
		aim int
	}{{trait.QuickDraw, 50}, {trait.DeliberateAim, 125}} {
		id := spawnGen(t, g, 2+2*i, 2, genPawn("Tri", func(p *pawngen.Pawn) { p.Traits = []trait.ID{c.tr} }))
		if got := (mindBrawls{g}).PunchTicks(id); got != weapon.DefOf(weapon.Fists).Cooldown || got != 90 {
			t.Errorf("%v: punch waits %d, want the fists' 90", c.tr, got)
		}
		if got := (combatSkills{g}).AimTicks(id, 100); got != c.aim {
			t.Errorf("%v: aim 100 -> %d, want %d", c.tr, got, c.aim)
		}
	}
}
