package recipe

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

func TestOneButcherRecipeForEveryAnimal(t *testing.T) {
	t.Parallel()
	d := DefOf(ButcherAnimal)
	if d.Bench != construction.ButcherTable || !d.Input.Accepts(item.CorpseAnimal) || d.Input.Qty != 1 || !d.BySpecies ||
		d.Output != item.Meat || d.Extra != item.Leather || d.Name != "butcher_animal" {
		t.Fatalf("butcher animal: %+v", d)
	}
	var names []string
	for _, r := range For(construction.ButcherTable) {
		if r.Legacy {
			t.Fatalf("legacy recipe %s offered", r.Name)
		}
		names = append(names, r.Name)
	}
	// PRD-4: kibble is made at the butcher table too (wiki: Kibble);
	// FOOD-3: so is butchering a human
	if len(names) != 3 || names[0] != "butcher_animal" || names[1] != "make_kibble" || names[2] != "butcher_human" {
		t.Fatalf("a butcher table offers one butcher recipe and kibble: %v", names)
	}
	for _, id := range []ID{ButcherDustbuck, ButcherRidgewolf} {
		if !DefOf(id).Legacy || Upgrade(id) != ButcherAnimal {
			t.Fatalf("legacy recipe %d stays known and upgrades", id)
		}
	}
	if Upgrade(SimpleMeal) != SimpleMeal || Upgrade(ButcherAnimal) != ButcherAnimal {
		t.Fatal("other recipes are left alone")
	}
}
