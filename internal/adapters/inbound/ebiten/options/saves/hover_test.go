// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package saves

import (
	"testing"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
)

// RowCentre is the centre of the n-th shown row (the list shows newest
// first, so the slot index differs from the row), and false past the last.
func TestRowCentre(t *testing.T) {
	t.Parallel()
	f, b := newFake()
	f.env.CanLoad = true
	b.Slots = []ports.SaveSlot{
		{Name: "old", Modified: time.Unix(1_700_000_000, 0)},
		{Name: "new", Modified: time.Unix(1_700_000_900, 0)},
	}
	const sw, sh = 1280, 720
	x := sw/2 - 300 + slotRowW/2
	for n, y := range []int{120 + slotRowH/2, 120 + (slotRowH + 6) + slotRowH/2} {
		gx, gy, ok := b.RowCentre(n, sw, sh)
		if !ok || gx != x || gy != y {
			t.Fatalf("row %d: (%d,%d,%v), want (%d,%d)", n, gx, gy, ok, x, y)
		}
	}
	if _, _, ok := b.RowCentre(2, sw, sh); ok {
		t.Fatal("a third row exists")
	}
	b.Slots = nil
	if _, _, ok := b.RowCentre(0, sw, sh); ok {
		t.Fatal("a row in an empty list")
	}
}
