// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package construction

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

func TestAX3NativeFloorCostsAndWork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		kind      Kind
		stuff     item.Kind
		cost      int
		work      int
		fixedWork bool
	}{
		{"floor", Floor, item.Wood, 3, 85, false},
		{"straw matting", StrawMatting, item.Hay, 2, 120, true},
		{"concrete", Concrete, item.Steel, 1, 100, true},
		{"paved tile", PavedTile, item.Steel, 2, 300, true},
		{"steel tile", SteelTile, item.Steel, 7, 800, true},
		{"silver tile", SilverTile, item.Silver, 70, 800, true},
		{"gold tile", GoldTile, item.Gold, 70, 800, true},
		{"sterile tile", SterileTile, item.Steel, 3, 1600, true},
		{"stone tile", StoneTile, item.GraniteBlocks, 4, 1100, true},
		{"flagstone", Flagstone, item.GraniteBlocks, 4, 500, true},
		{"carpet floor", CarpetFloor, item.Cloth, 7, 800, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := DefOf(tc.kind)
			if d.Name != tc.name || d.Wood != tc.cost || CostFor(tc.kind, tc.stuff) != tc.cost {
				t.Errorf("native material definition = %q %d/%d, want %q %d", d.Name, d.Wood, CostFor(tc.kind, tc.stuff), tc.name, tc.cost)
			}
			if d.Work != tc.work || WorkFor(tc.kind, tc.stuff) != tc.work {
				t.Errorf("native work = %d/%d, want %d", d.Work, WorkFor(tc.kind, tc.stuff), tc.work)
			}
			if got := FixedWork(tc.kind); got != tc.fixedWork {
				t.Errorf("FixedWork = %t, want %t", got, tc.fixedWork)
			}
			if !StuffOK(tc.kind, tc.stuff) {
				t.Errorf("material %v is not accepted", tc.stuff)
			}
		})
	}

	sterile := DefOf(SterileTile)
	if got := sterile.Extra[0]; got.Item != item.Silver || got.N != 12 {
		t.Fatalf("sterile tile secondary cost = %+v, want 12 silver", got)
	}
}
