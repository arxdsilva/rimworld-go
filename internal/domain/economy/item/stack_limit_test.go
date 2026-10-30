package item

import "testing"

// TestStackLimit (PRD-4, STO-2, wiki item pages): every kind carries its
// wiki stack limit; things the game never stacks are one per stack, and
// only the carpets (our items) keep StackLimit.
func TestStackLimit(t *testing.T) {
	t.Parallel()
	want := map[Kind]int{Chemfuel: 150, Silverdust: 150, Reagent: 150, Pemmican: 75, Plasteel: 75,
		AdvancedComponents: 50, Wort: 25, FineMeal: 10, SurvivalMeal: 10,
		Meal: 10, Wood: 75, Silver: 500, Gold: 500, Components: 50, Meat: 75, Milk: 75, Egg: 75, FertileEgg: 10,
		HerbalMedicine: 25, Beer: 25, GraniteBlocks: 75, MortarShells: 25, AshleafTea: 150, Steel: 75, Cloth: 75,
		Rifle: 1, Parka: 1, CorpseHuman: 1, CorpseAnimal: 1, Bones: 1, GraniteChunk: 1, Kidney: 1, ServoArm: 1, PegLeg: 1,
		FragGrenades: 1, ShieldBelt: 1, ChargeRifle: 1, CarpetWool: StackLimit}
	for k, n := range want {
		if k.StackLimit() != n {
			t.Errorf("%s stacks to %d, want %d", k, k.StackLimit(), n)
		}
	}
	for k := Kind(0); k < numKinds; k++ {
		if k.StackLimit() == StackLimit && !k.IsCarpet() {
			t.Errorf("%s has no wiki stack limit", k)
		}
	}
	if Kind(255).StackLimit() != StackLimit {
		t.Fatal("an unknown kind keeps the plain limit")
	}
}
