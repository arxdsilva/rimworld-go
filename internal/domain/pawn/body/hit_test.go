// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import (
	"testing"

	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/economy/item"
)

func TestWoundOf(t *testing.T) {
	t.Parallel()
	for f, want := range map[damage.Form]WoundKind{damage.FormBullet: Gunshot, damage.FormArrow: Stab, damage.FormStab: Stab,
		damage.FormCut: Cut, damage.FormBlunt: Bruise, damage.FormBite: Bite, damage.FormScratch: Scratch,
		damage.FormFlame: Burn, damage.FormBomb: Cut, damage.FormPoke: Bruise, damage.FormRangedStab: Stab,
		damage.Form(99): Bruise} {
		if WoundOf(f) != want {
			t.Errorf("%v leaves %d", f, WoundOf(f))
		}
	}
	for _, k := range []WoundKind{Gunshot, Stab} {
		if BleedRate(Wound{Part: Torso, Dmg: 5, Kind: k}) == 0 || ScarOdds(k, 14) == 0 {
			t.Errorf("kind %d bleeds and scars", k)
		}
	}
}

// TestStrikeOverkill: a hit within a part's hit points wounds it; one
// past them destroys it by the window or leaves it at 1 HP; the brain's
// loss is vital.
func TestStrikeOverkill(t *testing.T) {
	t.Parallel()
	never := func(int) int { panic("no roll when the chance is none or certain") }
	var b Body
	if Strike(&b, ArmL, 10, Cut, 0, 100, never, 1) || partDmgOf(&b, ArmL) != 10 {
		t.Fatal("a plain wound")
	}
	// 25 more on 25 left: overkill 0 -> kept at 1 HP, no roll
	if Strike(&b, ArmL, 25, Bruise, 400, 1000, never, 2) || partDmgOf(&b, ArmL) != 34 || b.PartMissing(ArmL) {
		t.Fatalf("kept at 1 HP: %d", partDmgOf(&b, ArmL))
	}
	// blunt overkill 13/35 = 371 permille, under 400: never destroyed
	var c Body
	if Strike(&c, ArmR, 48, Bruise, 400, 1000, never, 1) || c.PartMissing(ArmR) {
		t.Fatal("blunt under its window")
	}
	// bullet overkill 10/40 = 250 permille in 0..700: chance 357, rolled
	var d Body
	if !Strike(&d, LegL, 50, Gunshot, 0, 700, func(int) int { return 356 }, 1) || !d.PartMissing(LegL) || !d.PartMissing(FootL) {
		t.Fatal("destroyed under the chance, with the foot")
	}
	var e Body
	if Strike(&e, LegL, 50, Gunshot, 0, 700, func(int) int { return 357 }, 1) || e.PartMissing(LegL) {
		t.Fatal("kept over the chance")
	}
	var f Body
	if !Strike(&f, Brain, 60, Gunshot, 0, 700, never, 1) || !f.VitalLost() {
		t.Fatal("a certain overkill on the brain is vital")
	}
	if Strike(&f, Brain, 5, Cut, 0, 100, never, 2) || Strike(&f, Part(200), 5, Cut, 0, 100, never, 2) || Strike(&f, Torso, 0, Cut, 0, 100, never, 2) {
		t.Fatal("a missing or unknown part, or no damage, takes nothing")
	}
}

func partDmgOf(b *Body, p Part) int { return b.partDmg(p) }

// A replacement takes damage itself, not a cut split into organic children.
func TestStrikeBlowReplacement(t *testing.T) {
	b := Body{Missing: bit(LegL) | bit(FootL), Replacements: ReplacementsFrom([]item.Kind{LegL: item.PegLeg})}
	never := func(int) int { panic("no split or overkill roll expected") }
	if StrikeBlow(&b, LegL, 5, Cut, SplitCut, 0, 100, never, 1) || replacementDamage(&b, LegL) != 5 || len(b.Wounds) != 0 {
		t.Fatalf("replacement damage not isolated: %+v", b)
	}
	if !StrikeBlow(&b, LegL, b.maxHP(LegL)*2, Cut, SplitCut, 0, 100, never, 2) || b.hasReplacement(LegL) {
		t.Fatal("destroyed replacement survived overkill")
	}
}
