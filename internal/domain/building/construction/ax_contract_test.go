package construction

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestAxContractKinds (AX-0): every kind after the multi-analyzer is an
// AX contract kind, Planned until its owner flips it, with a unique name
// and a valid def; no earlier kind is Planned; only the traded urn, big
// screen and telescope are Unbuildable; the kind still fits its byte.
func TestAxContractKinds(t *testing.T) {
	t.Parallel()
	// Kind is a uint8, so the compiler already rejects a 256th kind; check
	// the last kind is valid and the one past it is not (no wrap to zero).
	if last := Kind(kindCount - 1); !last.Valid() || (last + 1).Valid() {
		t.Fatalf("%d kinds do not fit the saved byte", kindCount)
	}
	names := map[string]bool{}
	for _, d := range All() {
		if names[d.Name] {
			t.Errorf("duplicate name %q", d.Name)
		}
		names[d.Name] = true
		ax := d.Kind > MultiAnalyzer
		if d.Kind.Planned() && !ax || d.Kind != Kind(len(names)-1) { // owners flip their kinds live
			t.Errorf("%s (kind %d): planned %v", d.Name, d.Kind, d.Kind.Planned())
		}
		if d.Unbuildable != (d.Kind == Urn || d.Kind == BigScreenTV || d.Kind == Telescope) {
			t.Errorf("%s: unbuildable %v", d.Name, d.Unbuildable)
		}
		if ax && (d.Placed || len(Stuffs(d.Kind)) == 0 || !StuffOK(d.Kind, DefaultStuff(d.Kind))) {
			t.Errorf("%s: placed %v, stuffs %v", d.Name, d.Placed, Stuffs(d.Kind))
		}
		for _, e := range d.Extra {
			if (e.Item == 0) != (e.N == 0) || e.N < 0 {
				t.Errorf("%s: extra %+v", d.Name, e)
			}
		}
	}
}

// TestAxContractStuff (AX-0): Only fixes one material and StoneOnly takes
// stone blocks alone; kinds without them keep the old rules.
func TestAxContractStuff(t *testing.T) {
	t.Parallel()
	if !slices.Equal(Stuffs(SilverTile), []item.Kind{item.Silver}) || DefaultStuff(StrawMatting) != item.Hay ||
		StuffOK(CarpetFloor, item.Leather) || !StuffOK(CarpetFloor, item.Cloth) || StuffOK(IEDFoam, item.Steel) {
		t.Error("Only: one material")
	}
	if !slices.Equal(Stuffs(GrandStele), item.StoneBlocks()) || StuffOK(StoneTile, item.Wood) ||
		!StuffOK(Flagstone, item.MarbleBlocks) || !item.IsStoneBlocks(DefaultStuff(LargeStele)) {
		t.Error("StoneOnly: stone blocks")
	}
	if !slices.Equal(Stuffs(Wall)[:7], append(append([]item.Kind{item.Wood}, item.StoneBlocks()...), item.Steel)) || !StuffOK(Couch, item.Leather) {
		t.Error("the old rules hold")
	}
	if MaxHP(ShipBeam, item.Steel) != 400 || MaxHP(OrnateDoor, item.Wood) != 250 || MaxHP(SteelTile, item.Steel) != 84 {
		t.Errorf("hit points: beam %d, door %d, tile %d", MaxHP(ShipBeam, item.Steel), MaxHP(OrnateDoor, item.Wood), MaxHP(SteelTile, item.Steel))
	}
	if Movable(Column) || Movable(ShipEngine) || !Movable(Couch) || Movable(CarpetFloor) {
		t.Error("movable: fixed pieces stay")
	}
}
