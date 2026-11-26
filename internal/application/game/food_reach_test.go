// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// A nearer food stack behind a wall the pawn cannot cross must not hide
// the farther one it can fetch.
func TestBestFoodSkipsUnreachableStack(t *testing.T) {
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	g.ClearPlants(0, 0, 23, 11)
	g.SetGround(0, 0, 23, 11, "soil", "", "")
	id, err := g.SpawnNamedPawn(2, 5, "Eater")
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 12; y++ { // a wall splitting the map at x=3
		g.grid.SetWalkable(3, y, false)
	}
	g.structVer++
	if _, err := g.SpawnStack(item.SurvivalMeal, 5, 4, 5); err != nil { // closer-or-equal, unreachable
		t.Fatal(err)
	}
	if _, err := g.SpawnStack(item.SurvivalMeal, 5, 0, 5); err != nil { // reachable
		t.Fatal(err)
	}
	got, x, _, _ := g.bestFood(id, id)
	if got == 0 || x != 0 {
		t.Fatalf("bestFood chose %d at x=%d, want the reachable stack at x=0", got, x)
	}
}
