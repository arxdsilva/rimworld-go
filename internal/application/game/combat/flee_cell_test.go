// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package combat

import "testing"

// openGrid is a fully walkable w x h query.
type openGrid struct{ w, h int }

func (g openGrid) Width() int             { return g.w }
func (g openGrid) Height() int            { return g.h }
func (g openGrid) Walkable(x, y int) bool { return true }

func TestFleeCellWalksTheQueueToTheFarthestCell(t *testing.T) {
	t.Parallel()
	x, y, ok := FleeCell(openGrid{40, 40}, 20, 20, 21, 20)
	if !ok {
		t.Fatal("an open field has a cell to flee to")
	}
	if x >= 20 || x < 20-fleeSearchR {
		t.Fatalf("fled to (%d,%d), want west of the start within reach", x, y)
	}
}

func TestFleeCellBoxedInFindsNothing(t *testing.T) {
	t.Parallel()
	if _, _, ok := FleeCell(openGrid{1, 1}, 0, 0, 0, 0); ok {
		t.Fatal("a one-cell map has no other cell")
	}
}
