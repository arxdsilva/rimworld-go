package recipe

import (
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/skill"
)

// PRD-4 catalogue (ENG-003: wiki recipe pages, our scaling).
//
//   - Cooking (wiki: Fine meal, Lavish meal, Pemmican, Packaged survival
//     meal, Kibble): the wiki's nutrition in raw units (the simple meal's
//     0.5 nutrition is 2 raw units here, so 0.25 is 1), its work times
//     0.4 (the simple meal's 300 ticks are 120 here), its minimum skill.
//     Animal products (milk, eggs) count on the meat side.
//   - Drugs (wiki: Drug lab, Brewery): the wiki's leaf counts and work,
//     as the ENG-003 drug rows; ash flake, silverdust and battle tonic
//     are our names for flake, yayo and go-juice. Reagent stands in for
//     neutroamine, which the wiki never crafts: distilling it from
//     chemfuel and ashleaf is original tuning, so battle tonic can be
//     made from the colony's own goods.
//   - Benches (wiki: Biofuel refinery, Advanced component, the armour,
//     grenade and shell pages): material / 10 rounded up, components as
//     the wiki, work / 10, as PRD-3. Food for chemfuel is the wiki's 3.5
//     nutrition in raw units (14). The advanced helmet takes the flak
//     helmet's recipe (the wiki has no core advanced helmet).
//   - The fermenting barrel (wiki: Fermenting barrel): it takes wort up
//     to 25 and ferments it into as much beer (craft.Fermenting).

// in is one plain ingredient.
func in(k item.Kind, qty int) Input { return Input{Kind: k, Qty: qty} }

// cookWorkPct is the share of the wiki's work a cooking recipe takes
// (the simple meal: 120 of 300).
const cookWorkPct = 40

// meal is a cooked food at the stove from veg plant food and meat meat or
// animal products (raw units), out of it, the wiki's work and minimum
// Cooking.
func meal(id ID, name, label string, out item.Kind, veg, meat, qty, wikiWork int, minSkill uint8) Def {
	ins := []Input{{AnyVeg: true, Qty: veg}, {AnyMeat: true, Qty: meat}}
	return Def{ID: id, Name: name, Label: label, Bench: construction.Stove, Input: ins[0], Inputs: ins,
		Output: out, OutQty: qty, Work: wikiWork * cookWorkPct / 100, Skill: work.Cook, Learn: skill.Cooking, MinSkill: minSkill}
}

// kibble is animal feed (wiki: Kibble: 1 + 1 nutrition, 4 + 4 raw units
// here; 50 at the butcher table, 35 at the spot; 450 work).
func kibble(id ID, name, label string, bench construction.Kind, qty int) Def {
	d := meal(id, name, label, item.Kibble, 4, 4, qty, 450, 0)
	d.Bench = bench
	return d
}

// brewWort is wort at the brewing bench (wiki: Brewery, 25 hops make 5
// wort in 1000 ticks; brewgrain is our hops).
func brewWort() Def {
	return Def{ID: BrewWort, Name: "brew_wort", Label: "Brew wort", Bench: construction.BrewingBench,
		Input: in(item.Brewgrain, 25), Output: item.Wort, OutQty: 5, Work: 1000 * cookWorkPct / 100,
		Skill: work.Cook, Learn: skill.Cooking}
}

// BarrelWort is how much wort one fermenting barrel holds (wiki:
// Fermenting barrel).
const BarrelWort = 25

// fillWork is the work of pouring a load into a barrel (the wiki gives
// none: original tuning).
const fillWork = 30

// fillBarrel pours wort into a fermenting barrel, up to BarrelWort; the
// barrel ferments it into beer (Output, so "until you have" counts beer).
func fillBarrel() Def {
	return Def{ID: FillBarrel, Name: "fill_barrel", Label: "Fill with wort", Bench: construction.FermentingBarrel,
		Input: Input{Kind: item.Wort, Qty: 1, Max: BarrelWort}, Output: item.Beer, Work: fillWork,
		Skill: work.Cook, Learn: skill.Cooking, Ferment: true}
}

// drug is one dose made at the drug table from ins (wiki: Drug lab).
func drug(id ID, name, label string, out item.Kind, w int, minSkill uint8, ins ...Input) Def {
	return Def{ID: id, Name: name, Label: label, Bench: construction.DrugTable, Input: ins[0], Inputs: multi(ins),
		Output: out, OutQty: 1, Work: w, Skill: work.Craft, Learn: skill.Crafting, MinSkill: minSkill}
}

// refine is 35 chemfuel at the fuel refinery (wiki: Biofuel refinery).
func refine(id ID, name, label string, src Input, w int) Def {
	return Def{ID: id, Name: name, Label: label, Bench: construction.BiofuelRefinery, Input: src,
		Output: item.Chemfuel, OutQty: 35, Work: w, Skill: work.Craft, Learn: skill.Crafting}
}

// made is one out at bench from ins.
func made(id ID, name, label string, bench construction.Kind, out item.Kind, w int, minSkill uint8, ins ...Input) Def {
	return Def{ID: id, Name: name, Label: label, Bench: bench, Input: ins[0], Inputs: multi(ins),
		Output: out, OutQty: 1, Work: w, Skill: work.Craft, Learn: skill.Crafting, MinSkill: minSkill}
}

// multi is ins as Def.Inputs: nil for a single ingredient (Input alone).
func multi(ins []Input) []Input {
	if len(ins) < 2 {
		return nil
	}
	return ins
}

// smeltWear smelts metal apparel back to steel (wiki: Electric smelter,
// 1600 ticks, a quarter back).
func smeltWear() Def {
	return Def{ID: SmeltApparel, Name: "smelt_apparel", Label: "Smelt metal apparel", Bench: construction.ElectricSmelter,
		Input: Input{SmeltWear: true, Qty: 1}, Output: item.Steel, Work: 160, Skill: work.Craft, Learn: skill.Crafting}
}

// plantFood is raw plant food (the fine meal's vegetable side).
func plantFood(k item.Kind) bool { return k.Raw() && !animalFood(k) }

// animalFood is raw meat (FOOD-0: human and insect meat too) or an
// animal product (the meat side).
func animalFood(k item.Kind) bool { return k.IsMeat() || k == item.Milk || k == item.Egg }

// Cooked reports a meal a cook makes, which carries the cook's poison
// chance (the simple meal and PRD-4's better meals).
func Cooked(k item.Kind) bool {
	return k == item.Meal || k == item.FineMeal || k == item.LavishMeal || k == item.SurvivalMeal
}

// shredMech shreds one mechanoid wreck at the machining table (wiki:
// Machining table, "Shred mechanoid": 300 ticks, the yield by mechanoid
// type; work / 10).
func shredMech() Def {
	return Def{ID: ShredMech, Name: "shred_mech", Label: "Shred mechanoid", Bench: construction.MachiningTable,
		Input: Input{Kind: item.CorpseAnimal, Qty: 1, Wreck: true}, Output: item.Steel, Extra: item.Plasteel,
		Work: 30, Skill: work.Craft, Learn: skill.Crafting, Salvage: true}
}

// SalvageOf is the steel and plasteel shredding one wreck of mechanoid
// species s gives: animal.WreckYield (the wiki's per-type yield) scaled
// / 10, rounded up, as every material here (PRD-3).
func SalvageOf(s uint8) (steel, plasteel int) {
	y := animal.WreckYield(animal.Species(s))
	return (y.Steel + 9) / 10, (y.Plasteel + 9) / 10
}
