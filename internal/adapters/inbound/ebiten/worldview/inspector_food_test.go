// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package worldview

import (
	"slices"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
)

// TestInspectCaravanFoodAndPrisoner: the caravan's inspector shows its food
// days (tenths of a day) after "Heading for", and tags a prisoner member;
// a Prisoner slice shorter than Members is tolerated.
func TestInspectCaravanFoodAndPrisoner(t *testing.T) {
	t.Parallel()
	g := colonyGame(t)
	c := &g.world.Caravans[0]
	c.FoodDays = 28
	c.Prisoner = []bool{false, true}
	sh, v := newFake(g)
	v.Show("caravan:7")
	frame(sh, v, -1, -1, inputframe.Raw{})
	_, lines, _ := (inspector{v}).inspect()
	if !slices.Contains(lines, line{"Food", "2.8 days"}) {
		t.Fatalf("no food line in %v", lines)
	}
	if !slices.Contains(lines, line{"Bo (prisoner)", "flu"}) || !slices.Contains(lines, line{"Ada", "well"}) {
		t.Fatalf("members in %v", lines)
	}

	c.Prisoner = []bool{true} // shorter than Members
	v.read.dirty = true
	frame(sh, v, -1, -1, inputframe.Raw{})
	_, lines, _ = (inspector{v}).inspect()
	if !slices.Contains(lines, line{"Ada (prisoner)", "well"}) || !slices.Contains(lines, line{"Bo", "flu"}) {
		t.Fatalf("short Prisoner slice: %v", lines)
	}
	c.Prisoner = nil
	c.FoodDays = 0
	v.read.dirty = true
	frame(sh, v, -1, -1, inputframe.Raw{})
	_, lines, _ = (inspector{v}).inspect()
	if !slices.Contains(lines, line{"Food", "0.0 days"}) {
		t.Fatalf("an unfed caravan: %v", lines)
	}
}
