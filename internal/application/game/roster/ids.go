// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package roster

import (
	"slices"

	"rimworld-go/internal/domain/sim/entity"
)

// WithID inserts a known person into an id-ordered roster once, detaching
// its backing slice so a caller iterating the old roster remains stable.
func WithID(ids []entity.EntityID, id entity.EntityID) []entity.EntityID {
	i, found := slices.BinarySearch(ids, id)
	if found {
		return ids
	}
	out := make([]entity.EntityID, 0, len(ids)+1)
	out = append(append(append(out, ids[:i]...), id), ids[i:]...)
	return out
}

// WithoutID removes an id without changing an iterator's existing slice.
func WithoutID(ids []entity.EntityID, id entity.EntityID) []entity.EntityID {
	i := slices.Index(ids, id)
	if i < 0 {
		return ids
	}
	out := make([]entity.EntityID, 0, len(ids)-1)
	return append(append(out, ids[:i]...), ids[i+1:]...)
}
