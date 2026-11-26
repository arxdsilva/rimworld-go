// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package saveutil

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/terrain"
)

type roofEntry struct {
	ID entity.EntityID
	V  construction.Building
}

func TestLegacyRoofHashCellsPreservesOnlyDerivedSupport(t *testing.T) {
	const w, h = 32, 24
	land := terrain.NewMap(w, h)
	land.Set(2, 2, terrain.Rock)
	buildings := []roofEntry{
		{1, construction.Building{Kind: construction.Column, X: 20, Y: 18}},
		{2, construction.Building{Kind: construction.Wall, X: 16, Y: 5}},
	}
	support := func(x, y int) bool {
		return (x == 2 && y == 2) || (x == 20 && y == 18) || (x == 16 && y == 5)
	}
	circle := roof.SupportField(w, h, support)
	cells := make([]uint8, w*h)
	for i, ok := range circle {
		if ok {
			cells[i] |= uint8(roof.Supported)
		}
	}
	corner := 8*w + 8
	if circle[corner] {
		t.Fatal("live support unexpectedly square")
	}
	cells[corner] |= uint8(roof.Natural | roof.NaturalThick)
	before := slices.Clone(cells)
	out := LegacyRoofHashCells(w, h, land, buildings, cells)
	if !slices.Equal(before, cells) {
		t.Fatal("hash projection mutated authoritative roof grid")
	}
	for i, got := range out {
		x, y := i%w, i/w
		want := false
		for _, p := range [][2]int{{2, 2}, {20, 18}, {16, 5}} {
			dx, dy := x-p[0], y-p[1]
			if max(dx, -dx) <= roof.SupportRange && max(dy, -dy) <= roof.SupportRange {
				want = true
				break
			}
		}
		if (roof.Cell(got).Has(roof.Supported)) != want {
			t.Fatalf("historical support at (%d,%d): %b want %v", x, y, got, want)
		}
		if got&^uint8(roof.Supported) != cells[i]&^uint8(roof.Supported) {
			t.Fatalf("authoritative flags changed at %d", i)
		}
	}
	if !roof.Cell(out[corner]).Has(roof.Supported | roof.Natural | roof.NaturalThick) {
		t.Fatal("historical cache bit or actual thick roof lost")
	}
}
