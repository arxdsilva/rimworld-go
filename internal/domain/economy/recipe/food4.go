package recipe

import (
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/skill"
)

// FOOD-4 cooking (ENG-003, wiki: Meals, Simple meal, Fine meal, Lavish
// meal, Packaged survival meal).
//
//   - Food inputs (AnyRaw, AnyVeg, AnyMeat) count nutrition, not items:
//     Qty is in units of NutritionUnit (0.05, one raw item), and a stack
//     gives animal.StackNutrition per item, so a chicken egg (0.25) is
//     five units and a simple meal (0.5) takes two eggs or ten potatoes.
//   - The vegetarian and carnivore fine meal take 0.75 of one side, the
//     lavish 1.25 (wiki: Meals); the product is the same meal kind, its
//     ingredients telling them apart.
//   - Batch bills cook BatchSize meals from BatchSize times the inputs
//     (wiki: Simple meal "x4", 2 nutrition, quadruple the work: 1200;
//     Packaged survival meal bulk 1440; Pemmican 64 from 1 + 1 in 2800). The fine bulk takes 1440 as the
//     survival meal, whose single work (450) it shares; the lavish bulk
//     2560 is the same 3.2 ratio on its 800 (original tuning: the wiki
//     prints no lavish or fine bulk work).

// NutritionUnit is one food input unit in needs units (needs.Full =
// 10000 is 1.0 nutrition): 0.05, a raw food item.
const NutritionUnit = 500

// BatchSize is how many meals a batch bill cooks.
const BatchSize = 4

// The batch bills' wiki work (ticks; scaled by cookWorkPct).
const (
	bulkSimpleWork   = 1200 // wiki: Simple meal
	bulkSurvivalWork = 1440 // wiki: Packaged survival meal
	bulkFineWork     = 1440 // as the survival meal (original tuning)
	bulkLavishWork   = 2560 // 3.2 x 800 (original tuning)
)

// ByNutrition reports a food input, measured in nutrition units.
func (in Input) ByNutrition() bool { return in.AnyRaw || in.AnyVeg || in.AnyMeat }

// Weight is how many of in's units one item of st gives: its nutrition
// in NutritionUnit (at least one) for a food input, else one.
func (in Input) Weight(st item.Stack) int {
	if !in.ByNutrition() {
		return 1
	}
	return max(1, animal.StackNutrition(st)/NutritionUnit)
}

// Items is how many items of st cover units of in (rounded up).
func (in Input) Items(st item.Stack, units int) int {
	w := in.Weight(st)
	return (units + w - 1) / w
}

// TakeFrom is how many items in takes from stack st alone: up to Max of
// it, the items covering its nutrition, or Qty.
func (in Input) TakeFrom(st item.Stack) int {
	switch {
	case in.Max > 0:
		return min(st.Qty, in.Max)
	case in.ByNutrition():
		return in.Items(st, in.Qty)
	}
	return in.Qty
}

// Enough reports stack st (qty items of it) feeds in on its own.
func (in Input) Enough(st item.Stack, qty int) bool { return qty*in.Weight(st) >= in.Qty }

// oneSided is a meal at the stove from one food input (the vegetarian
// and carnivore variants).
func oneSided(id ID, name, label string, out item.Kind, src Input, wikiWork int, minSkill uint8) Def {
	return Def{ID: id, Name: name, Label: label, Bench: construction.Stove, Input: src,
		Output: out, OutQty: 1, Work: wikiWork * cookWorkPct / 100, Skill: work.Cook, Learn: skill.Cooking, MinSkill: minSkill}
}

// bulk is BatchSize meals of out from BatchSize x (each + each) units.
func bulk(id ID, name, label string, out item.Kind, each, wikiWork int, minSkill uint8) Def {
	return meal(id, name, label, out, BatchSize*each, BatchSize*each, BatchSize, wikiWork, minSkill)
}

// bulkSimple is BatchSize simple meals from 2 nutrition of raw food.
func bulkSimple() Def {
	d := oneSided(CookSimpleMealBulk, "cook_simple_meal_bulk", "Cook 4 simple meals", item.Meal,
		Input{AnyRaw: true, Qty: BatchSize * 10}, bulkSimpleWork, 0)
	d.OutQty = BatchSize
	return d
}
