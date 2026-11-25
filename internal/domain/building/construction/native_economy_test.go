// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package construction

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

func TestAX2NativeConstructionCostsAndWork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		kind  Kind
		stuff item.Kind
		cost  int
		work  int
		extra [3]Cost
	}{
		{"ornate door", OrnateDoor, item.Wood, 75, 15000, [3]Cost{{Item: item.Gold, N: 50}}},
		{"column", Column, item.Wood, 20, 750, [3]Cost{}},
		{"animal flap", AnimalFlap, item.Cloth, 25, 850, [3]Cost{}},
		{"animal sleeping box", AnimalSleepingBox, item.Wood, 25, 180, [3]Cost{}},
		{"table 2x2", Table2x2, item.Wood, 50, 1500, [3]Cost{}},
		{"table 2x4", Table2x4, item.Wood, 95, 3000, [3]Cost{}},
		{"table 3x3", Table3x3, item.Wood, 100, 3300, [3]Cost{}},
		{"couch", Couch, item.Cloth, 200, 20000, [3]Cost{}},
		{"plant pot", PlantPot, item.Wood, 20, 250, [3]Cost{}},
		{"small shelf", SmallShelf, item.Wood, 10, 250, [3]Cost{}},
		{"wall torch", WallTorch, item.Wood, 15, 100, [3]Cost{}},
		{"wall lamp", WallLamp, item.Steel, 15, 330, [3]Cost{}},
		{"flood light", FloodLight, item.Steel, 50, 600, [3]Cost{}},
		{"large stele", LargeStele, item.GraniteBlocks, 125, 8000, [3]Cost{}},
		{"grand stele", GrandStele, item.GraniteBlocks, 250, 16000, [3]Cost{}},
		{"egg box", EggBox, item.Wood, 20, 2000, [3]Cost{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := DefOf(tc.kind)
			if d.Name != tc.name || d.Wood != tc.cost || d.Work != tc.work {
				t.Errorf("native definition = %q %d/%d, want %q %d/%d", d.Name, d.Wood, d.Work, tc.name, tc.cost, tc.work)
			}
			if got := CostFor(tc.kind, tc.stuff); got != tc.cost {
				t.Errorf("native primary cost = %d, want %d", got, tc.cost)
			}
			_, workPct := StuffStats(tc.stuff)
			if got, want := WorkFor(tc.kind, tc.stuff), tc.work*workPct/100; got != want {
				t.Errorf("stuff-adjusted work = %d, want %d", got, want)
			}
			if !StuffOK(tc.kind, tc.stuff) {
				t.Errorf("material %v is not accepted", tc.stuff)
			}
			if d.Extra != tc.extra {
				t.Errorf("secondary materials = %+v, want %+v", d.Extra, tc.extra)
			}
		})
	}

	urn := DefOf(Urn)
	if urn.Name != "urn" || urn.Wood != 20 || urn.Work != 50 || !urn.Unbuildable {
		t.Fatalf("urn must retain its native stored cost and original reinstall work: %+v", urn)
	}
	if got := CostFor(Urn, item.Wood); got != 20 {
		t.Fatalf("urn stored material cost = %d, want 20", got)
	}
	if !DoubleSleepingSpot.Instant() {
		t.Fatal("double sleeping spot is free placement")
	}
}

func TestNativeResearchAndFurnitureConstructionRows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		kind       Kind
		stuff      item.Kind
		cost       int
		checkCost  bool
		work       int
		components int
		extra      [3]Cost
	}{
		{"table", Table, item.Wood, 28, true, 750, 0, [3]Cost{}},
		{"research bench", ResearchBench, item.Wood, 75, true, 2800, 0, [3]Cost{{Item: item.Steel, N: 25}}},
		{"electric research bench", ElectricResearchBench, item.Steel, 150, true, 5000, 10, [3]Cost{{Item: item.Steel, N: 100}}},
		{"bed", Bed, item.Wood, 0, false, 800, 0, [3]Cost{}},
		{"double bed", DoubleBed, item.Wood, 0, false, 1500, 0, [3]Cost{}},
		{"hospital bed", HospitalBed, item.Wood, 0, false, 2800, 2, [3]Cost{}},
		{"bedroll", Bedroll, item.Cloth, 0, false, 600, 0, [3]Cost{}},
		{"double bedroll", DoubleBedroll, item.Leather, 0, false, 1100, 0, [3]Cost{}},
		{"animal bed", AnimalBed, item.Cloth, 0, false, 400, 0, [3]Cost{}},
		{"end table", EndTable, item.Wood, 30, true, 1000, 0, [3]Cost{}},
		{"dresser", Dresser, item.Wood, 50, true, 2000, 0, [3]Cost{}},
		{"stool", Stool, item.Wood, 25, true, 450, 0, [3]Cost{}},
		{"armchair", Armchair, item.Cloth, 110, true, 14000, 0, [3]Cost{}},
		{"shelf", Shelf, item.Wood, 20, true, 500, 0, [3]Cost{}},
		{"multi-analyzer", MultiAnalyzer, item.Steel, 40, true, 10000, 8, [3]Cost{{Item: item.Plasteel, N: 50}, {Item: item.Gold, N: 20}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := DefOf(tc.kind)
			if d.Name != tc.name || d.Work != tc.work || d.Components != tc.components {
				t.Errorf("native definition = %q work=%d components=%d, want %q work=%d components=%d",
					d.Name, d.Work, d.Components, tc.name, tc.work, tc.components)
			}
			if tc.checkCost {
				if d.Wood != tc.cost {
					t.Errorf("native primary units = %d, want %d", d.Wood, tc.cost)
				}
				if got := CostFor(tc.kind, tc.stuff); got != tc.cost {
					t.Errorf("native primary cost = %d, want %d", got, tc.cost)
				}
			}
			_, workPct := StuffStats(tc.stuff)
			if got, want := WorkFor(tc.kind, tc.stuff), tc.work*workPct/100; got != want {
				t.Errorf("stuff-adjusted work = %d, want %d", got, want)
			}
			if !StuffOK(tc.kind, tc.stuff) {
				t.Errorf("material %v is not accepted", tc.stuff)
			}
			if d.Extra != tc.extra {
				t.Errorf("secondary materials = %+v, want %+v", d.Extra, tc.extra)
			}
		})
	}
}
