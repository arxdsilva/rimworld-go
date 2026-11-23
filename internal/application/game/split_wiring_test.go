// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/body"
)

// CBT 3.4: a blow's damage form reaches the body as hit.Blow.Split (R5):
// a cut wounds several parts, a bomb one, a fixture cut one.

// woundedParts is the distinct parts b's wounds are on.
func woundedParts(b *body.Body) int {
	var seen uint64
	for _, w := range b.Wounds {
		seen |= 1 << w.Part
	}
	n := 0
	for ; seen != 0; seen &= seen - 1 {
		n++
	}
	return n
}

// TestKnifeHitSplits: a knife (FormCut) hit on a raider through the real
// hit path (Absorb, HurtHit) can wound several parts in one tick, with the
// cut's total (R1: 8 on one part, 11 on two, 13 on three).
func TestKnifeHitSplits(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		seed         uint64
		parts, total int
	}{{1, 1, 8}, {2, 3, 13}, {3, 2, 11}} {
		g := New(Config{Seed: c.seed, Width: 12, Height: 8})
		rid := spawnRaiderAt(t, g, 4, 4, 0)
		knife := weapon.HitOf(weapon.Knife, weapon.DefOf(weapon.Knife))
		knife.Amount = 8
		hits(g).HurtHit(rid, hits(g).Absorb(rid, knife, g.rng.IntN), 0)
		b := g.bodies[rid]
		if woundedParts(b) != c.parts || b.Damage() != c.total || b.Missing != 0 || g.VitalOf(rid).Injury != c.total {
			t.Fatalf("seed %d: a knife hit wounds %d parts for %d: %+v", c.seed, c.parts, c.total, b)
		}
		for _, w := range b.Wounds {
			if w.Kind != body.Cut || w.At != uint64(g.clock.Now()) {
				t.Fatalf("seed %d: every share is a cut of this tick: %+v", c.seed, w)
			}
		}
	}
}

// TestBlastWoundsOnePart: a blast (FormBomb) has no split: on a humanlike
// and on an animal each blow wounds one part with all its damage.
func TestBlastWoundsOnePart(t *testing.T) {
	t.Parallel()
	const dmg = 3 // under every part's hit points: a wound, never a loss
	for seed := uint64(1); seed <= 10; seed++ {
		g, _, dog := bondYard(t, seed)
		rid := spawnRaiderAt(t, g, 8, 8, 0)
		bomb := damage.Hit{Form: damage.FormBomb, Amount: dmg}
		hits(g).HurtHit(rid, hits(g).Absorb(rid, bomb, g.rng.IntN), 0)
		hits(g).HurtHit(dog, hits(g).Absorb(dog, bomb, g.rng.IntN), 0)
		for name, b := range map[string]*body.Body{"raider": g.bodies[rid], "dog": animalBody(g, dog)} {
			if b == nil || len(b.Wounds) != 1 || b.Wounds[0].Dmg != dmg || b.Missing != 0 {
				t.Fatalf("seed %d: a blast wounds one part of the %s with all %d: %+v", seed, name, dmg, b)
			}
		}
	}
}

// TestInjureCutUnsplit: the fixture cut (InjureCut, hurtAs) carries no
// split, so it still wounds exactly one part with exactly dmg, on a
// humanlike and on an animal (whose blows all go through body.SpreadBlow).
func TestInjureCutUnsplit(t *testing.T) {
	t.Parallel()
	const dmg = 5
	for seed := uint64(1); seed <= 10; seed++ {
		g, ada, dog := bondYard(t, seed)
		g.InjureCut(ada, dmg)
		g.InjureCut(dog, dmg)
		for name, b := range map[string]*body.Body{"Ada": g.bodies[ada], "dog": animalBody(g, dog)} {
			if b == nil || len(b.Wounds) != 1 || b.Wounds[0].Dmg != dmg || b.Wounds[0].Kind != body.Cut {
				t.Fatalf("seed %d: InjureCut wounds one part of %s with %d: %+v", seed, name, dmg, b)
			}
		}
		if a, _ := g.animals.Get(dog); g.VitalOf(ada).Injury != dmg || a.Injury != dmg {
			t.Fatalf("seed %d: Injury %d and %d, want %d", seed, g.VitalOf(ada).Injury, a.Injury, dmg)
		}
	}
}
