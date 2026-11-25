// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package planet

import (
	"rimworld-go/internal/domain/faction"
	"slices"
	"testing"
)

func TestTerritoriesOverLandAndAfterBaseLoss(t *testing.T) {
	// 0-1-2-3-4-5: tile 3 is water. Factions at 0 and 2, isolated 4/5.
	p := &Planet{
		Center: make([][3]float32, 6), Water: []uint8{Land, Land, Land, 1, Land, Land},
		NeighOff: []uint32{0, 1, 3, 5, 7, 9, 10}, Neigh: []int32{1, 0, 2, 1, 3, 2, 4, 3, 5, 4},
		Settlement:  []int32{0, -1, 1, -1, -1, -1},
		Factions:    []faction.Faction{{ID: 10}, {ID: 20}},
		settlements: []settlementRef{{faction: 0, tile: 0}, {faction: 1, tile: 2}},
	}
	want := []int{10, 10, 20, -1, -1, -1}
	if got := Territories(p); !slices.Equal(got, want) {
		t.Fatalf("territory %v want %v", got, want)
	}
	p.Settlement[0] = -1
	want = []int{20, 20, 20, -1, -1, -1}
	got := Territories(p)
	if !slices.Equal(got, want) {
		t.Fatalf("after base loss %v", got)
	}
	got[0] = 999
	if !slices.Equal(Territories(p), want) {
		t.Fatal("projection aliases world")
	}
}
