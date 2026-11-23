// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package gear

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/combat/weapon"
)

// TestSHT203WeaponTooltip (SHT-203): the Gear tab's weapon row tooltip, as
// Rows builds it (WeaponTip plus WeaponLines), holds the four accuracy
// bands and the burst count of a ranged weapon; a melee weapon has none.
func TestSHT203WeaponTooltip(t *testing.T) {
	t.Parallel()
	tipOf := func(p game.PawnView) string {
		for _, r := range Rows(p, newFake()) {
			if r.What == "weapon" {
				return r.Tip
			}
		}
		t.Fatal("no weapon row")
		return ""
	}
	ranged := tipOf(game.PawnView{Weapon: "rifle", WeaponStats: weapon.View{Name: "Rifle", Damage: 12, Warmup: 90, Cooldown: 120,
		Burst: 3, Range: 25, Accuracy: [4]int{900, 800, 600, 400}}})
	for _, w := range []string{"touch 90%", "short 80%", "medium 60%", "long 40%", "Burst 3"} {
		if !strings.Contains(ranged, w) {
			t.Errorf("ranged tooltip lacks %q:\n%s", w, ranged)
		}
	}
	melee := tipOf(game.PawnView{Weapon: "club", WeaponStats: weapon.View{Name: "Club", Melee: true, Cooldown: 90}})
	for _, w := range []string{"touch", "Burst"} {
		if strings.Contains(melee, w) {
			t.Errorf("melee tooltip has %q:\n%s", w, melee)
		}
	}
}
