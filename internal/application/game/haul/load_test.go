package haul

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// TestSplitLoadKeepsCorpseSpecies: a load split off a stack of elk
// corpses is still elk (the split once dropped the species: deer).
func TestSplitLoadKeepsCorpseSpecies(t *testing.T) {
	st := item.Stack{Kind: item.CorpseAnimal, Qty: 3, X: 4, Y: 5, Rot: 77, Animal: uint8(animal.Elk), DiedAt: 900}
	got := SplitLoad(st, 1)
	if got.Qty != 1 || got.Animal != uint8(animal.Elk) || got.DiedAt != 900 || got.Rot != 77 || got.X != 4 {
		t.Fatalf("split load = %+v", got)
	}
}

func TestToppable(t *testing.T) {
	if !Toppable(item.Stack{Kind: item.Wood, Qty: 900}) || Toppable(item.Stack{Kind: item.Wood, Qty: 901}) {
		t.Fatal("a plain stack takes a full load")
	}
	if !Toppable(item.Stack{Kind: item.Pemmican, Qty: 74}) || Toppable(item.Stack{Kind: item.Pemmican, Qty: 75}) {
		t.Fatal("a small-limit stack takes any room")
	}
}

func TestCellRoomAndCapacity(t *testing.T) {
	var c Cell
	if c.Room(75, 1) != 75 {
		t.Fatal("an empty cell takes a full stack")
	}
	c.Add(true, 60)
	if c.Room(75, 1) != 15 || Capacity(75, c.Room(75, 1)) != 15 {
		t.Fatal("60 of 75 leaves room for 15")
	}
	if c.Room(75, 3) != 75 {
		t.Fatal("a shelf with a free slot takes a full stack")
	}
	c.Add(true, 70)
	c.Add(false, 5)
	if c.Room(75, 3) != 15 {
		t.Fatal("a full shelf tops up its smallest stack of the kind")
	}
	var other Cell
	other.Add(false, 5)
	if other.Room(75, 1) != 0 || Capacity(75, 0) != 1 {
		t.Fatal("a cell of another kind takes none; a load is never empty")
	}
	if Capacity(item.StackLimit, item.StackLimit) != item.CarryLimit || Capacity(10, 75) != 10 {
		t.Fatal("capacity is the carry limit and the stack limit")
	}
}

func TestSlotsAndShares(t *testing.T) {
	key := SlotKey(7, item.Components)
	if fid, comp := SlotFrame(key); fid != 7 || !comp || SlotKey(7, item.Wood) != 7 {
		t.Fatalf("slot keys: %d %v", fid, comp)
	}
	fr := construction.Frame{Stuff: item.Wood, Need: 10, Wood: 10, CompNeed: 2}
	if !SlotOpen(fr, item.Components) || SlotOpen(fr, item.Wood) || SlotOpen(fr, item.Steel) {
		t.Fatal("slot open: the components, once the wood is in")
	}
	if SupplyKind(fr) != item.Components || SupplyNeed(fr, item.Components) != 2 || SupplyHave(fr, item.Wood) != 10 {
		t.Fatal("supply of a frame missing only components")
	}
	fr.Wood = 4
	if SupplyKind(fr) != item.Wood || SupplyNeed(fr, item.Wood) != 6 || SupplyHave(fr, item.Components) != 0 {
		t.Fatal("supply of a frame missing wood")
	}
	sh := Shares([]entity.EntityID{1, 2, 3}, []int{30, 50, 40}, 60)
	if len(sh) != 2 || sh[0] != (Share{1, 30}) || sh[1] != (Share{2, 30}) {
		t.Fatalf("shares %v", sh)
	}
	if r := Route(sh); len(r) != 2 || r[0] != 1 || r[1] != 2 {
		t.Fatalf("route %v", r)
	}
}
