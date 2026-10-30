package haul

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// A frame has two delivery slots (HAUL-001): its material, then its
// components (NG-1 §4.8). A share or route entry names a slot by key: the
// frame id for the material, the frame id with CompSlot set for the
// components. The keys fit the saved maps unchanged (entity ids stay far
// below it).
const CompSlot = entity.EntityID(1) << 62

// SlotKey is the delivery slot of frame fid that takes items of kind k.
func SlotKey(fid entity.EntityID, k item.Kind) entity.EntityID {
	if k == item.Components {
		return fid | CompSlot
	}
	return fid
}

// SlotFrame splits a slot key into its frame id and whether it is the
// components slot.
func SlotFrame(key entity.EntityID) (fid entity.EntityID, comp bool) {
	return key &^ CompSlot, key&CompSlot != 0
}

// SlotOpen reports the frame's slot for items of kind k still takes them
// (the walk and the hand-over check, which ignore loads on the way).
func SlotOpen(fr construction.Frame, k item.Kind) bool {
	if k == item.Components {
		return fr.Comp < fr.CompNeed
	}
	return k == fr.Stuff && fr.Wood < fr.Need
}

// SupplyKind is what a hand-ordered delivery to frame fr brings: its
// material until that is in, then its components.
func SupplyKind(fr construction.Frame) item.Kind {
	if fr.Wood >= fr.Need && fr.Comp < fr.CompNeed {
		return item.Components
	}
	return fr.Stuff
}

// SupplyNeed is how much of kind k frame fr still lacks.
func SupplyNeed(fr construction.Frame, k item.Kind) int {
	if k == item.Components {
		return fr.CompNeed - fr.Comp
	}
	return fr.Need - fr.Wood
}

// SupplyHave is how much of kind k frame fr already holds.
func SupplyHave(fr construction.Frame, k item.Kind) int {
	if k == item.Components {
		return fr.Comp
	}
	return fr.Wood
}
