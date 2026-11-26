// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import (
	"rimworld-go/internal/domain/economy/item"
	"testing"
)

func pickDamageTestPart(b *Body, target Part) int {
	for r := 0; r < 1000; r++ {
		p, ok := b.PickPart(func(n int) int {
			if r >= n {
				return n
			}
			return r
		})
		if ok && p == target {
			return r
		}
	}
	return -1
}

func TestUnplacedFittedHeart(t *testing.T) {
	var natural Body
	natural.hurtPart(Heart, 24, Bruise, 1)
	r := pickDamageTestPart(&natural, Heart)
	Spread(&natural, 1, Bruise, func(int) int { return r }, 2)
	t.Logf("natural heart damage=%d vitalLost=%v", natural.Damage(), natural.VitalLost())
	if natural.Damage() != 25 || natural.VitalLost() {
		t.Fatal("natural control failed")
	}
	var fitted Body
	fitted.destroy(Heart)
	if !fitted.Apply(Operation{Kind: InstallImplant, Part: Heart, Item: item.PumpHeart}, Success, nil, 1) {
		t.Fatal("failed to fit heart")
	}
	r = pickDamageTestPart(&fitted, Heart)
	if r < 0 {
		t.Fatal("fitted heart not selectable")
	}
	fitted.hurtPart(Heart, 24, Bruise, 2)
	t.Logf("before spread damage=%d vitalLost=%v", fitted.Damage(), fitted.VitalLost())
	before := fitted.Damage()
	Spread(&fitted, 1, Bruise, func(int) int { return r }, 3)
	if fitted.VitalLost() || fitted.Damage() != before+1 {
		t.Fatalf("unplaced damage violated lethal-part protection: %d to %d", before, fitted.Damage())
	}
	t.Logf("after 1-point spread damage=%d vitalLost=%v", fitted.Damage(), fitted.VitalLost())
}

func TestQuadrupedAsthmaAncestry(t *testing.T) {
	var healthy = Body{Anatomy: Quadruped}
	if !healthy.CanDevelop(CondAsthma, QLungL) || !healthy.CanDevelop(CondAsthma, QLungR) {
		t.Fatal("healthy controls failed")
	}
	b := Body{Anatomy: Quadruped}
	b.destroy(QJaw)
	if !b.CanDevelop(CondAsthma, QLungL) || !b.CanDevelop(CondAsthma, QLungR) {
		t.Fatal("missing jaw prevented intact lung eligibility")
	}
	t.Logf("jaw missing=%v torso missing=%v lungs missing=%v/%v CanDevelop=%v/%v DevelopAsthma=%v", b.isMissing(QJaw), b.isMissing(QTorso), b.isMissing(QLungL), b.isMissing(QLungR), b.CanDevelop(CondAsthma, QLungL), b.CanDevelop(CondAsthma, QLungR), DevelopAsthma(&b, 1))
}

// The live animal damage budget must apply to every native split, not just
// the first Cut/Blunt implementations. An old healed loss is not a lethal
// threshold, and a genuinely exhausted scalar budget stops later shares.
func TestAnimalNewSplitFormsRespectScalarBudget(t *testing.T) {
	for _, c := range []struct {
		name         string
		s            Split
		kind         WoundKind
		heart, torso int
	}{
		{"scratch", SplitScratch, Scratch, 5, 5},
		{"stab", SplitStab, Stab, 4, 7},
		{"bullet", SplitBullet, Gunshot, 10, 10},
		{"poke", SplitPoke, Bruise, 4, 7},
		{"ranged stab", SplitRangedStab, Stab, 4, 7},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, stopped := range []bool{false, true} {
				b := Body{Anatomy: Quadruped, Scale: 2000}
				b.Missing = bit(QLegFL) | bit(QLegFR) | bit(QLegHL) | bit(QLegHR)
				if !bodyDead(&b) {
					t.Fatal("fixture must have enough historical loss to trigger anatomical death")
				}
				r := pickDamageTestPart(&b, QHeart)
				if r < 0 {
					t.Fatal("heart not selectable")
				}
				draws := 0
				roll := func(n int) int {
					draws++
					if draws != 1 || r >= n {
						t.Fatalf("unexpected draw %d bound %d", draws, n)
					}
					return r
				}
				left, want := 100, c.heart+c.torso
				torso := c.torso
				if stopped {
					left = c.heart
					want = c.heart
					torso = 0
				}
				got := SpreadBlowWithin(&b, 10, c.kind, c.s, left, roll, 1)
				if got != want || b.partDmg(QHeart) != c.heart || b.partDmg(QTorso) != torso || draws != 1 {
					t.Fatalf("stop %v: total %d heart %d torso %d draws %d; want %d/%d/%d/1", stopped, got, b.partDmg(QHeart), b.partDmg(QTorso), draws, want, c.heart, torso)
				}
			}
		})
	}
}

func TestReplacementIsSingleTargetForNewSplits(t *testing.T) {
	for _, s := range []Split{SplitScratch, SplitStab, SplitBullet, SplitPoke, SplitRangedStab} {
		b := Body{Missing: bit(Heart), Replacements: ReplacementsFrom([]item.Kind{Heart: item.PumpHeart})}
		if StrikeBlow(&b, Heart, 5, Stab, s, 0, 100, splitNever, 1) || replacementDamage(&b, Heart) != 5 || len(b.Wounds) != 0 {
			t.Fatalf("split %d damaged outside the installed replacement: %+v", s, b)
		}
	}
}
