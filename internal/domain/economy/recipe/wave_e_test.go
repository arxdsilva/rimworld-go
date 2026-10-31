package recipe

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

// TestWaveECatalogue (PRD-4): every new recipe takes only items that
// exist, makes a known product at the wiki's bench, and every armour
// piece, meal tier and drug of the catalogue has a recipe.
func TestWaveECatalogue(t *testing.T) {
	t.Parallel()
	made := map[item.Kind]construction.Kind{}
	for id := CookFineMeal; id < numRecipes; id++ {
		d := DefOf(id)
		if d.ID != id || d.Name == "" || d.Label == "" || d.Work <= 0 || d.Bench == 0 {
			t.Errorf("recipe %d: %+v", id, d)
		}
		for _, in := range d.AppendInputs(nil) {
			if in.Qty <= 0 || in.Kind.String() == "unknown" {
				t.Errorf("%s: input %+v", d.Name, in)
			}
			if !in.AnyVeg && !in.AnyMeat && !in.AnyRaw && !in.SmeltWear && len(d.InputKinds()) == 0 {
				t.Errorf("%s: no item feeds it", d.Name)
			}
		}
		if !d.Ferment && d.OutQty <= 0 && !d.Input.SmeltWear && !d.Salvage && !d.BySpecies {
			t.Errorf("%s makes nothing", d.Name)
		}
		if !d.Salvage {
			made[d.Output] = d.Bench
		}
	}
	want := map[item.Kind]construction.Kind{
		item.FineMeal: construction.Stove, item.LavishMeal: construction.Stove, item.Pemmican: construction.Stove,
		item.SurvivalMeal: construction.Stove, item.Kibble: construction.ButcherSpot, item.Wort: construction.BrewingBench,
		item.Beer: construction.FermentingBarrel, item.AshFlake: construction.DrugTable, item.Silverdust: construction.DrugTable,
		item.BattleTonic: construction.DrugTable, item.Reagent: construction.DrugTable, item.Chemfuel: construction.BiofuelRefinery,
		item.AdvancedComponents: construction.FabricationBench, item.Steel: construction.ElectricSmelter,
		item.FlakVest: construction.MachiningTable, item.FlakPants: construction.MachiningTable, item.FlakJacket: construction.MachiningTable,
		item.PlateArmor: construction.FueledSmithy, item.SimpleHelmet: construction.FueledSmithy,
		item.AdvancedHelmet: construction.MachiningTable, item.MarineArmor: construction.FabricationBench,
		item.MarineHelmet: construction.FabricationBench, item.FragGrenades: construction.MachiningTable,
		item.MolotovCocktails: construction.MachiningTable, item.MortarShells: construction.MachiningTable,
		item.IncendiaryShells: construction.MachiningTable, item.EMPShells: construction.MachiningTable,
		item.SmokeShells: construction.MachiningTable,
	}
	for k, bench := range want {
		if made[k] != bench {
			t.Errorf("%s made at %s, want %s", k, made[k], bench)
		}
	}
	for _, k := range item.Apparel() {
		if !k.IsArmor() {
			continue
		}
		found := false
		for _, d := range All() {
			found = found || d.Output == k
		}
		if !found {
			t.Errorf("armor %s has no recipe", k)
		}
	}
}

// TestMealInputs: the veg side takes plant food, the meat side meat and
// animal products; kibble yields 50 at the table and 35 at the spot.
func TestMealInputs(t *testing.T) {
	t.Parallel()
	fine := DefOf(CookFineMeal)
	veg, meat := fine.Inputs[0], fine.Inputs[1]
	if !veg.Accepts(item.Corn) || veg.Accepts(item.Meat) || veg.Accepts(item.Egg) || !meat.Accepts(item.Meat) ||
		!meat.Accepts(item.Milk) || meat.Accepts(item.Corn) || veg.Accepts(item.Meal) {
		t.Fatal("fine meal sides")
	}
	if veg.Label() != "5 plant food" || meat.Label() != "5 meat or animal product" || fine.MinSkill != 6 || fine.Work != 180 {
		t.Fatalf("fine meal: %s, %s, skill %d, work %d", veg.Label(), meat.Label(), fine.MinSkill, fine.Work)
	}
	if DefOf(MakeKibble).OutQty != 50 || DefOf(MakeKibbleSpot).OutQty != 35 || !DefOf(MakeKibbleSpot).At(construction.ButcherSpot) {
		t.Fatal("kibble yields")
	}
	if !Cooked(item.FineMeal) || !Cooked(item.Meal) || Cooked(item.Pemmican) || Cooked(item.Kibble) {
		t.Fatal("cooked meals carry the cook's poison roll")
	}
	fill := DefOf(FillBarrel)
	if !fill.Ferment || fill.Input.Kind != item.Wort || fill.Input.Take(40) != BarrelWort || fill.Output != item.Beer {
		t.Fatalf("fill barrel %+v", fill)
	}
}

// TestSmeltApparelYield: metal armor smelts to a quarter of its steel;
// cloth, wood and stone-free pieces give nothing, and the weapon bill
// takes no apparel.
func TestSmeltApparelYield(t *testing.T) {
	t.Parallel()
	in := DefOf(SmeltApparel).Input
	vest := item.Stack{Kind: item.FlakVest, Qty: 1}
	if !in.AcceptsStack(vest) || SmeltYield(vest) != 1 || in.Label() != "1 metal apparel" {
		t.Fatalf("flak vest smelts to %d (%s)", SmeltYield(vest), in.Label())
	}
	wooden := item.Stack{Kind: item.PlateArmor, Qty: 1, Stuff: item.Wood}
	if in.AcceptsStack(wooden) || in.AcceptsStack(item.Stack{Kind: item.Parka, Qty: 1}) {
		t.Fatal("no steel, nothing to smelt")
	}
	if DefOf(SmeltWeapon).Input.Accepts(item.FlakVest) || in.Accepts(item.Mace) {
		t.Fatal("weapons and apparel keep their own bills")
	}
}

// TestShredMechWreck (DEF-4 wrecks, wiki: Machining table): a wreck is
// shredded at the machining table for its type's salvage, scaled / 10;
// butchering never takes one, nor shredding an animal carcass.
func TestShredMechWreck(t *testing.T) {
	t.Parallel()
	shred, butcher := DefOf(ShredMech), DefOf(ButcherAnimal)
	wreck := item.Stack{Kind: item.CorpseAnimal, Qty: 1, Animal: uint8(animal.Bulwark)}
	deer := item.Stack{Kind: item.CorpseAnimal, Qty: 1, Animal: uint8(animal.Deer)}
	if !shred.Input.AcceptsStack(wreck) || shred.Input.AcceptsStack(deer) || butcher.Input.AcceptsStack(wreck) ||
		!butcher.Input.AcceptsStack(deer) || shred.Bench != construction.MachiningTable || shred.Input.Label() != "1 mechanoid wreck" {
		t.Fatal("wrecks are shredded, carcasses butchered")
	}
	if st, pl := SalvageOf(uint8(animal.Bulwark)); st != 3 || pl != 1 {
		t.Fatalf("bulwark salvage %d steel %d plasteel, want 3 and 1 (wiki 30 and 10, / 10)", st, pl)
	}
	if st, pl := SalvageOf(uint8(animal.Harrow)); st != 2 || pl != 0 {
		t.Fatalf("harrow salvage %d %d", st, pl)
	}
	if st, pl := SalvageOf(uint8(animal.Deer)); st != 0 || pl != 0 {
		t.Fatal("an animal salvages nothing")
	}
}
