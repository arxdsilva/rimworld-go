package recipe

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

func TestRecipesAndBills(t *testing.T) {
	t.Parallel()
	// the simple meal first, then PRD-4's fine, lavish, pemmican and
	// survival meals
	var outs []item.Kind
	for _, d := range For(construction.Stove) {
		outs = append(outs, d.Output)
	}
	if want := []item.Kind{item.Meal, item.FineMeal, item.LavishMeal, item.Pemmican, item.SurvivalMeal}; !slices.Equal(outs, want) {
		t.Fatalf("stove cooks %v, want %v", outs, want)
	}
	meal := DefOf(SimpleMeal)
	if !meal.Input.Accepts(item.Berries) || !meal.Input.Accepts(item.Meat) || meal.Input.Accepts(item.Meal) {
		t.Fatal("meals take any raw food")
	}
	// cooking multiplies nutrition
	if meal.OutQty*item.Meal.Nutrition() <= meal.Input.Qty*item.Potatoes.Nutrition() {
		t.Fatal("a meal must be worth more than its raw inputs")
	}
	if !(Bill{Mode: DoCount, Target: 2, Done: 1}).Wants(0) || (Bill{Mode: DoCount, Target: 2, Done: 2}).Wants(0) {
		t.Fatal("do-count")
	}
	if !(Bill{Mode: UntilHave, Target: 5}).Wants(4) || (Bill{Mode: UntilHave, Target: 5}).Wants(5) {
		t.Fatal("until-have")
	}
	if (Bill{Mode: Forever, Suspended: true}).Wants(0) {
		t.Fatal("suspended bills never run")
	}
}

func TestKnifeSpearRecipes(t *testing.T) {
	t.Parallel()
	k, s := DefOf(Knife), DefOf(Spear)
	if k.Bench != construction.Workbench || k.Input != (Input{Kind: item.GraniteBlocks, Qty: 3, Stuff: item.MadeFrom(item.Knife)}) || k.Output != item.Knife || k.OutQty != 1 {
		t.Fatalf("knife recipe = %+v", k)
	}
	if s.Bench != construction.Workbench || s.Input != (Input{Kind: item.Wood, Qty: 5, Stuff: item.MadeFrom(item.Spear)}) || s.Output != item.Spear || s.OutQty != 1 {
		t.Fatalf("spear recipe = %+v", s)
	}
	if k.Work <= 0 || s.Work <= k.Work {
		t.Fatalf("a spear takes longer than a knife: %d vs %d", s.Work, k.Work)
	}
	seen := map[item.Kind]bool{}
	for _, d := range For(construction.Workbench) {
		seen[d.Output] = true
	}
	if !seen[item.Knife] || !seen[item.Spear] {
		t.Fatal("the workbench offers knife and spear")
	}
}

func TestCremateBurnsAHumanCorpseOnAPyre(t *testing.T) {
	t.Parallel()
	d := DefOf(Cremate)
	if d.Bench != construction.Pyre || !d.Input.Accepts(item.CorpseHuman) || d.Input.Qty != 1 {
		t.Fatalf("cremate: %+v", d)
	}
	if d.Input.Accepts(item.CorpseAnimal) || d.OutQty != 0 || d.Extra != item.Wood || d.ExtraQty != 2 || d.Work != 300 {
		t.Fatalf("cremate burns one body with 2 wood, makes nothing: %+v", d)
	}
	if len(For(construction.Pyre)) != 1 {
		t.Fatal("a pyre only cremates")
	}
	for _, r := range For(construction.ButcherTable) {
		if r.Input.Accepts(item.CorpseHuman) != (r.ID == ButcherHuman) {
			t.Fatal("human corpses are butchered only by Butcher human (FOOD-3)")
		}
	}
}

func TestUntilHaveWaitsThenResumesAt(t *testing.T) {
	t.Parallel()
	b := Bill{Recipe: CampfireMeal, Mode: UntilHave, Target: 10}
	if b.ResumeAt() != 8 {
		t.Fatalf("resume at %d, want 8", b.ResumeAt())
	}
	b = b.Update(10)
	if !b.Waiting || b.Wants(10) {
		t.Fatal("at target the bill waits")
	}
	b = b.Update(9)
	if !b.Waiting || b.Wants(9) {
		t.Fatal("9 is still waiting")
	}
	b = b.Update(8)
	if b.Waiting || !b.Wants(8) {
		t.Fatal("8 resumes")
	}
	one := Bill{Mode: UntilHave, Target: 1}.Update(1)
	if !one.Waiting || one.ResumeAt() != 0 {
		t.Fatalf("target 1: %+v", one)
	}
	if one = one.Update(0); one.Waiting || !one.Wants(0) {
		t.Fatal("target 1 resumes at 0")
	}
	if (Bill{Mode: Forever, Waiting: true}).Update(0).Waiting {
		t.Fatal("other modes never wait")
	}
}
