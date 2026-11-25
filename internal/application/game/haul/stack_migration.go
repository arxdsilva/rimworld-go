// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package haul

import (
	"rimworld-go/internal/domain/ai/reservation"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

// NearbySpill uses the native row-major ring tie order.
func NearbySpill(x, y, radius int, accept func(int, int) bool) (int, int, bool) {
	for r := 1; r <= radius; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(mathx.Abs(dx), mathx.Abs(dy)) == r && accept(x+dx, y+dy) {
					return x + dx, y + dy, true
				}
			}
		}
	}
	return 0, 0, false
}

// LimitMigration is a load-time physical stack owner. It does not split
// off-map inventories or carried loads, whose units are not ground tiles.
type LimitMigration interface {
	ItemIDs() []entity.EntityID
	Stack(entity.EntityID) (item.Stack, bool)
	SetStack(entity.EntityID, item.Stack)
	AppendStack(item.Stack)
	Items() item.Book
	Zones() *zone.Map
	CancelClaim(entity.EntityID)
	FreeTile(item.Stack, int, int) bool
}

func MigrateLimits(h LimitMigration, claims *reservation.Table) {
	var ids []entity.EntityID
	affected := map[entity.EntityID]bool{}
	for _, id := range h.ItemIDs() {
		st, ok := h.Stack(id)
		if ok && st.Carrier == 0 && h.Items().StackLimit(st.Kind) < st.Kind.StackLimit() && st.Qty > CellLimit(h.Items(), h.Zones(), st.Kind, st.X, st.Y) {
			ids = append(ids, id)
			affected[id] = true
		}
	}
	if len(ids) == 0 {
		return
	}
	// Cancel jobs which reserved quantities under the old policy before
	// splitting. The first part preserves the original entity and metadata.
	for _, claim := range claims.Export() {
		if !claim.Tile && affected[claim.Target] {
			h.CancelClaim(claim.Owner)
			claims.ReleaseOwner(claim.Owner)
		}
	}
	for _, id := range ids {
		st, ok := h.Stack(id)
		if !ok || st.Carrier != 0 {
			continue
		}
		limit := CellLimit(h.Items(), h.Zones(), st.Kind, st.X, st.Y)
		if st.Qty <= limit {
			continue
		}
		claims.ReleaseTarget(id)
		left := st.Qty - limit
		st.Qty = limit
		h.SetStack(id, st)
		for left > 0 {
			part := st
			part.Qty = min(left, limit)
			if x, y, ok := NearbySpill(st.X, st.Y, 4, func(x, y int) bool { return h.FreeTile(part, x, y) }); ok {
				part.X, part.Y = x, y
			}
			// An entirely full map still keeps separate legal stacks rather
			// than destroying units; normal hauling later clears the overlap.
			h.AppendStack(part)
			left -= part.Qty
		}
	}
}

// CellLimit is the stack limit in the storage zone at a physical tile.
func CellLimit(book item.Book, zones *zone.Map, k item.Kind, x, y int) int {
	id, ok := zones.At(x, y)
	if !ok {
		return book.StackLimit(k)
	}
	z, _ := zones.Zone(id)
	return z.Kind.StackCap(book.StackLimit(k))
}
