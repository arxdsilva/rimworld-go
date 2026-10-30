package reservation

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

// TestTryCountSharesAStack: count claims coexist while their sum fits the
// stack; Free and FreeFor report what is left.
func TestTryCountSharesAStack(t *testing.T) {
	tab := NewTable()
	a, b, c, s := entity.EntityID(1), entity.EntityID(2), entity.EntityID(3), entity.EntityID(9)
	if !tab.TryCount(a, s, 3, 10) || !tab.TryCount(b, s, 5, 10) {
		t.Fatal("two count claims within the stack should both succeed")
	}
	if tab.TryCount(c, s, 3, 10) {
		t.Fatal("a third claim past the stack's quantity must fail")
	}
	if got := tab.Free(s, 10); got != 2 {
		t.Fatalf("free = %d, want 2", got)
	}
	if got := tab.FreeFor(a, s, 10); got != 5 {
		t.Fatalf("free for a = %d, want 5 (its own 3 count as free to it)", got)
	}
	if !tab.TryCount(c, s, 2, 10) {
		t.Fatal("the last two fit")
	}
	if !tab.TryCount(a, s, 1, 10) || tab.Free(s, 10) != 2 {
		t.Fatalf("a re-claim replaces the count: free %d", tab.Free(s, 10))
	}
	if tab.TryCount(a, s, 0, 10) || tab.TryCount(0, s, 1, 10) || tab.TryCount(a, 0, 1, 10) {
		t.Fatal("zero count, owner or target never claims")
	}
}

// TestWholeAndCountExclude: a whole claim excludes count claims and the
// reverse; an owner's own count claim may become whole.
func TestWholeAndCountExclude(t *testing.T) {
	tab := NewTable()
	a, b, s := entity.EntityID(1), entity.EntityID(2), entity.EntityID(9)
	tab.TryEntity(a, s)
	if tab.TryCount(b, s, 1, 10) {
		t.Fatal("a count claim on a wholly held stack must fail")
	}
	if !tab.TryCount(a, s, 4, 10) || tab.Free(s, 10) != 0 || tab.FreeFor(a, s, 10) != 10 {
		t.Fatal("the whole owner keeps the whole stack")
	}
	tab.ReleaseOwner(a)
	tab.TryCount(b, s, 1, 10)
	if tab.TryEntity(a, s) {
		t.Fatal("a whole claim on a shared stack must fail")
	}
	if !tab.TryEntity(b, s) {
		t.Fatal("the only count owner may take the stack whole")
	}
	if o, ok := tab.EntityOwner(s); !ok || o != b || tab.Free(s, 10) != 0 {
		t.Fatalf("owner %d %v free %d", o, ok, tab.Free(s, 10))
	}
	if len(tab.Export()) != 1 || tab.Export()[0].Count != 0 {
		t.Fatalf("converted claim exported %+v", tab.Export())
	}
}

// TestEntityOwnerOfSharedStack: a shared stack reads as taken, by its
// lowest count owner whatever the claim order.
func TestEntityOwnerOfSharedStack(t *testing.T) {
	tab := NewTable()
	tab.TryCount(5, 9, 1, 10)
	tab.TryCount(2, 9, 1, 10)
	if o, ok := tab.EntityOwner(9); !ok || o != 2 {
		t.Fatalf("owner %d %v, want 2", o, ok)
	}
}

// TestReleaseEntityDropsACount: releasing one count frees exactly it.
func TestReleaseEntityDropsACount(t *testing.T) {
	tab := NewTable()
	tab.TryCount(1, 9, 3, 10)
	tab.TryCount(2, 9, 4, 10)
	tab.ReleaseEntity(1, 9)
	if got := tab.Free(9, 10); got != 6 {
		t.Fatalf("free = %d, want 6", got)
	}
	tab.ReleaseEntity(1, 9) // not held: nothing
	tab.ReleaseEntity(2, 9)
	if _, ok := tab.EntityOwner(9); ok || len(tab.Export()) != 0 {
		t.Fatalf("claims left: %+v", tab.Export())
	}
}

// TestReleaseTarget: every claim on a gone target is dropped, whole or
// count, and its owners keep their other claims.
func TestReleaseTarget(t *testing.T) {
	tab := NewTable()
	tab.TryCount(1, 9, 3, 10)
	tab.TryCount(2, 9, 4, 10)
	tab.TryEntity(1, 8)
	tab.TryEntity(3, 7)
	tab.ReleaseTarget(9)
	tab.ReleaseTarget(7)
	if _, ok := tab.EntityOwner(9); ok {
		t.Fatal("count claims survived ReleaseTarget")
	}
	if _, ok := tab.EntityOwner(7); ok {
		t.Fatal("whole claim survived ReleaseTarget")
	}
	want := []Claim{{Owner: 1, Target: 8}}
	if got := tab.Export(); !reflect.DeepEqual(got, want) {
		t.Fatalf("export %+v, want %+v", got, want)
	}
	tab.ReleaseTarget(42) // nothing held: no-op
}

// TestReleaseTile: the tile is free for anyone, its owner keeps the rest.
func TestReleaseTile(t *testing.T) {
	tab := NewTable()
	tab.TryTile(1, 2, 3)
	tab.TryTile(1, 4, 4)
	tab.ReleaseTile(2, 3)
	tab.ReleaseTile(9, 9)
	if !tab.TryTile(2, 2, 3) {
		t.Fatal("released tile should be claimable")
	}
	if o, ok := tab.TileOwner(4, 4); !ok || o != 1 {
		t.Fatal("other tile claim lost")
	}
	tab.ReleaseTile(4, 4)
	if got := len(tab.Export()); got != 1 {
		t.Fatalf("%d claims, want 1", got)
	}
}

// TestCountClaimsRoundTrip: counts survive Export/Import, the export is
// sorted by owner whatever the claim order, and a reload exports the same.
func TestCountClaimsRoundTrip(t *testing.T) {
	tab := NewTable()
	tab.TryCount(5, 9, 2, 10)
	tab.TryTile(5, 1, 1)
	tab.TryEntity(3, 8)
	tab.TryCount(3, 9, 7, 10)
	got := tab.Export()
	want := []Claim{
		{Owner: 3, Target: 8},
		{Owner: 3, Target: 9, Count: 7},
		{Owner: 5, Target: 9, Count: 2},
		{Owner: 5, Tile: true, X: 1, Y: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("export %+v, want %+v", got, want)
	}
	back := Import(got)
	if !reflect.DeepEqual(back.Export(), want) {
		t.Fatalf("reimport %+v", back.Export())
	}
	if back.Free(9, 10) != 1 || back.FreeFor(5, 9, 10) != 3 {
		t.Fatalf("reloaded free %d / %d", back.Free(9, 10), back.FreeFor(5, 9, 10))
	}
	if o, _ := back.EntityOwner(9); o != 3 {
		t.Fatalf("reloaded owner %d", o)
	}
}
