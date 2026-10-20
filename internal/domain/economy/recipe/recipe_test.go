package recipe

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

func TestRecipesAndBills(t *testing.T) {
	t.Parallel()
	if len(For(construction.Stove)) != 1 || For(construction.Stove)[0].Output != item.Meal {
		t.Fatal("stove cooks meals")
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
	if k.Bench != construction.Workbench || k.Input != (Input{Kind: item.Stone, Qty: 3}) || k.Output != item.Knife || k.OutQty != 1 {
		t.Fatalf("knife recipe = %+v", k)
	}
	if s.Bench != construction.Workbench || s.Input != (Input{Kind: item.Wood, Qty: 5}) || s.Output != item.Spear || s.OutQty != 1 {
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
	if d.Input.Accepts(item.CorpseDustbuck) || d.OutQty != 0 || d.Extra != item.Wood || d.ExtraQty != 2 || d.Work != 300 {
		t.Fatalf("cremate burns one body with 2 wood, makes nothing: %+v", d)
	}
	if len(For(construction.Pyre)) != 1 {
		t.Fatal("a pyre only cremates")
	}
	for _, r := range For(construction.ButcherTable) {
		if r.Input.Accepts(item.CorpseHuman) {
			t.Fatal("human corpses are never butchered")
		}
	}
}
