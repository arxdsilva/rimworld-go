// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestRaidersStealPluginGoods: a thief takes a plugin stack worth stealMin or
// more by the book's value (40 a unit here): two units, not one.
func TestRaidersStealPluginGoods(t *testing.T) {
	t.Parallel()
	items := pluginItems("frontier.bell")
	bell := item.Kind(item.CoreKinds)
	g := New(Config{Seed: 1, Width: 30, Height: 30, Content: Content{Items: items}})
	one, err := g.SpawnStack(bell, 1, 5, 5)
	if err != nil {
		t.Fatal(err)
	}
	m := raidMischief{g: g}
	if _, _, _, ok := m.Loot(5, 5, 10); ok {
		t.Fatalf("a 40 silver stack %d is worth stealing", one)
	}
	two, err := g.SpawnStack(bell, 2, 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	if id, x, y, ok := m.Loot(5, 5, 10); !ok || id != two || x != 8 || y != 8 {
		t.Fatalf("loot %d (%d, %d) %v, want the 80 silver stack %d", id, x, y, ok, two)
	}
}
