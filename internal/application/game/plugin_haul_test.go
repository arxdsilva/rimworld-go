// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// limitedBook is core's kinds plus one plugin good filed under components
// whose stack holds limit units.
func limitedBook(limit int) (item.Book, item.Kind) {
	core := item.Core()
	defs, all := make([]item.Def, 0, item.CoreKinds+1), core.Keys()
	for k := range item.CoreKinds {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	defs = append(defs, item.Def{Label: "bell", Path: "Manufactured/Components", Mass: 1000, Value: 40, Stack: limit})
	return item.NewBook(defs, append(all, "frontier.bell")), item.Kind(item.CoreKinds)
}

// haulLimited hauls qty bells onto a stockpile (or a built shelf) and
// reports the largest stack seen on a storage cell and the largest load
// carried on the way, with the units stored at the end.
func haulLimited(t *testing.T, limit, qty int, shelf bool) (g *Game, bell item.Kind, maxCell, maxCarry, stored int) {
	t.Helper()
	items, bell := limitedBook(limit)
	g = New(Config{Seed: 1, Width: 22, Height: 14, Content: Content{Items: items}})
	pawn, _ := g.SpawnNamedPawn(1, 1, "Ada")
	if shelf {
		placeBuilt(t, g, construction.Shelf, construction.South, 14, 8)
	} else {
		applyNow(g, command.Command{Kind: command.DesignateStockpile, X: 14, Y: 8, X2: 16, Y2: 10})
	}
	g.SpawnStack(bell, qty, 1, 4)
	for i := 0; i < 700; i++ {
		g.SetNeeds(pawn, needs.Full, needs.Full)
		g.Step(TicksPerCell * 3)
		stored = 0
		for _, st := range stacksOf(g, bell) {
			if _, ok := g.storageAt(st.X, st.Y); ok && st.Carrier == 0 {
				stored += st.Qty
				maxCell = max(maxCell, st.Qty)
			} else if st.Carrier != 0 {
				maxCarry = max(maxCarry, st.Qty)
			}
		}
		if stored == qty {
			break
		}
	}
	return g, bell, maxCell, maxCarry, stored
}

// TestPluginHaulStackLimit: a plugin kind with stack_limit 25 is stacked
// and hauled at most 25 to a cell; a limit of 1 keeps one unit a stack.
func TestPluginHaulStackLimit(t *testing.T) {
	t.Parallel()
	_, _, maxCell, _, stored := haulLimited(t, 25, 60, false)
	if stored != 60 || maxCell > 25 || maxCell == 0 {
		t.Fatalf("stored %d, largest stack %d, want 60 and at most 25", stored, maxCell)
	}
	_, _, maxCell, _, stored = haulLimited(t, 1, 3, false)
	if stored != 3 || maxCell != 1 {
		t.Fatalf("limit 1: stored %d, largest stack %d", stored, maxCell)
	}
	if !stackFirstTakes(t, 25) || stackFirstTakes(t, 1) {
		t.Fatal("StackFirst must gather a stackable plugin kind and skip a one-per-stack one")
	}
}

// stackFirstTakes reports a bench's StackFirst hauls a bell stack of a kind
// whose stack_limit is limit, in the setup of stackFirstGame.
func stackFirstTakes(t *testing.T, limit int) bool {
	t.Helper()
	items, bell := limitedBook(limit)
	g := New(Config{Seed: 1, Width: 20, Height: 12, Content: Content{Items: items}})
	ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
	buildNow(t, g, construction.Workbench, item.Wood, construction.South, 16, 8)
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 2, Y: 1, X2: 3, Y2: 2})
	g.Step(1)
	st := stack(t, g, bell, 20, 8, 9)
	return craftPrep{g}.StackFirst(ada, 19, 11, []entity.EntityID{st})
}

// TestPluginHaulCapacity: the load planned and picked up is capped by the
// book's limit, not the core default.
func TestPluginHaulCapacity(t *testing.T) {
	t.Parallel()
	_, _, _, carry, stored := haulLimited(t, 10, 35, false)
	if stored != 35 || carry == 0 || carry > 10 {
		t.Fatalf("stored %d, largest load %d, want 35 and 1..10", stored, carry)
	}
}

// TestPluginShelfStores: a hauler puts a plugin stack on a built shelf and
// the filter tree lists the kind as allowed.
func TestPluginShelfStores(t *testing.T) {
	t.Parallel()
	g, _, maxCell, _, stored := haulLimited(t, 25, 40, true)
	if stored != 40 || maxCell > 25 {
		t.Fatalf("shelf stored %d, largest stack %d", stored, maxCell)
	}
	items, _ := limitedBook(25)
	for _, z := range g.World().Zones {
		if z.Kind != "shelf" || z.Shelf == 0 {
			continue
		}
		for _, r := range StorageFilterRowsIn(items, z.Kind, z.Filter) {
			if r.Leaf && r.Label == "bell" && r.On {
				return
			}
		}
		t.Fatalf("shelf filter %v does not list the bell", z.Filter)
	}
	t.Fatal("no shelf zone")
}
