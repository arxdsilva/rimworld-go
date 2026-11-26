// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package faction

// FindByID returns the live faction in fs, allowing changes to its
// diplomatic state to update the authoritative world roster.
func FindByID(fs []Faction, id int) *Faction {
	for i := range fs {
		if fs[i].ID == id {
			return &fs[i]
		}
	}
	return nil
}

// RefByID resolves a saved faction ID to the 1-based index used by
// visiting groups and raids. A missing ID has no valid reference.
func RefByID(fs []Faction, id int) int {
	for i := range fs {
		if fs[i].ID == id {
			return i + 1
		}
	}
	return 0
}
