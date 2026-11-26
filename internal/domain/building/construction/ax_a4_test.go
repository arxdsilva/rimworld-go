// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package construction

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

func TestAX4NativePowerIndustryCostsAndWork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		kind       Kind
		stuff      item.Kind
		cost       int
		work       int // Chemfuel generator retains original tuning work; the other values are wiki ticks.
		components int
		extra      [3]Cost
	}{
		{"waterproof conduit", WaterproofConduit, item.Steel, 10, 35, 0, [3]Cost{}},
		{"chemfuel generator", ChemfuelGenerator, item.Steel, 100, 320, 3, [3]Cost{}},
		{"watermill generator", Watermill, item.Wood, 280, 4000, 3, [3]Cost{{Item: item.Steel, N: 80}}},
		{"deep drill", DeepDrill, item.Steel, 100, 10000, 2, [3]Cost{}},
		{"ground-penetrating scanner", GroundScanner, item.Steel, 150, 12000, 4, [3]Cost{{Item: item.AdvancedComponents, N: 1}}},
		{"moisture pump", MoisturePump, item.Steel, 75, 1500, 4, [3]Cost{}},
		{"electric crematorium", ElectricCrematorium, item.GraniteBlocks, 150, 4500, 2, [3]Cost{{Item: item.Steel, N: 20}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := DefOf(tc.kind)
			if d.Name != tc.name || d.Wood != tc.cost || CostFor(tc.kind, tc.stuff) != tc.cost {
				t.Errorf("native definition/material = %q %d/%d, want %q %d", d.Name, d.Wood, CostFor(tc.kind, tc.stuff), tc.name, tc.cost)
			}
			if d.Work != tc.work {
				t.Errorf("native work = %d, want %d", d.Work, tc.work)
			}
			if d.Components != tc.components {
				t.Errorf("native components = %d, want %d", d.Components, tc.components)
			}
			if !StuffOK(tc.kind, tc.stuff) {
				t.Errorf("material %v is not accepted", tc.stuff)
			}
			if FixedWork(tc.kind) {
				t.Error("AX-4 power/industry work must retain the selected-stuff work factor")
			}
			_, workPct := StuffStats(tc.stuff)
			wantWork := tc.work * workPct / 100
			if got := WorkFor(tc.kind, tc.stuff); got != wantWork {
				t.Errorf("stuff-adjusted work = %d, want %d", got, wantWork)
			}
			if d.Extra != tc.extra {
				t.Errorf("native secondary materials = %+v, want %+v", d.Extra, tc.extra)
			}
		})
	}
}

func TestAX4SecondaryCostsGateFrameReadiness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		kind   Kind
		stuff  item.Kind
		extras [3]Cost
	}{
		{"ornate door", OrnateDoor, item.Wood, [3]Cost{{Item: item.Gold, N: 50}}},
		{"research bench", ResearchBench, item.Wood, [3]Cost{{Item: item.Steel, N: 25}}},
		{"electric research bench", ElectricResearchBench, item.Steel, [3]Cost{{Item: item.Steel, N: 100}}},
		{"multi-analyzer", MultiAnalyzer, item.Steel, [3]Cost{{Item: item.Plasteel, N: 50}, {Item: item.Gold, N: 20}}},
		{"sterile tile", SterileTile, item.Steel, [3]Cost{{Item: item.Silver, N: 12}}},
		{"watermill generator", Watermill, item.Wood, [3]Cost{{Item: item.Steel, N: 80}}},
		{"ground-penetrating scanner", GroundScanner, item.Steel, [3]Cost{{Item: item.AdvancedComponents, N: 1}}},
		{"electric crematorium", ElectricCrematorium, item.GraniteBlocks, [3]Cost{{Item: item.Steel, N: 20}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := DefOf(tc.kind)
			if d.Extra != tc.extras {
				t.Fatalf("secondary definition = %+v, want %+v", d.Extra, tc.extras)
			}
			need := CostFor(tc.kind, tc.stuff)
			components := ComponentCost(tc.kind)
			fr := Frame{
				Kind:     tc.kind,
				Stuff:    tc.stuff,
				Need:     need,
				Wood:     need,
				CompNeed: components,
				Comp:     components,
			}
			if fr.Ready() {
				t.Fatal("frame is ready before its secondary materials arrive")
			}
			for i, extra := range tc.extras {
				if extra.N == 0 {
					continue
				}
				if got := fr.ExtraNeed(i); got != extra.N {
					t.Errorf("secondary slot %d need = %d, want %d", i, got, extra.N)
				}
				fr.ExtraGot[i] = extra.N - 1
				if fr.Ready() {
					t.Errorf("frame is ready with secondary slot %d short", i)
				}
				fr.ExtraGot[i] = extra.N
			}
			if fr.ExtraNeed(-1) != 0 || fr.ExtraNeed(len(fr.ExtraGot)) != 0 {
				t.Fatal("secondary slot lookup must be bounded")
			}
			if !fr.ExtrasIn() || !fr.Ready() {
				t.Fatal("frame is ready after all primary, component and secondary materials arrive")
			}

			reinstall := Frame{Kind: tc.kind, Reinstall: 1}
			if reinstall.ExtraNeed(0) != 0 || !reinstall.ExtrasIn() || !reinstall.Ready() {
				t.Fatal("reinstall must retain the secondary materials already held")
			}
			swap := Frame{Kind: tc.kind, Swaps: 1}
			if swap.ExtraNeed(0) != 0 || !swap.ExtrasIn() || !swap.Ready() {
				t.Fatal("in-place material swap must retain the secondary materials already held")
			}
		})
	}
}
