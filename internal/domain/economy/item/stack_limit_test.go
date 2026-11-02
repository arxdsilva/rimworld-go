package item

import "testing"

// TestStackLimit (STK-ST, user decision): every kind of the same type
// stacks to StackLimit on a tile; things with per-unit identity
// (weapons, apparel, belts, corpses, implants, organs) and stone chunks
// are one per stack. Bones and the former wiki-limited kinds take 1000.
func TestStackLimit(t *testing.T) {
	t.Parallel()
	want := map[Kind]int{Chemfuel: 1000, Silverdust: 1000, Reagent: 1000, Pemmican: 1000, Plasteel: 1000,
		AdvancedComponents: 1000, Wort: 1000, FineMeal: 1000, SurvivalMeal: 1000,
		Meal: 1000, Wood: 1000, Silver: 1000, Gold: 1000, Components: 1000, Meat: 1000, Milk: 1000, Egg: 1000, FertileEgg: 1000,
		HerbalMedicine: 1000, Beer: 1000, GraniteBlocks: 1000, MortarShells: 1000, AshleafTea: 1000, Steel: 1000, Cloth: 1000,
		Bones: 1000, NutrientPasteMeal: 1000, CarpetWool: 1000, Straw: 1000, Rice: 1000,
		Rifle: 1, Parka: 1, CorpseHuman: 1, CorpseAnimal: 1, GraniteChunk: 1, Kidney: 1, ServoArm: 1, PegLeg: 1,
		FragGrenades: 1, ShieldBelt: 1, ChargeRifle: 1, TShirt: 1}
	if StackLimit != 1000 {
		t.Fatalf("StackLimit = %d, want 1000", StackLimit)
	}
	for k, n := range want {
		if k.StackLimit() != n {
			t.Errorf("%s stacks to %d, want %d", k, k.StackLimit(), n)
		}
	}
	for k := Kind(0); k < numKinds; k++ {
		one := k.IsWeapon() || k.IsContractWeapon() || k.IsApparel() || k.IsBelt() || k.IsCorpse() || IsChunk(k) ||
			k.IsImplant() || k.IsOrgan()
		if got := k.StackLimit(); one && got != 1 || !one && got != StackLimit {
			t.Errorf("%s stacks to %d (per-unit identity or chunk: %v)", k, got, one)
		}
	}
	if Kind(255).StackLimit() != StackLimit {
		t.Fatal("an unknown kind keeps the plain limit")
	}
}
