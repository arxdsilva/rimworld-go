package recipe

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

func ids(ds []Def) []ID {
	out := make([]ID, len(ds))
	for i, d := range ds {
		out[i] = d.ID
	}
	return out
}

// TestBenchMapping: each PRD-3 bench runs its recipes, electric twins
// follow their base bench, moved recipes stay at their old bench.
func TestBenchMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		bench construction.Kind
		has   []ID
		not   []ID
	}{
		{construction.CraftingSpot, []ID{Club, Knife, MakeShortBow, MakeGreatbow}, []ID{Spear, MakeMace, MakeRevolver}},
		{construction.FueledSmithy, []ID{Club, Knife, Spear, MakeMace, MakeGladius, MakeLongsword, MakePila, MakeShortBow}, []ID{MakeRevolver}},
		{construction.ElectricSmithy, []ID{MakeMace, MakeLongsword, Spear}, []ID{MakeRevolver}},
		{construction.MachiningTable, []ID{MakeRevolver, MakeMinigun, MakeEMPGrenades, MakeRifle, MakeProstheticLeg}, []ID{MakeMace}},
		{construction.FabricationBench, []ID{FabricateComponent, MakeServoArm}, []ID{MakeRevolver}},
		{construction.ElectricSmelter, []ID{SmeltWeapon}, nil},
		{construction.ArtBench, []ID{SculptSmall, SculptLarge, SculptGrand}, nil},
		{construction.ButcherSpot, []ID{ButcherAnimal}, []ID{ButcherDustbuck}},
		{construction.Workbench, []ID{Club, Knife, Spear, MakeRifle, FabricateComponent, MakeServoArm}, []ID{MakeMace}},
		{construction.ElectricStove, []ID{SimpleMeal}, nil},
		{construction.BiofuelRefinery, nil, []ID{SimpleMeal}},
	}
	for _, tc := range cases {
		got := ids(For(tc.bench))
		for _, id := range tc.has {
			if !slices.Contains(got, id) {
				t.Errorf("%s does not run %s", tc.bench, DefOf(id).Name)
			}
		}
		for _, id := range tc.not {
			if slices.Contains(got, id) {
				t.Errorf("%s runs %s", tc.bench, DefOf(id).Name)
			}
		}
	}
	if Base(construction.ElectricSmithy) != construction.FueledSmithy || Base(construction.ElectricStove) != construction.Stove ||
		Base(construction.ArtBench) != construction.ArtBench {
		t.Fatal("base benches")
	}
}

// TestNewRecipes: the smithy's blades are stuffed (steel by default), the
// guns take steel and components, sculptures build their kind from its
// material count.
func TestNewRecipes(t *testing.T) {
	t.Parallel()
	m := DefOf(MakeMace)
	if !m.Stuffed() || m.Input.Kind != item.Steel || !m.Input.Accepts(item.Wood) || m.Input.Accepts(item.GraniteBlocks) {
		t.Fatalf("mace %+v", m.Input)
	}
	r := DefOf(MakeRevolver)
	if r.InputsLabel() != "3 steel + 2 components" || r.Output != item.Revolver {
		t.Fatalf("revolver %q", r.InputsLabel())
	}
	s := DefOf(SculptGrand)
	if s.Builds != construction.GrandSculpture || s.Input.Qty != construction.DefOf(construction.GrandSculpture).Wood ||
		!s.Input.Accepts(item.MarbleBlocks) || !s.Input.Accepts(item.Silver) || s.Input.Accepts(item.Cloth) || !s.Stuffed() {
		t.Fatalf("grand sculpture %+v", s)
	}
	if DefOf(MakeShortBow).Also == nil || DefOf(MakePila).Also != nil {
		t.Fatal("bows at the spot also at the smithy; the pila only at the smithy")
	}
}

// TestSmeltYield: a quarter of the steel back (at least one), only from
// weapons a recipe makes with steel, in steel.
func TestSmeltYield(t *testing.T) {
	t.Parallel()
	cases := []struct {
		st   item.Stack
		want int
	}{
		{item.Stack{Kind: item.Minigun, Qty: 1}, 4},
		{item.Stack{Kind: item.Revolver, Qty: 1}, 1},
		{item.Stack{Kind: item.Longsword, Qty: 1}, 2}, // default steel
		{item.Stack{Kind: item.Longsword, Qty: 1, Stuff: item.Wood}, 0},
		{item.Stack{Kind: item.Club, Qty: 1, Stuff: item.Steel}, 1},
		{item.Stack{Kind: item.Club, Qty: 1}, 0},   // wooden
		{item.Stack{Kind: item.Rifle, Qty: 1}, 0},  // the crude rifle is wood
		{item.Stack{Kind: item.Steel, Qty: 10}, 0}, // not a weapon
		{item.Stack{Kind: item.Revolver, Qty: 2}, 2},
	}
	for _, tc := range cases {
		if got := SmeltYield(tc.st); got != tc.want {
			t.Errorf("%s/%s: %d, want %d", tc.st.Kind, tc.st.Stuff, got, tc.want)
		}
	}
	in := DefOf(SmeltWeapon).Input
	if !in.Accepts(item.Revolver) || !in.Accepts(item.Club) || in.Accepts(item.Rifle) || in.Accepts(item.Steel) {
		t.Fatal("smeltable kinds")
	}
	if !in.AcceptsStack(item.Stack{Kind: item.Mace, Qty: 1}) || in.AcceptsStack(item.Stack{Kind: item.Club, Qty: 1}) {
		t.Fatal("smeltable stacks")
	}
	if in.Label() != "1 metal weapon" {
		t.Fatalf("label %q", in.Label())
	}
}
