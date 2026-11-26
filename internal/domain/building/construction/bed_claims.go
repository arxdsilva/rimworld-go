// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package construction

// The sleeping-slot policy is separate from the persisted building record.
// All mutations return a building value, keeping the established save
// fields and preserving unrelated construction state, including paid costs.

// BedSlot is the sleeping slot pawn id holds here, or -1.
func BedSlot(b Building, id uint64) int {
	switch {
	case id == 0:
		return -1
	case b.Claimant == id:
		return 0
	case b.Claimant2 == id && b.Kind.Sleepers() > 1:
		return 1
	}
	return -1
}

// CanClaimBed reports whether pawn id may sleep here (it already holds a
// slot, or one is free).
func CanClaimBed(b Building, id uint64) bool {
	if BedSlot(b, id) >= 0 {
		return true
	}
	return b.Claimant == 0 || (b.Kind.Sleepers() > 1 && b.Claimant2 == 0)
}

// ClaimBed gives pawn id a slot (keeping one it already holds).
func ClaimBed(b Building, id uint64) Building {
	switch {
	case BedSlot(b, id) >= 0:
	case b.Claimant == 0:
		b.Claimant = id
	case b.Kind.Sleepers() > 1 && b.Claimant2 == 0:
		b.Claimant2 = id
	}
	return b
}

// ReleaseBed frees pawn id's slot.
func ReleaseBed(b Building, id uint64) Building {
	if b.Claimant == id {
		b.Claimant = 0
	}
	if b.Claimant2 == id {
		b.Claimant2 = 0
	}
	return b
}

// BedOwner is the first claimant that is not id (for "owned by" reasons).
func BedOwner(b Building, id uint64) uint64 {
	if b.Claimant != 0 && b.Claimant != id {
		return b.Claimant
	}
	if b.Claimant2 != id {
		return b.Claimant2
	}
	return 0
}
