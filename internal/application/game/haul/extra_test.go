package haul

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

// AX-4: a frame's secondary materials are slots of their own, wanted
// after the material and the components, and given into ExtraGot.
func TestExtraSlots(t *testing.T) {
	// a watermill: 31 wood, 3 components, 9 steel (Def.Extra)
	fr := construction.Frame{Kind: construction.Watermill, Stuff: item.Wood, Need: 31, CompNeed: 3}
	slots := Slots(nil, 7, fr)
	if len(slots) != 3 || slots[2].Kind != item.Steel || slots[2].Need != 9 || slots[2].Key != ExtraKey(7, 0) {
		t.Fatalf("slots %+v", slots)
	}
	if fid, comp := SlotFrame(ExtraKey(7, 2)); fid != 7 || comp {
		t.Fatalf("an extra key names its frame: %d %v", fid, comp)
	}
	if SupplyKind(fr) != item.Wood {
		t.Fatal("the material first")
	}
	fr.Wood, fr.Comp = 31, 3
	if SupplyKind(fr) != item.Steel || SupplyNeed(fr, item.Steel) != 9 || fr.Ready() {
		t.Fatalf("then the steel: %v %d ready=%v", SupplyKind(fr), SupplyNeed(fr, item.Steel), fr.Ready())
	}
	if !SlotOpen(fr, ExtraKey(7, 0), item.Steel) || SlotOpen(fr, ExtraKey(7, 0), item.Wood) {
		t.Fatal("the steel slot takes steel only")
	}
	fr = Give(fr, ExtraKey(7, 0), 9)
	if fr.ExtraGot[0] != 9 || !fr.Ready() || SlotOpen(fr, ExtraKey(7, 0), item.Steel) {
		t.Fatalf("delivered: %+v", fr)
	}
	names, got, need := ExtraView(fr)
	if names[0] != item.Steel.String() || got[0] != 9 || need[0] != 9 || names[1] != "" {
		t.Fatalf("view %v %v %v", names, got, need)
	}
	fr.Reinstall = 3
	if fr.ExtraNeed(0) != 0 || len(Slots(nil, 7, fr)) != 2 {
		t.Fatal("a reinstall needs no secondary material")
	}
}

// Same kind as the material (a billiards table of wood takes wood as a
// secondary material too): the material fills first.
func TestExtraSameKindAsMaterial(t *testing.T) {
	fr := construction.Frame{Kind: construction.BilliardsTable, Stuff: item.Wood, Need: 9}
	if s, ok := SlotFor(1, fr, item.Wood); !ok || s.Key != 1 {
		t.Fatalf("the material slot first: %+v", s)
	}
	fr.Wood = 9
	if s, ok := SlotFor(1, fr, item.Wood); !ok || s.Key != ExtraKey(1, 1) || s.Need != 3 {
		t.Fatalf("then the wood extra: %+v", s)
	}
}
