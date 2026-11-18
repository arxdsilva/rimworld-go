// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package inspector

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/skill"
)

// The bill picker offers the run's recipes, a plugin's included (PLG-025):
// it reads Env.Content, never the game's own list alone.
func TestBillPickerOffersPluginRecipes(t *testing.T) {
	f, a := benchFixture(t)
	core := recipe.Core()
	list, keys := core.All(), core.Keys()
	list = append(list, recipe.Def{Name: "demo.trail_stew", Label: "Cook trail stew", Bench: construction.Stove,
		Input: recipe.Input{AnyRaw: true, Qty: 8}, Output: item.Meal, OutQty: 1, Work: 200, Skill: work.Cook, Learn: skill.Cooking})
	keys = append(keys, "demo.trail_stew")
	f.env.Content = game.Content{Recipes: recipe.NewBook(list, keys)}

	has := func(rs []game.RecipeView) bool {
		for _, r := range rs {
			if r.Label == "Cook trail stew" {
				return true
			}
		}
		return false
	}
	if !has(a.RecipesFor("stove")) {
		t.Fatal("the stove's bill picker does not offer the plugin recipe")
	}
	f.env.Content = game.Content{}
	if has(a.RecipesFor("stove")) {
		t.Fatal("core content offers a plugin recipe")
	}
}
