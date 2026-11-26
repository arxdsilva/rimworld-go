// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package roofing owns physical cave-in resolution, separate from map
// topology and presentation. The host applies damage through native owners.
package roofing

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

type Debris interface {
	Loose(x, y int, buf []entity.EntityID) []entity.EntityID
	Stack(entity.EntityID) (item.Stack, bool)
	PutStack(entity.EntityID, item.Stack)
	DropStack(entity.EntityID)
}
type Site interface {
	Victims(x, y int) []entity.EntityID
	Crush(entity.EntityID, bool)
	ClearSite(x, y int)
	FillRock(x, y int)
	Rubble(x, y int)
}
type Host interface {
	Debris
	Site
}

// Collapse resolves a single tile from the batch. A thick collapse crushes
// and buries debris in new rock; thin debris damages actual item condition
// or a quarter of an unconditioned stack (original local tuning).
func Collapse(h Host, x, y int, thick bool) {
	for _, id := range h.Victims(x, y) {
		h.Crush(id, thick)
	}
	if thick {
		h.ClearSite(x, y)
	}
	for _, id := range h.Loose(x, y, nil) {
		st, ok := h.Stack(id)
		if !ok {
			continue
		}
		if thick {
			h.DropStack(id)
			continue
		}
		if st.MaxHP() > 0 {
			st.Wear += 25
		} else {
			st.Qty -= max(1, st.Qty/4)
		}
		if st.Qty <= 0 || st.WornOut() {
			h.DropStack(id)
		} else {
			h.PutStack(id, st)
		}
	}
	if thick {
		h.FillRock(x, y)
	} else {
		h.Rubble(x, y)
	}
}
