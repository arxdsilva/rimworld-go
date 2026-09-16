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
	if !tab.TryTile(a, 1, 1) || !tab.TryTile(a, 1, 1) {
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
