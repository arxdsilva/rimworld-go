package construction

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// A carpet is built only from carpet items, and carpet items build nothing
// else.
func TestCarpetStuffOnlyCarpets(t *testing.T) {
	t.Parallel()
	for _, c := range item.Carpets() {
		if !StuffOK(Carpet, c) {
			t.Fatalf("carpet from %s refused", c)
		}
		for _, k := range []Kind{Wall, Floor, Door, Workbench, Bed} {
			if StuffOK(k, c) {
				t.Fatalf("%s built from %s", k, c)
			}
		}
	}
	for _, s := range []item.Kind{item.Wood, item.Stone, item.Cloth, item.Straw, item.Leather} {
		if StuffOK(Carpet, s) {
			t.Fatalf("carpet built from raw %s", s)
		}
	}
	if DefaultStuff(Carpet) != item.CarpetStraw || DefaultStuff(Wall) != item.Wood {
		t.Fatal("default stuff")
	}
	if got := Stuffs(Carpet); len(got) != len(item.Carpets()) || got[0] != DefaultStuff(Carpet) {
		t.Fatalf("carpet stuffs = %v", got)
	}
}

// Deconstructing a carpet returns the whole item, not half of one.
func TestCarpetRefundsWhole(t *testing.T) {
	t.Parallel()
	if got := Refund(Carpet); got != 1 {
		t.Fatalf("carpet refund = %d, want 1", got)
	}
	if DefOf(Carpet).BlocksMove || DefOf(Carpet).Layer != LayerObject || WoodCost(Carpet) != 1 {
		t.Fatalf("carpet def = %+v", DefOf(Carpet))
	}
}
