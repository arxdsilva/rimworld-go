package room

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/world/filth"
)

func TestTallyFloorsAndFilth(t *testing.T) {
	t.Parallel()
	// 16 bare cells: beauty -16 over a divisor of 28, cleanliness -1 each
	var bare Tally
	in := bare.Inputs(16, 0, 0, 0)
	if in.BeautySumTenths != -160 || in.CleanlinessHundredths != -100 || in.Wealth != 0 {
		t.Fatalf("bare %+v", in)
	}
	// wood floor everywhere: beauty 0, cleanliness 0; one carpet over wood +2
	var wood Tally
	for range 16 {
		wood.AddFloor(item.Wood)
	}
	wood.AddCarpet(WoodFloor, item.CarpetCloth)
	in = wood.Inputs(16, 0, 0, 0)
	if in.BeautySumTenths != 20 || in.CleanlinessHundredths != 0 || in.Wealth <= 0 || wood.Covered != 16 {
		t.Fatalf("wood %+v %+v", in, wood)
	}
	// smooth stone and stone tiles are +1; a carpet on bare ground +3
	var mixed Tally
	mixed.AddFloor(item.GraniteBlocks)
	mixed.AddCarpet(Bare, item.CarpetWool)
	in = mixed.Inputs(4, 2, 0, 0) // 1 tile, 2 smooth, 1 bare under the carpet
	if in.BeautySumTenths != 10+20-10+30 || FloorOf(item.Stone) != StoneTile || FloorUnder(item.Wood, true, true) != WoodFloor ||
		FloorUnder(0, false, true) != Smooth || FloorUnder(0, false, false) != Bare {
		t.Fatalf("mixed %+v", in)
	}
	// a full cell's worth of filth: -15 beauty, -5 cleanliness
	in = wood.Inputs(16, 0, filth.Full, 0)
	if in.BeautySumTenths != 20-150 || in.CleanlinessHundredths != -500/16 {
		t.Fatalf("filthy %+v", in)
	}
}

func TestTallyBuildingsAndRoles(t *testing.T) {
	t.Parallel()
	var tl Tally
	tl.AddBuilding(construction.Bed, item.Wood, quality.Default, 2)
	tl.AddBuilding(construction.Chair, item.Wood, quality.Default, 1)
	tl.AddBuilding(construction.Dresser, item.Wood, quality.Default, 2)
	if tl.Impassable != 4 || tl.BeautyTenths != 10+80+50 { // bed 1, dining chair 8 (BED-3), dresser 5
		t.Fatalf("bedroom tally %+v", tl)
	}
	var fine Tally
	fine.AddBuilding(construction.Bed, item.Wood, quality.Legendary, 2)
	if fine.BeautyTenths <= 10 || fine.Wealth <= BuildingValue(construction.Bed, item.Wood, quality.Default) {
		t.Fatalf("legendary bed %+v", fine)
	}
	if BuildingBeauty(construction.HospitalBed, item.Gold, quality.Default) != 20000 ||
		BuildingBeauty(construction.Bed, item.MarbleBlocks, quality.Default) != 1350+1000 || BuildingBeauty(construction.Table, item.Wood, quality.Default) != 0 {
		t.Fatal("material beauty")
	}
	if BuildingValue(construction.Wall, item.GraniteBlocks, quality.Default) <= BuildingValue(construction.Wall, item.Wood, quality.Default) {
		t.Fatal("stone is worth more")
	}
	tl.AddBuilding(construction.HorseshoePin, item.Wood, quality.Default, 1)
	tl.AddBuilding(construction.Stove, item.Wood, quality.Default, 2)
	if tl.RecBuildings != 1 || tl.Stoves != 1 {
		t.Fatalf("role furniture %+v", tl)
	}
	c := Contents{Cells: 16, Beds: 1}
	if tl.Role(c, 0, 0) != Bedroom || tl.Role(c, 1, 0) != Hospital || tl.Role(c, 0, 1) != PrisonCell {
		t.Fatal("roles from the tally")
	}
	if st := tl.Stats(c, 0, 1500); st.Wealth != tl.Wealth+1500 || st.SpaceTenths != Space(16, tl.Impassable) {
		t.Fatalf("stats %+v", st)
	}
	if ItemValue(item.Stack{Kind: item.Meal, Qty: 3}) != 3*item.Meal.Value() ||
		ItemValue(item.Stack{Kind: item.Parka, Qty: 1, Quality: quality.Stored(quality.Masterwork)}) <= item.Parka.Value() {
		t.Fatal("item value")
	}
}

func TestImpressivenessStages(t *testing.T) {
	t.Parallel()
	for v, want := range map[int]int{-50: 0, 199: 0, 200: 1, 499: 3, 500: 4, 2399: 8, 2400: 9, 99999: 9} {
		if ImpressivenessStage(v) != want {
			t.Fatalf("stage %d = %d", v, ImpressivenessStage(v))
		}
	}
	if StageName(-1) != "" || StageName(Stages) != "" || StageName(5) != "somewhat impressive" {
		t.Fatal("stage names")
	}
}

func TestAddAndCountFacts(t *testing.T) {
	t.Parallel()
	var c Contents
	var tl Tally
	for _, b := range []construction.Building{{Kind: construction.Floor, Stuff: item.Wood}, {Kind: construction.Carpet, Stuff: item.CarpetCloth},
		{Kind: construction.Bed}, {Kind: construction.Table}, {Kind: construction.Chair}, {Kind: construction.Lamp},
		{Kind: construction.Workbench}, {Kind: construction.Bed, Quality: quality.Stored(quality.Excellent)}} {
		Add(&c, &tl, b, 1, Bare)
	}
	if c != (Contents{Floored: 1, Beds: 2, Tables: 1, Chairs: 1, Lamps: 1, Benches: 1}) || tl.Covered != 1 || tl.BeautyTenths <= 20 {
		t.Fatalf("add %+v %+v", c, tl)
	}
	// a 3x1 map: room 0 at x 1, outdoors elsewhere
	grid := []int{Outdoors, 0, Outdoors}
	stacks := component.NewStore[item.Stack]()
	stacks.Set(1, item.Stack{Kind: item.Gold, Qty: 10, X: 1})
	stacks.Set(2, item.Stack{Kind: item.Gold, Qty: 10, X: 1, Carrier: 9}) // carried: not the room's
	stacks.Set(3, item.Stack{Kind: item.Gold, Qty: 10, X: 0})             // outdoors
	stacks.Set(4, item.Stack{Kind: item.Gold, Qty: 10, X: 5, Y: 5})       // off the grid
	bldgs := component.NewStore[construction.Building]()
	bldgs.Set(5, construction.Building{Kind: construction.Bed, X: 1, Medical: true})
	bldgs.Set(6, construction.Building{Kind: construction.Bed, X: 1, Medical: true, ForPrisoners: true})
	bldgs.Set(7, construction.Building{Kind: construction.Bed, X: 1})
	f := CountFacts(nil, grid, 3, 1, stacks, bldgs)
	if len(f) != 1 || f[0] != (Fact{Wealth: 10 * item.Gold.Value(), MedicalBeds: 1, PrisonerBeds: 1}) {
		t.Fatalf("facts %+v", f)
	}
	if g := CountFacts(f, grid, 3, 1, stacks, bldgs); &g[0] != &f[0] || g[0] != f[0] {
		t.Fatal("scratch reused and refilled the same")
	}
}
