// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package travel

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"slices"
)

// RecoverCargo adds physical goods after the source owner has verified
// map, holder and route. It counts the caravan's actual pack animals,
// including animals in native world stashes rather than cargo holds.
// Capacity preflight and debit run on the single simulation owner.
func (o Ops) RecoverCargo(cid uint64, st item.Stack, members int, stashed []Beast, debit func() bool) (int32, bool) {
	if cid == 0 || members < 1 || st.Qty < 1 || !o.h.Caravan(cid) || debit == nil {
		return 0, false
	}
	h := o.s.cargo[cid]
	capacity := carrierCapacity(members, h, TradeCarriers{Stashed: stashed})
	room := capacity
	use := func(s item.Stack) bool {
		grams := int64(o.h.Items().MassGrams(s.Kind))
		if s.Qty <= 0 || grams <= 0 || int64(s.Qty) > room/grams {
			return false
		}
		room -= int64(s.Qty) * grams
		return true
	}
	if h != nil {
		for _, s := range h.Items {
			if !use(s) {
				return 0, false
			}
		}
	}
	if !use(st) || !debit() {
		return 0, false
	}
	st.X, st.Y, st.Carrier = 0, 0, 0
	h = o.s.cargoOf(cid)
	// Keep the exact physical payload (including meal rot, poison and
	// ingredients); only completely identical lots may be grouped.
	h.Items = appendTradeGood(h.Items, st)
	o.h.Changed()
	return int32((capacity - room) * 1000 / capacity), true
}

// PackSource is the map-local physical stack owner for site pack-up.
type PackSource interface {
	ItemIDs() []entity.EntityID
	Stack(entity.EntityID) (item.Stack, bool)
	DeleteItem(entity.EntityID)
}

// PackMapGoods takes permitted ground goods and abandoned settlement loot
// in stable item order; the map owner removes each transferred stack.
func PackMapGoods(o Ops, h PackSource, cid uint64, loot bool) {
	for _, sid := range slices.Clone(h.ItemIDs()) {
		if st, ok := h.Stack(sid); ok && (!st.Forbidden || loot && st.Unclaimed) {
			st.Forbidden = false
			o.Carry(cid, st)
			h.DeleteItem(sid)
		}
	}
}
