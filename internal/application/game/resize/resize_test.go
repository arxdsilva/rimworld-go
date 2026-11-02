package resize

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

// TestV10IsTheWikiBenches: save v10 migrates exactly the kinds STK-BN
// resized.
func TestV10IsTheWikiBenches(t *testing.T) {
	t.Parallel()
	want := map[construction.Kind]bool{}
	for _, k := range []construction.Kind{construction.Workbench, construction.Stove, construction.ButcherTable,
		construction.TailoringBench, construction.StonecutterTable, construction.BrewingBench, construction.DrugTable,
		construction.ElectricStove, construction.ElectricTailoringBench, construction.FueledSmithy, construction.ElectricSmithy,
		construction.MachiningTable, construction.ArtBench, construction.ElectricSmelter, construction.FabricationBench,
		construction.BiofuelRefinery, construction.RadioConsole} {
		want[k] = true
	}
	for _, d := range construction.All() {
		if V10(d.Kind) != want[d.Kind] {
			t.Errorf("%s: migrated %v, want %v", d.Kind, V10(d.Kind), want[d.Kind])
		}
	}
	for _, d := range construction.All() {
		if V9(d.Kind) && V10(d.Kind) {
			t.Errorf("%s grew twice", d.Kind)
		}
	}
}

func TestRefunds(t *testing.T) {
	t.Parallel()
	if s, n, c := RefundV9(construction.Building{Kind: construction.ElectricResearchBench}); s != item.Steel || n != 65 || c != 3 {
		t.Fatalf("hi-tech bench refund %v %d %d", s, n, c)
	}
	if s, n, c := RefundV9(construction.Building{Kind: construction.ResearchBench, Stuff: item.Wood}); s != item.Wood || n != 8 || c != 0 {
		t.Fatalf("research bench refund %v %d %d", s, n, c)
	}
	b := construction.Building{Kind: construction.MachiningTable, Stuff: item.Steel}
	if s, n, c := RefundV10(b); s != item.Steel || n != construction.WoodCost(b.Kind) || c != construction.ComponentCost(b.Kind) {
		t.Fatalf("machining table refund %v %d %d", s, n, c)
	}
}
