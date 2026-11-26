// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/geology"
)

type orePlacementFixture struct {
	w, h   int
	taken  map[[2]int]bool
	veins  map[[2]int]geology.Ore
	stacks []item.Stack
}

func (f *orePlacementFixture) LumpSize() (int, int)       { return f.w, f.h }
func (f *orePlacementFixture) LumpTaken() map[[2]int]bool { return f.taken }
func (*orePlacementFixture) LumpFree(x, y int) bool       { return x != 10 && y != 10 }
func (f *orePlacementFixture) LumpVein(x, y int, ore geology.Ore) {
	if f.veins == nil {
		f.veins = map[[2]int]geology.Ore{}
	}
	f.veins[[2]int{x, y}] = ore
}
func (f *orePlacementFixture) LumpStack(k item.Kind, qty, x, y int) {
	f.stacks = append(f.stacks, item.Stack{Kind: k, Qty: qty, X: x, Y: y})
}

func TestOrePlacementRespectsOccupiedTilesAndMineYields(t *testing.T) {
	t.Parallel()
	f := &orePlacementFixture{w: 60, h: 60, taken: map[[2]int]bool{{42, 30}: true}}
	expect := LumpCells(f.w, f.h, 3, func(x, y int) bool {
		return x != 10 && y != 10 && !f.taken[[2]int{x, y}]
	})
	cells := PlaceLump(f, item.Steel, geology.MineYields[geology.Steel]*5, 3)
	if !slices.Equal(cells, expect) || len(f.veins) != len(cells) || len(f.stacks) != 0 {
		t.Fatalf("ore placement differs: got=%v want=%v veins=%v stacks=%v", cells, expect, f.veins, f.stacks)
	}
	for _, c := range cells {
		if got := f.veins[[2]int{c % f.w, c / f.w}]; got != geology.Steel {
			t.Fatalf("cell %d has %v ore", c, got)
		}
	}
}

func TestOrePlacementDropsNonVeinGoods(t *testing.T) {
	t.Parallel()
	f := &orePlacementFixture{w: 60, h: 60}
	if cells := PlaceLump(f, item.Wood, 75, 0); len(cells) != 0 ||
		len(f.stacks) != 1 || f.stacks[0].Kind != item.Wood || f.stacks[0].Qty != 75 ||
		len(f.veins) != 0 {
		t.Fatalf("non-vein goods placement changed: cells=%v stacks=%v veins=%v", cells, f.stacks, f.veins)
	}
}
