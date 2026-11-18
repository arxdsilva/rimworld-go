// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package overlay

import "testing"

func TestFertilityTint(t *testing.T) {
	t.Parallel()
	if FertilityTint(0).A != 0 {
		t.Error("barren ground has no tint")
	}
	poor, soil, rich := FertilityTint(70), FertilityTint(100), FertilityTint(140)
	if poor == soil || soil == rich || poor.A == 0 || rich.A == 0 {
		t.Errorf("tints %v %v %v", poor, soil, rich)
	}
}
