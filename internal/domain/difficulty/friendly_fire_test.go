// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package difficulty

import (
	"strings"
	"testing"
)

func TestFriendlyFireDefaultsAndLegacyUpgrade(t *testing.T) {
	t.Parallel()
	for _, n := range Presets() {
		if n.Settings.FriendlyFire != DefaultFriendlyFire || n.Settings.FriendlyFirePercent() != 40 || n.Settings.BalanceVersion != 3 {
			t.Fatalf("%s: expected explicit vanilla 40, got %+v", n.Key, n.Settings)
		}
	}
	if (Settings{}).FriendlyFirePercent() != 40 || (Settings{FriendlyFire: 100}).FriendlyFirePercent() != 100 {
		t.Fatal("legacy settings must retain vanilla default")
	}
	custom := Of(Seasoned)
	custom.FriendlyFire = 0
	if custom.FriendlyFirePercent() != 0 || Clamp(custom).FriendlyFire != 0 {
		t.Fatal("explicit zero was lost")
	}
	encode := Encode(custom)
	decoded, err := Decode(encode)
	if err != nil || decoded.FriendlyFirePercent() != 0 || decoded.BalanceVersion != 3 {
		t.Fatalf("explicit zero round-trip: %+v %v", decoded, err)
	}
	old := strings.Replace(Encode(Of(Seasoned)), ",ff=40", "", 1)
	restored, err := Decode(old)
	if err != nil || restored.FriendlyFirePercent() != 100 {
		t.Fatalf("legacy command payload: %+v %v", restored, err)
	}
	legacy := Settings{FriendlyFire: 0}
	upgraded := Clamp(legacy)
	if upgraded.FriendlyFire != 40 || upgraded.BalanceVersion != 3 {
		t.Fatalf("legacy editor upgrade: %+v", upgraded)
	}
	if len(Valid(Settings{FriendlyFire: 101})) == 0 {
		t.Fatal("101% should be refused")
	}
}

// Version 1 stored explicit friendly-fire settings before adaptation-growth
// and food poisoning received their own controls. Loading and resaving v1
// keeps an explicitly disabled friendly-fire slider disabled.
func TestVersionOneFriendlyFireSurvivesDifficultyUpgrade(t *testing.T) {
	t.Parallel()
	v1 := Settings{BalanceVersion: 1, FriendlyFire: 0}
	got := v1.Upgraded()
	if got.BalanceVersion != 3 || got.FriendlyFire != 0 ||
		got.AdaptGrowth != 100 || got.FoodPoisonChance != 100 {
		t.Fatalf("v1 to v2 difficulty settings: %+v", got)
	}
	newZero := got
	newZero.FoodPoisonChance = 0
	newZero.AdaptGrowth = 0
	if newZero.Upgraded() != newZero {
		t.Fatalf("explicit new zero multipliers were modified: %+v", newZero.Upgraded())
	}
}
