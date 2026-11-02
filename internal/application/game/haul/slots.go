package haul

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// A frame has delivery slots (HAUL-001): its material, then its
// components (NG-1 §4.8), then its secondary materials (AX-4:
// construction.Def.Extra, gold in an ornate door, steel in a watermill).
// A share or route entry names a slot by key: the frame id for the
// material, the frame id with CompSlot set for the components, with
// extraBits(i) set for secondary material i. The keys fit the saved maps
// unchanged (entity ids stay far below them).
const CompSlot = entity.EntityID(1) << 62

// extraShift places a secondary slot's number (1..3) at bits 59-60.
const extraShift = 59

const slotBits = CompSlot | entity.EntityID(3)<<extraShift

// ExtraKey is the delivery slot of frame fid's secondary material i.
func ExtraKey(fid entity.EntityID, i int) entity.EntityID {
	return fid | entity.EntityID(i+1)<<extraShift
}

// SlotKey is the delivery slot of frame fid that takes items of kind k:
// the material's or the components' (SlotFor also finds a secondary one).
func SlotKey(fid entity.EntityID, k item.Kind) entity.EntityID {
	if k == item.Components {
		return fid | CompSlot
	}
	return fid
}

// SlotFrame splits a slot key into its frame id and whether it is the
// components slot.
func SlotFrame(key entity.EntityID) (fid entity.EntityID, comp bool) {
	return key &^ slotBits, key&CompSlot != 0
}

// slotExtra is the secondary material a key names (-1: none).
func slotExtra(key entity.EntityID) int { return int(key>>extraShift&3) - 1 }

// Keys are every slot key of frame fid (the material's first).
func Keys(fid entity.EntityID) [5]entity.EntityID {
	return [5]entity.EntityID{fid, fid | CompSlot, ExtraKey(fid, 0), ExtraKey(fid, 1), ExtraKey(fid, 2)}
}

// Slot is one delivery slot of a frame: its key, the kind it takes, how
// many it needs in all and how many it holds.
type Slot struct {
	Key        entity.EntityID
	Kind       item.Kind
	Need, Have int
}

// Open reports the slot still takes items.
func (s Slot) Open() bool { return s.Have < s.Need }

// SlotOf is frame fr's (id fid) slot named by key; ok false for a slot
// the frame does not have.
func SlotOf(fid entity.EntityID, fr construction.Frame, key entity.EntityID) (Slot, bool) {
	_, comp := SlotFrame(key)
	switch i := slotExtra(key); {
	case comp:
		return Slot{Key: key, Kind: item.Components, Need: fr.CompNeed, Have: fr.Comp}, fr.CompNeed > 0
	case i >= 0:
		n := fr.ExtraNeed(i)
		return Slot{Key: key, Kind: construction.DefOf(fr.Kind).Extra[i].Item, Need: n, Have: fr.ExtraGot[i]}, n > 0
	}
	return Slot{Key: fid, Kind: fr.Stuff, Need: fr.Need, Have: fr.Wood}, true
}

// Slots appends frame fr's slots (id fid) to dst: the material, the
// components and the secondary materials it takes, in that order.
func Slots(dst []Slot, fid entity.EntityID, fr construction.Frame) []Slot {
	for _, key := range Keys(fid) {
		if s, ok := SlotOf(fid, fr, key); ok {
			dst = append(dst, s)
		}
	}
	return dst
}

// SlotFor is the first open slot of frame fr taking kind k (the material
// before a secondary slot of the same kind); ok false when none is open.
func SlotFor(fid entity.EntityID, fr construction.Frame, k item.Kind) (Slot, bool) {
	for _, key := range Keys(fid) {
		if s, ok := SlotOf(fid, fr, key); ok && s.Kind == k && s.Open() {
			return s, true
		}
	}
	return Slot{}, false
}

// SlotOpen reports frame fr's slot key still takes items of kind k (the
// walk and the hand-over check, which ignore loads on the way).
func SlotOpen(fr construction.Frame, key entity.EntityID, k item.Kind) bool {
	fid, _ := SlotFrame(key)
	s, ok := SlotOf(fid, fr, key)
	return ok && s.Kind == k && s.Open()
}

// Give hands n items to frame fr's slot key and returns the frame.
func Give(fr construction.Frame, key entity.EntityID, n int) construction.Frame {
	_, comp := SlotFrame(key)
	switch i := slotExtra(key); {
	case comp:
		fr.Comp += n
	case i >= 0:
		fr.ExtraGot[i] += n
	default:
		fr.Wood += n
	}
	return fr
}

// SupplyKind is what a hand-ordered delivery to frame fr brings: its
// material until that is in, then its components, then its secondary
// materials.
func SupplyKind(fr construction.Frame) item.Kind {
	for _, key := range Keys(0) {
		if s, ok := SlotOf(0, fr, key); ok && s.Open() {
			return s.Kind
		}
	}
	return fr.Stuff
}

// SupplyNeed is how much of kind k frame fr still lacks (its first open
// slot of that kind).
func SupplyNeed(fr construction.Frame, k item.Kind) int {
	s, _ := SlotFor(0, fr, k)
	return s.Need - s.Have
}

// SupplyHave is how much of kind k frame fr's first open slot of that
// kind already holds (the material's when none is open).
func SupplyHave(fr construction.Frame, k item.Kind) int {
	if s, ok := SlotFor(0, fr, k); ok {
		return s.Have
	}
	return fr.Wood
}

// ExtraView is frame fr's secondary materials for its view: kind names,
// delivered and needed, slot by slot ("" unused).
func ExtraView(fr construction.Frame) (names [3]string, got, need [3]int) {
	for i, c := range construction.DefOf(fr.Kind).Extra {
		if n := fr.ExtraNeed(i); n > 0 {
			names[i], got[i], need[i] = c.Item.String(), fr.ExtraGot[i], n
		}
	}
	return names, got, need
}
