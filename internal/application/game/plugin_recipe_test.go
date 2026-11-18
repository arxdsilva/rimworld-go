package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/command"
)

// A plugin recipe goes the real way (PLG-025): the AddBill command takes
// its ID past the core recipes, a crafter works it, and its product
// appears. Nothing on the path is bounded by the core recipe count.
func TestPluginRecipeIsMadeThroughAddBill(t *testing.T) {
	core := recipe.Core()
	list, keys := core.All(), core.Keys()
	list = append(list, recipe.Def{Name: "demo.quick_club", Label: "Whittle quick club", Bench: construction.Workbench,
		Input: recipe.Input{Kind: item.Wood, Qty: 3}, Output: item.Club, OutQty: 2, Work: 60, Skill: work.Craft, Learn: skill.Crafting})
	keys = append(keys, "demo.quick_club")
	book := recipe.NewBook(list, keys)
	id, ok := book.IDOf("demo.quick_club")
	if !ok || int(id) < core.Len() {
		t.Fatalf("plugin recipe id %d (ok %v), core has %d", id, ok, core.Len())
	}

	g := New(Config{Seed: 1, Width: 20, Height: 20, Content: Content{Recipes: book}})
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	setSkill(g, ada, skill.Crafting, 10)
	bench := buildNow(t, g, construction.Workbench, construction.Stuffs(construction.Workbench)[0], construction.South, 9, 8)
	g.SpawnStack(item.Wood, 10, 3, 5)
	b, _ := g.bldgs.Get(bench)
	g.Enqueue(command.Command{Kind: command.AddBill, X: b.X, Y: b.Y, Recipe: uint8(id), Mode: uint8(recipe.DoCount), Count: 1})
	for i := 0; i < 20000 && g.stockOf(item.Club) < 2; i++ {
		if i%200 == 0 {
			g.SetNeeds(ada, needs.Full, needs.Full)
		}
		g.Step(1)
	}
	if got := g.stockOf(item.Club); got < 2 {
		t.Fatalf("the plugin recipe made %d clubs, want 2", got)
	}
}
