// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package sense

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// pluginBook is core plus one plugin item: 700 g and 30 silver a unit.
func pluginBook() (item.Book, item.Kind) {
	core := item.Core()
	keys, defs := core.Keys(), make([]item.Def, 0, item.CoreKinds+1)
	for k := range item.CoreKinds {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	keys = append(keys, "demo.widget")
	defs = append(defs, item.Def{Label: "Widget", Mass: 700, Value: 30, Stack: 50})
	return item.NewBook(defs, keys), item.Kind(item.CoreKinds)
}

func pluginGear(qty int) (*Sense, *fakeGear, item.Kind) {
	book, k := pluginBook()
	f := &fakeGear{pawns: []entity.EntityID{1}, manip: 1000, book: book,
		stacks: map[entity.EntityID]item.Stack{10: {Kind: k, Qty: qty}}}
	s, _ := errandScene(f)
	return s, f, k
}

func TestPluginStowRespectsMass(t *testing.T) {
	t.Parallel()
	s, f, _ := pluginGear(100)
	limit := s.Gear.Capacity(1) / 700 // 57
	if s.Gear.Stow(1, 10, limit+1) || s.Gear.Has(1) || f.stacks[10].Qty != 100 {
		t.Fatal("stowed a plugin stack heavier than the room, or changed something")
	}
	if !s.Gear.Stow(1, 10, limit) || f.stacks[10].Qty != 100-limit {
		t.Fatalf("up to the limit: %+v", f.stacks[10])
	}
	if got := s.Gear.Room(1); got >= 700 {
		t.Fatalf("room left %d g, want under one unit", got)
	}
}

func TestPluginPackGearGrams(t *testing.T) {
	t.Parallel()
	s, f, k := pluginGear(10)
	if !s.Gear.Stow(1, 10, 10) {
		t.Fatal("stow")
	}
	e := s.Gear.Pack(1).Items[0]
	if got := s.Gear.PackGear(1, e).Grams; got != 7000 {
		t.Fatalf("PackGear grams %d, want 7000", got)
	}
	// BackToPack: a carried plugin stack goes back when it fits, not when it does not.
	f.stacks[11] = item.Stack{Kind: k, Qty: 10, Carrier: 1}
	s.FromPack[1] = 11
	if !s.Gear.BackToPack(1, 11) {
		t.Fatal("a plugin stack that fits did not go back")
	}
	f.stacks[12] = item.Stack{Kind: k, Qty: 60, Carrier: 1}
	s.FromPack[1] = 12
	if s.Gear.BackToPack(1, 12) {
		t.Fatal("went back past the room")
	}
}

func TestPluginPackWealth(t *testing.T) {
	t.Parallel()
	s, _, _ := pluginGear(10)
	if !s.Gear.Stow(1, 10, 10) {
		t.Fatal("stow")
	}
	if got := s.Gear.Wealth(); got != 300 {
		t.Fatalf("wealth %d, want 300", got)
	}
}
