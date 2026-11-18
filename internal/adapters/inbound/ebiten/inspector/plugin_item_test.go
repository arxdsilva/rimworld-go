// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package inspector

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/economy/item"
)

// A plugin item (PLG-026) goes by its label in the inspector's title and
// in a bill's ingredient list; core's names are unchanged.
func TestPluginItemLabels(t *testing.T) {
	t.Parallel()
	core := item.Core()
	defs := make([]item.Def, 0, item.CoreKinds+1)
	for k := range item.CoreKinds {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	items := item.NewBook(append(defs, item.Def{Label: "trail bell", Path: "Manufactured/Components"}),
		append(core.Keys(), "frontier.trail_bell"))
	bell := item.Kind(item.CoreKinds)
	if got := ItemTitleIn(game.ItemView{Kind: "frontier.trail_bell", Qty: 2}, items); got != "Trail bell ×2" {
		t.Fatalf("title %q", got)
	}
	if got := ItemTitleIn(game.ItemView{Kind: "wood", Qty: 3}, items); got != "Wood ×3" {
		t.Fatalf("core title %q", got)
	}
	bl := game.BillView{Ingredients: craft.NewBillIngredients(game.BillIngredientView{Kind: uint8(bell), Allowed: true},
		game.BillIngredientView{Kind: uint8(item.Steel)})}
	s := billSettingsIn(bl, items)
	if len(s.Ingredients) != 2 || s.Ingredients[0].Label != "trail bell" || !s.Ingredients[0].Allowed || s.Ingredients[1].Label != "steel" {
		t.Fatalf("ingredients %+v", s.Ingredients)
	}
}
