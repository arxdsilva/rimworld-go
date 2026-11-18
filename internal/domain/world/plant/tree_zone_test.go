// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package plant

import "testing"

func TestZoneSowable(t *testing.T) {
	if !DefOf(Potato).ZoneSowable() || !DefOf(Tree).ZoneSowable() || DefOf(BerryBush).ZoneSowable() {
		t.Fatal("zone sowable: crops and trees only")
	}
	if DefOf(Tree).Crop || Frostbitten(Tree, -900) {
		t.Fatal("trees are not crops and never frost-killed")
	}
}
