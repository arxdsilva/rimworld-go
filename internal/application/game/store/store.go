// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package store

import "rimworld-go/internal/domain/sim/entity"

// Store is the root (no methods of its own): its components carry the
// entry points.
type Store struct {
	Urgent    Urgent
	Paste     Paste
	Buildings Buildings
}

// Saved is the store's saved state (nil fields while nothing is held).
type Saved struct {
	// Urgent is the stacks marked to haul urgently, by id.
	Urgent []entity.EntityID
}

// Export copies the saved state out.
func (s *Store) Export() Saved { return Saved{Urgent: s.Urgent.Export()} }

// Import restores an exported state.
func (s *Store) Import(v Saved) { s.Urgent.Import(v.Urgent) }
