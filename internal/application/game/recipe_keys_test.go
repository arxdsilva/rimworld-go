package game

import (
	"bytes"
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// recipeBookWith is core's recipes plus one workbench recipe per plugin
// key (4 wood into a wood slat carpet), as modding.Build freezes them.
func recipeBookWith(keys ...string) recipe.Book {
	core := recipe.Core()
	list, all := append([]recipe.Def(nil), core.All()...), core.Keys()
	for _, k := range keys {
		list = append(list, recipe.Def{Name: recipe.NameOf(k), Label: "Plugin " + k, Bench: construction.Workbench,
			Input: recipe.Input{Kind: item.Wood, Qty: 4}, Output: item.CarpetWood, OutQty: 1, Work: 100,
			Skill: core.DefOf(recipe.Club).Skill, Learn: core.DefOf(recipe.Club).Learn})
		all = append(all, k)
	}
	return recipe.NewBook(list, all)
}

// billKeys is bench bid's bills as recipe keys of book.
func billKeys(g *Game, book recipe.Book, bid entity.EntityID) []string {
	var out []string
	for _, bl := range g.craft.Bills.Of(bid) {
		out = append(out, book.Key(bl.Recipe))
	}
	return out
}

// A plugin's recipe is offered at its bench, shows on the bench's bill
// list and is made there (PLG-025).
func TestPluginRecipeShownAndMade(t *testing.T) {
	t.Parallel()
	book := recipeBookWith("frontier.slats")
	slats, _ := book.IDOf("frontier.slats")
	g := New(Config{Seed: 1, Width: 14, Height: 8, Content: Content{Recipes: book}})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	placeBuilt(t, g, construction.Workbench, construction.South, 5, 2)
	g.SpawnStack(item.Wood, 4, 1, 4)
	offered := slices.ContainsFunc(RecipesOf(Content{Recipes: book}), func(r RecipeView) bool {
		return r.ID == uint8(slats) && r.Label == "Plugin frontier.slats" && r.RunsAt("workbench")
	})
	if !offered || slices.ContainsFunc(Recipes(), func(r RecipeView) bool { return r.ID == uint8(slats) }) {
		t.Fatalf("the run's recipes offer the plugin recipe (%v); core's do not", offered)
	}
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(slats), Mode: uint8(recipe.DoCount), Count: 1})
	g.Step(1)
	bid := g.layerBldgAt(5, 2, construction.LayerObject)
	if bv := buildingView(g, bid); len(bv.Bills) != 1 || bv.Bills[0].Label != "Plugin frontier.slats" || bv.Bills[0].Recipe != "frontier.slats" {
		t.Fatalf("bench bills %+v", bv.Bills)
	}
	for i := 0; i < 400 && g.stockOf(item.CarpetWood) == 0; i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(TicksPerCell * 10)
	}
	if g.stockOf(item.CarpetWood) != 1 || g.stockOf(item.Wood) != 0 {
		t.Fatalf("carpets %d, wood left %d", g.stockOf(item.CarpetWood), g.stockOf(item.Wood))
	}
}

// A save keeps its plugin recipes by key (PLG-044): bills, and an AddBill
// still pending, follow their recipe where the plugin set moved it, and a
// bill whose plugin is gone is dropped, never pointed at another recipe.
func TestSavedRecipesFollowTheirKeys(t *testing.T) {
	t.Parallel()
	saved := recipeBookWith("a.first", "b.second")
	g := New(Config{Width: 12, Height: 12, Content: Content{Recipes: saved}})
	placeBuilt(t, g, construction.Workbench, construction.South, 5, 2)
	bid := g.layerBldgAt(5, 2, construction.LayerObject)
	first, _ := saved.IDOf("a.first")
	second, _ := saved.IDOf("b.second")
	for _, r := range []recipe.ID{first, recipe.MakeStrawCarpet, second} {
		g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(r), Mode: uint8(recipe.Forever), Count: 1})
	}
	g.Step(1)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(first), Mode: uint8(recipe.Forever), Count: 1})
	var buf bytes.Buffer
	if err := g.Save(&buf, "recipes"); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()

	for _, tc := range []struct {
		name string
		book recipe.Book
		want []string // the bench's bills, after the pending AddBill
	}{
		{"same set", saved, []string{"a.first", "core.make_straw_carpet", "b.second", "a.first"}},
		{"first plugin gone", recipeBookWith("b.second"), []string{"core.make_straw_carpet", "b.second"}},
		{"reordered", recipeBookWith("b.second", "a.first"), []string{"a.first", "core.make_straw_carpet", "b.second", "a.first"}},
		{"core only", recipe.Core(), []string{"core.make_straw_carpet"}},
	} {
		l, _, err := LoadContent(bytes.NewReader(data), Content{Recipes: tc.book})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		l.Step(1)
		if got := billKeys(l, tc.book, bid); !slices.Equal(got, tc.want) {
			t.Errorf("%s: bills %v, want %v", tc.name, got, tc.want)
		}
	}
}
