// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// pluginItems is core's item kinds plus one plugin good per key (a bell
// filed under components, worth 40), as modding.Build freezes them.
func pluginItems(keys ...string) item.Book {
	core := item.Core()
	defs, all := make([]item.Def, 0, item.CoreKinds+len(keys)), core.Keys()
	for k := range item.CoreKinds {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	for _, k := range keys {
		defs = append(defs, item.Def{Label: "bell " + k, Path: "Manufactured/Components", Mass: 1000, Value: 40,
			Stack: item.StackLimit, Texture: "textures/bell.png"})
		all = append(all, k)
	}
	return item.NewBook(defs, all)
}

// castBell is core's recipes plus a workbench recipe casting 2 of plugin
// kind bell from 8 steel and one hanging a bell on a wood post (the bell
// as an input), smelting with items.
func castBell(items item.Book, bell item.Kind) recipe.Book {
	core := recipe.Core()
	list, keys := append([]recipe.Def(nil), core.All()...), core.Keys()
	list = append(list, recipe.Def{Name: "cast_bell", Label: "Cast bells", Bench: construction.Workbench,
		Input: recipe.Input{Kind: item.Steel, Qty: 8}, Output: bell, OutQty: 2, Work: 100,
		Skill: core.DefOf(recipe.Club).Skill, Learn: core.DefOf(recipe.Club).Learn},
		recipe.Def{Name: "hang_bell", Label: "Hang a bell", Bench: construction.Workbench,
			Inputs: []recipe.Input{{Kind: bell, Qty: 1}, {Kind: item.Wood, Qty: 2}}, Input: recipe.Input{Kind: bell, Qty: 1},
			Output: item.CarpetWood, OutQty: 1, Work: 100, Skill: core.DefOf(recipe.Club).Skill, Learn: core.DefOf(recipe.Club).Learn})
	return recipe.NewBook(list, append(keys, "frontier.cast_bell", "frontier.hang_bell")).WithItems(items)
}

// noUnknown fails when a pawn, building or recipe view names an item
// "unknown": a plugin item goes by its key or label everywhere.
func noUnknown(t *testing.T, g *Game, c Content) {
	t.Helper()
	views := []any{g.Snapshot(), g.World().Buildings, RecipesOf(c)}
	for _, v := range views {
		if s := fmt.Sprintf("%+v", v); strings.Contains(s, "unknown") {
			i := strings.Index(s, "unknown")
			t.Fatalf("a view says unknown: …%s…", s[max(0, i-80):min(len(s), i+20)])
		}
	}
}

// stacksOf is every stack of kind k.
func stacksOf(g *Game, k item.Kind) []item.Stack {
	var out []item.Stack
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == k {
			out = append(out, st)
		}
	}
	return out
}

// A plugin recipe makes a plugin good, which is hauled to a stockpile,
// shown by its key and label, and smelted back to steel at an electric
// smelter by its recipe's steel (PLG-026).
func TestPluginItemMadeStoredAndSmelted(t *testing.T) {
	t.Parallel()
	items := pluginItems("frontier.bell")
	bell := item.Kind(item.CoreKinds)
	rb := castBell(items, bell)
	cast, _ := rb.IDOf("frontier.cast_bell")
	g := New(Config{Seed: 1, Width: 22, Height: 14, Content: Content{Recipes: rb, Items: items}})
	pawn, _ := g.SpawnNamedPawn(1, 1, "Ada")
	placeBuilt(t, g, construction.Workbench, construction.South, 5, 2)
	g.SpawnStack(item.Steel, 8, 1, 4)
	applyNow(g, command.Command{Kind: command.DesignateStockpile, X: 14, Y: 8, X2: 16, Y2: 10})
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(cast), Mode: uint8(recipe.DoCount), Count: 1})
	stored := func() int {
		n := 0
		for _, st := range stacksOf(g, bell) {
			if _, ok := g.storageAt(st.X, st.Y); ok && st.Carrier == 0 {
				n += st.Qty
			}
		}
		return n
	}
	for i := 0; i < 600 && stored() < 2; i++ {
		g.SetNeeds(pawn, needs.Full, needs.Full)
		g.Step(TicksPerCell * 3)
		noUnknown(t, g, Content{Recipes: rb, Items: items})
	}
	if stored() != 2 || g.stockOf(item.Steel) != 0 {
		t.Fatalf("bells stored %d (all %v), steel left %d", stored(), stacksOf(g, bell), g.stockOf(item.Steel))
	}
	w := g.World()
	if !slices.ContainsFunc(w.Items, func(v ItemView) bool { return v.Kind == "frontier.bell" && v.Qty == 2 }) {
		t.Fatal("the item view does not name the bell by its key")
	}
	if !slices.ContainsFunc(w.Stored, func(r StoredResourceView) bool {
		return r.Kind == "frontier.bell" && r.Label == "bell frontier.bell" && r.Category == "Manufactured" && r.Qty == 2
	}) {
		t.Fatalf("stored rows %+v", w.Stored)
	}

	sm := g.placeGenerated(construction.Building{Kind: construction.ElectricSmelter, Stuff: item.Steel, X: 8, Y: 6})
	if sm == 0 {
		t.Fatal("no smelter")
	}
	onPower(t, g, 8, 6, 8, 10)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 8, Y: 6, Recipe: uint8(recipe.SmeltWeapon), Mode: uint8(recipe.Forever), Count: 1})
	for i := 0; i < 900 && len(stacksOf(g, bell)) > 0; i++ {
		g.SetNeeds(pawn, needs.Full, needs.Full)
		g.Step(TicksPerCell * 3)
		noUnknown(t, g, Content{Recipes: rb, Items: items})
	}
	if left := stacksOf(g, bell); len(left) > 0 || g.stockOf(item.Steel) != 2 {
		t.Fatalf("bells left %v; steel back %d, want 1 a bell", left, g.stockOf(item.Steel))
	}
	_ = entity.EntityID(sm)
}

// A plugin good is a recipe input: a bench gathers it with the core
// ingredient and makes the product (PLG-026).
func TestPluginItemAsRecipeInput(t *testing.T) {
	t.Parallel()
	items := pluginItems("frontier.bell")
	bell := item.Kind(item.CoreKinds)
	rb := castBell(items, bell)
	hang, _ := rb.IDOf("frontier.hang_bell")
	c := Content{Recipes: rb, Items: items}
	g := New(Config{Seed: 1, Width: 16, Height: 10, Content: c})
	pawn, _ := g.SpawnNamedPawn(1, 1, "Ada")
	placeBuilt(t, g, construction.Workbench, construction.South, 5, 2)
	g.SpawnStack(bell, 1, 1, 5)
	g.SpawnStack(item.Wood, 2, 2, 6)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(hang), Mode: uint8(recipe.DoCount), Count: 1})
	for i := 0; i < 400 && g.stockOf(item.CarpetWood) == 0; i++ {
		g.SetNeeds(pawn, needs.Full, needs.Full)
		g.Step(TicksPerCell * 3)
		noUnknown(t, g, c)
	}
	if g.stockOf(item.CarpetWood) != 1 || len(stacksOf(g, bell)) != 0 {
		t.Fatalf("carpets %d, bells left %v", g.stockOf(item.CarpetWood), stacksOf(g, bell))
	}
	bid := g.layerBldgAt(5, 2, construction.LayerObject)
	if r := RecipesOf(c); !slices.ContainsFunc(r, func(v RecipeView) bool {
		return v.Name == "hang_bell" && v.Input == "1 bell frontier.bell + 2 wood" && slices.Contains(v.InputKinds, "bell frontier.bell")
	}) {
		t.Fatalf("recipe views %+v", r[len(r)-1])
	}
	_ = bid
}

// A save keeps its plugin items by key (PLG-026): stacks, a carried one
// and a stockpile's filter follow their kind where the plugin set moved
// it, and a gone plugin's stacks are removed (the game runs on).
func TestSavedPluginItemsFollowTheirKeys(t *testing.T) {
	t.Parallel()
	saved := pluginItems("a.cup", "b.bell")
	cup, bell := item.Kind(item.CoreKinds), item.Kind(item.CoreKinds+1)
	g := New(Config{Seed: 1, Width: 16, Height: 12, Content: Content{Items: saved}})
	pawn, _ := g.SpawnNamedPawn(1, 1, "Ada")
	g.SpawnStack(cup, 3, 4, 4)
	g.SpawnStack(bell, 5, 5, 5)
	carried, _ := g.SpawnStack(cup, 1, 1, 1)
	st, _ := g.stacks.Get(carried)
	st.Carrier = pawn
	g.stacks.Set(carried, st)
	applyNow(g, command.Command{Kind: command.DesignateStockpile, X: 10, Y: 8, X2: 12, Y2: 10})
	sp := zoneViewAt(t, g, 10, 8)
	stockCmd(g, command.Command{Kind: command.SetStockpileFilter, Target: uint64(sp.ID), Name: "a.cup", Enabled: false})
	if f := zoneViewAt(t, g, 10, 8).Filter; slices.Contains(f, "a.cup") || !slices.Contains(f, "b.bell") {
		t.Fatalf("saved filter %v", f)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "items"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		book       item.Book
		cups       int // stacks of a.cup after the load
		bell       item.Kind
		bellInPile bool
	}{
		{"same set", saved, 2, bell, true},
		{"cup plugin gone", pluginItems("b.bell"), 0, cup, true},
		{"reordered", pluginItems("b.bell", "a.cup"), 2, cup, true},
		{"core only", item.Core(), 0, item.None, false},
	} {
		l, _, err := LoadContent(bytes.NewReader(buf.Bytes()), Content{Items: tc.book})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		cups, bells := 0, 0
		for _, id := range l.itemIDs {
			st, _ := l.stacks.Get(id)
			switch tc.book.Name(st.Kind) {
			case "a.cup":
				cups++
			case "b.bell":
				bells++
				if st.Kind != tc.bell || st.Qty != 5 {
					t.Errorf("%s: bell is kind %d qty %d", tc.name, st.Kind, st.Qty)
				}
			}
			if int(st.Kind) >= tc.book.Len() {
				t.Errorf("%s: a stack of kind %d past the book survived", tc.name, st.Kind)
			}
		}
		z, _ := l.zones.Zone(zone.ID(sp.ID))
		cupNow, _ := tc.book.KindOf("a.cup")
		if cups != tc.cups || bells != map[bool]int{true: 1}[tc.bell != item.None] ||
			tc.bell != item.None && z.Allows(tc.bell) != tc.bellInPile || tc.book.Plugin(cupNow) && z.Allows(cupNow) {
			t.Errorf("%s: cups %d bells %d filter %v", tc.name, cups, bells, z.Filter)
		}
		l.Step(TicksPerCell * 20) // nothing points at a removed stack
	}
}

// A plugin good counts toward the colony's wealth at its own value, and a
// patched core item at its patched one (PLG-026).
func TestPluginItemValueInWealth(t *testing.T) {
	t.Parallel()
	core := item.Core()
	defs := make([]item.Def, 0, item.CoreKinds+1)
	for k := range item.CoreKinds {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	defs[item.Wool].Value = 4
	defs = append(defs, item.Def{Label: "bell", Path: "Items/Parts", Value: 40, Mass: 1000, Stack: item.StackLimit})
	items := item.NewBook(defs, append(core.Keys(), "frontier.bell"))
	worth := func(c Content) int {
		g := New(Config{Seed: 1, Width: 10, Height: 10, Content: c})
		g.SpawnStack(item.Kind(item.CoreKinds), 3, 2, 2)
		g.SpawnStack(item.Wool, 10, 4, 4)
		sum, _ := g.WealthOf(0, len(g.itemIDs))
		return sum
	}
	if got, want := worth(Content{Items: items}), 3*40+10*4; got != want {
		t.Fatalf("wealth %d, want %d", got, want)
	}
}
