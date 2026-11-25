// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package reservation

import (
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

func TestTable_RejectsExclusiveEntityConflict(t *testing.T) {
	tab := NewTable()
	a, b, item := entity.EntityID(1), entity.EntityID(2), entity.EntityID(9)
	if !tab.TryEntity(a, item) {
		t.Fatal("first claim should succeed")
	}
	if tab.TryEntity(b, item) {
		t.Fatal("second pawn must not take the same entity")
	}
	if owner, ok := tab.EntityOwner(item); !ok || owner != a {
		t.Fatalf("owner = %v %d", ok, owner)
	}
}

func TestTable_RejectsExclusiveTileConflict(t *testing.T) {
	tab := NewTable()
	a, b := entity.EntityID(1), entity.EntityID(2)
	if !tab.TryTile(a, 3, 4) {
		t.Fatal("first tile claim should succeed")
	}
	if tab.TryTile(b, 3, 4) {
		t.Fatal("second pawn must not take the same tile")
	}
}

func TestTable_SameOwnerReclaimOK(t *testing.T) {
	tab := NewTable()
	a := entity.EntityID(1)
	first, again := tab.TryTile(a, 1, 1), tab.TryTile(a, 1, 1)
	if !first || !again {
		t.Fatal("owner may refresh their own claim")
	}
}

func TestTable_ReleaseOwnerClearsClaims(t *testing.T) {
	tab := NewTable()
	a, item := entity.EntityID(1), entity.EntityID(8)
	tab.TryEntity(a, item)
	tab.TryTile(a, 2, 2)
	tab.ReleaseOwner(a)
	if _, ok := tab.EntityOwner(item); ok {
		t.Fatal("entity claim should be gone")
	}
	if _, ok := tab.TileOwner(2, 2); ok {
		t.Fatal("tile claim should be gone")
	}
	if !tab.TryEntity(entity.EntityID(2), item) {
		t.Fatal("another pawn should be able to claim after release")
	}
}

func TestClaimChangeHistory(t *testing.T) {
	tab := NewTable()
	owner, target := entity.EntityID(1), entity.EntityID(2)
	entityChange := func(id entity.EntityID, x, y int) bool { return id == target }
	tileChange := func(id entity.EntityID, x, y int) bool { return id == 0 && x == 7 && y == 8 }
	rev := tab.Ver()
	tab.TryTile(owner, 50, 50)
	if tab.ChangedSince(rev, entityChange) || tab.ChangedSince(rev, tileChange) {
		t.Fatal("unrelated tile invalidated consumer")
	}
	rev = tab.Ver()
	tab.TryCount(owner, target, 2, 10)
	if !tab.ChangedSince(rev, entityChange) || tab.ChangedSince(rev, tileChange) {
		t.Fatal("count claim not tracked precisely")
	}
	rev = tab.Ver()
	tab.TryCount(owner, target, 2, 10)
	if tab.Ver() != rev || tab.ChangedSince(rev, entityChange) {
		t.Fatal("idempotent claim changed revision")
	}
	tab.TryCount(owner, target, 3, 10)
	if !tab.ChangedSince(rev, entityChange) {
		t.Fatal("count replacement not tracked")
	}
	rev = tab.Ver()
	tab.TryTile(owner, 7, 8)
	tab.TryEntity(owner, target)
	if !tab.ChangedSince(rev, tileChange) || !tab.ChangedSince(rev, entityChange) {
		t.Fatal("tile/whole conversion not tracked")
	}
	rev = tab.Ver()
	tab.ReleaseOwner(owner)
	if !tab.ChangedSince(rev, tileChange) || !tab.ChangedSince(rev, entityChange) {
		t.Fatal("bulk release lost affected target")
	}
	tab.TryEntity(owner, target)
	rev = tab.Ver()
	tab.ReleaseTarget(target)
	if !tab.ChangedSince(rev, entityChange) {
		t.Fatal("target removal not tracked")
	}
	rev = tab.Ver()
	for i := 0; i <= changeHistory; i++ {
		tab.TryTile(owner, i, 50)
	}
	if !tab.ChangedSince(rev, tileChange) {
		t.Fatal("expired history did not conservatively invalidate")
	}
}
